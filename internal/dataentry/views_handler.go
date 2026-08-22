package dataentry

import (
	"context"
	"log/slog"
	"net/http"
	"strings"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	v1 "github.com/Sourcehaven-BV/rela/internal/apiwire/v1"
	"github.com/Sourcehaven-BV/rela/internal/dataentryconfig"
	entityPkg "github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/natsort"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/visibility"
)

// viewsHandler owns the read-only view-assembly surface: the view
// traversal engine (views.go), the section builders (sections.go), and
// the /_views, /_sidepanel, /_sidebar endpoints, plus the form-resolution
// helpers those builders share (editFormForType/createFormForType).
// Extracted from App (TKT-R68TV8).
//
// Collaborator rationale mirrors attachmentHandler/writeHandler: fixed
// service handles by value (store/reader/serializer/affordances/logo are
// set once in NewApp and never swapped), the schema snapshot and Services
// bundle as closures so every operation reads the current published
// state, and App's shared read gate as a closure so the uniform-404
// behavior can't drift between the read and view paths. This surface is
// read-only — no writeMu, no entitymanager.
type viewsHandler struct {
	schema      func() *Schema
	store       store.Store
	reader      entityReader
	serializer  entitySerializer
	affordances affordanceService
	// viewReader is the row-gating + field-redacting read-out seam
	// (DEC-ZBI39P). Every entity this surface hands to the section builders
	// passes through it, so a neighbor the principal cannot read is dropped
	// and a survivor's hidden fields are redacted.
	viewReader visibility.Reader
	// services returns the read bundle; view traversal and relation-column
	// resolution read through it exactly as the App methods did.
	services func() Services
	// logo backs the sidebar logo URL.
	logo *logoStore
	// gateRead is App.gateReadOrNotFound: ACL-gates an entity read,
	// writing the uniform 404 on denial (hidden = nonexistent).
	gateRead func(w http.ResponseWriter, r *http.Request, typeName, entityID string) bool
	// aclImpl resolves the active ACL for permitsNavEntry's sidebar filter.
	// A closure, not a captured value, for the same reason commandHandler
	// holds one: tests reassign app.acl AFTER construction, so a captured
	// value would go stale and the filter would consult the wrong policy.
	aclImpl func() acl.ACL
}

// currentACL resolves the active ACL, or nil when the handler was constructed
// without the closure. Callers must treat nil as "hide" (see permitsNavEntry)
// — a wiring omission has to fail closed, not panic and not show.
func (h *viewsHandler) currentACL() acl.ACL {
	if h.aclImpl == nil {
		return nil
	}
	return h.aclImpl()
}

// redactor is the field-redaction seam for this surface, mirroring
// appRedactor: hidden properties resolve through the current affordance
// service so a redaction decision always reflects live policy.
func (h *viewsHandler) redactor() visibility.FieldRedactor {
	return affRedactor{aff: func() affordanceService { return h.affordances }}
}

// handleV1SidePanel handles GET /api/v1/_sidepanel/{formId}/{entityId}.
func (h *viewsHandler) handleV1SidePanel(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeV1Error(w, r, http.StatusMethodNotAllowed, "method_not_allowed", "Method not allowed", "")
		return
	}

	// Parse path: /api/v1/_sidepanel/{formId}/{entityId}
	path := strings.TrimPrefix(r.URL.Path, "/api/v1/_sidepanel/")
	parts := strings.SplitN(path, "/", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		writeV1Error(w, r, http.StatusBadRequest, "invalid_path", "Path must be /_sidepanel/{formId}/{entityId}", "")
		return
	}

	formID := parts[0]
	entityID := parts[1] // Get form config
	s := h.schema()
	form, ok := s.Cfg.Forms[formID]
	if !ok {
		writeV1Error(w, r, http.StatusNotFound, "form_not_found", "Form not found", "")
		return
	}

	// Check if form has side panel
	if form.SidePanel == nil {
		writeV1JSON(w, http.StatusOK, []v1.SidePanelSection{})
		return
	}

	// ACL gate (TKT-6N9O1Y): the side panel reveals the entry entity and its
	// traversal neighbors. Gate the entry read BEFORE getEntity/executeSidePanel
	// so a principal who cannot read it gets a 404 indistinguishable from a
	// missing id, and the traversal never runs for a denied caller.
	if !h.gateRead(w, r, form.EntityType, entityID) {
		return
	}

	// Get the entry entity
	entry, found := h.reader.getEntity(r.Context(), entityID)
	if !found {
		writeV1Error(w, r, http.StatusNotFound, "entity_not_found", "Entity not found", "")
		return
	}

	// Execute side panel traversal
	sections := h.executeSidePanel(r.Context(), form.SidePanel, entityID, form.EntityType)
	if sections == nil {
		writeV1JSON(w, http.StatusOK, []v1.SidePanelSection{})
		return
	}

	// Build a synthetic ViewConfig to resolve add/link buttons
	viewConfig := ViewConfig{
		Entry:    ViewEntry{Type: form.EntityType},
		Traverse: form.SidePanel.Traverse,
		Sections: form.SidePanel.Sections,
	}
	h.resolveSectionButtonsWithTraverse(viewConfig, sections, entry)

	// Convert to API response format
	result := make([]v1.SidePanelSection, 0, len(sections))
	for _, sec := range sections {
		apiSec := v1.SidePanelSection{
			Heading:      sec.Heading,
			SectionID:    sec.SectionID,
			Display:      sec.Display,
			IsEmpty:      sec.IsEmpty,
			EmptyMessage: sec.EmptyMessage,
		}

		// Convert fields
		for _, f := range sec.Fields {
			apiSec.Fields = append(apiSec.Fields, v1.SectionField(f))
		}

		// Convert entities
		for _, e := range sec.Entities {
			apiEnt := v1.SidePanelEntity{
				ID:         e.ID,
				Title:      e.Title,
				Type:       e.Type,
				EditFormID: e.EditFormID,
				Content:    e.Content,
				HasContent: e.HasContent,
			}
			for _, f := range e.Fields {
				apiEnt.Fields = append(apiEnt.Fields, v1.SectionField(f))
			}
			apiSec.Entities = append(apiSec.Entities, apiEnt)
		}

		// Convert add/link info
		if sec.AddInfo != nil {
			apiSec.AddInfo = &v1.ViewAddInfo{
				Relation: sec.AddInfo.Relation,
				LinkAs:   sec.AddInfo.LinkAs,
				PeerID:   sec.AddInfo.PeerID,
			}
			for _, t := range sec.AddInfo.Targets {
				apiSec.AddInfo.Targets = append(apiSec.AddInfo.Targets, v1.ViewAddTarget(t))
			}
		}
		if sec.LinkInfo != nil {
			apiSec.LinkInfo = &v1.ViewLinkInfo{
				Relation:    sec.LinkInfo.Relation,
				LinkAs:      sec.LinkInfo.LinkAs,
				PeerID:      sec.LinkInfo.PeerID,
				EntityTypes: sec.LinkInfo.EntityTypes,
			}
		}

		result = append(result, apiSec)
	}

	writeV1JSON(w, http.StatusOK, result)
}

// handleV1Sidebar returns denormalized sidebar data with entity counts.
func (h *viewsHandler) handleV1Sidebar(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeV1Error(w, r, http.StatusMethodNotAllowed, "method_not_allowed", "Method not allowed", "")
		return
	}
	s := h.schema()

	counts := sidebarCounts{
		filterCache: make(map[string]int),
		h:           h,
	}

	// Build navigation with counts. Entries the principal cannot use are
	// omitted (permitsNavEntry) — a UX filter, not a boundary; see its doc.
	// The ACL is resolved ONCE here rather than per entry, matching
	// resolveCommands.
	navigation := make([]v1.SidebarGroup, 0)
	aclImpl := h.currentACL()

	for _, entry := range s.Cfg.Navigation {
		if entry.IsGroup() {
			group := v1.SidebarGroup{
				Group:     entry.Group,
				Collapsed: entry.Collapsed,
				Items:     make([]v1.SidebarItem, 0),
			}
			for _, item := range entry.Items {
				if !permitsNavEntry(r.Context(), aclImpl, item) {
					continue
				}
				sidebarItem := h.navEntryToSidebarItem(r.Context(), item, counts)
				group.Items = append(group.Items, sidebarItem)
			}
			// A group whose every item was filtered out is dropped rather than
			// rendered as a bare heading: an empty labeled group reads as a
			// rendering bug, and it outlines what the principal cannot reach
			// without being useful to them.
			if len(group.Items) == 0 {
				continue
			}
			navigation = append(navigation, group)
		} else {
			// Top-level item without group
			if !permitsNavEntry(r.Context(), aclImpl, entry) {
				continue
			}
			item := h.navEntryToSidebarItem(r.Context(), entry, counts)
			navigation = append(navigation, v1.SidebarGroup{
				Items: []v1.SidebarItem{item},
			})
		}
	}

	resp := v1.SidebarResponse{
		App: v1.AppConfig{
			Name:        s.Cfg.App.Name,
			Description: s.Cfg.App.Description,
		},
		Navigation: navigation,
	}
	resp.LogoURL = h.logo.URL()
	resp.InlineCreate = h.inlineCreateForms(r.Context())

	writeV1JSON(w, http.StatusOK, resp)
}

// sidebarCounts caches sidebar entity counts, applying list- or kanban-
// level filters when present. Every count flows through the ACL read
// scope (TKT-VMD8) — one code path regardless of NopACL vs. Declarative
// (RR-2O27), so the sidebar can never disagree with the list it links
// to. filterCache is a within-request memo keyed by list/kanban id; it
// is safe precisely because a sidebarCounts value lives for one request
// (one principal) — a longer-lived cache would alias counts across
// principals (RR-BZ4M).
type sidebarCounts struct {
	filterCache map[string]int // key: "list:<id>" or "kanban:<id>"
	h           *viewsHandler
}

// listCount returns the entity count for the given list, applying any
// configured filters. Results are cached per call.
func (c *sidebarCounts) listCount(ctx context.Context, listID string, list dataentryconfig.List) int {
	key := "list:" + listID
	if n, ok := c.filterCache[key]; ok {
		return n
	}
	n := c.countWithFilters(ctx, list.EntityType, list.Filters)
	c.filterCache[key] = n
	return n
}

// kanbanCount returns the entity count for the given kanban, applying
// any configured filters. Results are cached per call.
func (c *sidebarCounts) kanbanCount(ctx context.Context, kanbanID string, kanban dataentryconfig.Kanban) int {
	key := "kanban:" + kanbanID
	if n, ok := c.filterCache[key]; ok {
		return n
	}
	n := c.countWithFilters(ctx, kanban.EntityType, kanban.Filters)
	c.filterCache[key] = n
	return n
}

// countWithFilters returns the count of entities of the given type that
// are visible to the requesting principal AND pass the supplied config
// filters. Ordering is ACL → config filter → count (TKT-VMD8 AC7).
//
// Without config filters the count comes straight from GraphCount —
// identical cost to the old Store.CountEntities for the AllowAll case.
// With config filters the visible entities are loaded and filtered
// in-memory; performance scales with the visible-set size (RR-REQW —
// for visible sets >10k prefer pre-filtering via entity_type in nav
// config, or file the follow-up that pushes filters into GraphQuery).
//
// Backend errors degrade to 0 with a warning — parity with the old
// CountEntities error path: a broken sidebar count must not take the
// whole sidebar down, and the list endpoint surfaces the real error.
//
// ReadQuery (one member-of walk reuse via the request-scoped
// acl.Request) and the GraphQuery/GraphCount run once per nav item —
// two lists over the same type recompute rather than share. Accepted:
// filterCache keys on list/kanban id, not (type, filters); a
// (type, filters)-keyed memo is the obvious upgrade if sidebar
// latency ever warrants it.
func (c *sidebarCounts) countWithFilters(
	ctx context.Context, entityType string, filters []dataentryconfig.FilterConfig,
) int {
	rqr := readGateFromContext(ctx).ReadQuery(ctx, entityType)
	if rqr.DenyAll {
		return 0
	}
	q := store.GraphQuery{EntityType: entityType}
	if rqr.Query != nil {
		q = *rqr.Query
	}

	if len(filters) == 0 {
		matched, _, err := c.h.services().Store.GraphCount(ctx, q)
		if err != nil {
			slog.Warn("sidebar: GraphCount failed; count degraded to 0",
				"entity_type", entityType, "error", err)
			return 0
		}
		return matched
	}

	var entities []*entityPkg.Entity
	for e, err := range c.h.services().Store.GraphQuery(ctx, q) {
		if err != nil {
			slog.Warn("sidebar: GraphQuery failed; count degraded to 0",
				"entity_type", entityType, "error", err)
			return 0
		}
		entities = append(entities, e)
	}
	return len(applyFilters(entities, filters))
}

// permitsNavEntry reports whether a navigation entry should appear in this
// principal's sidebar (TKT-TXDK8U). It is a thin wrapper over
// [permitsGatedUIElement]; the policy and its reasoning live there.
func permitsNavEntry(ctx context.Context, aclImpl acl.ACL, entry dataentryconfig.NavigationEntry) bool {
	return permitsGatedUIElement(ctx, aclImpl, entry.Permission)
}

// permitsGatedUIElement reports whether a `permission:`-gated UI element should
// be shown to this principal (TKT-TXDK8U for sidebar nav entries, TKT-53KICM
// for dashboard cards).
//
// This is a UX filter and NOTHING ELSE. It decides what to RENDER, never what
// to allow. If you want an authorization check, this is the wrong function —
// build one, and see authorizeCommand for the shape.
//
// Hiding an element changes no enforcement: its target is reachable by typing
// the URL and behaves exactly as it did before — a list still returns its
// ACL-scoped rows, which for a principal who may read none of them is simply an
// empty list, and a dashboard card's query still runs through the ACL-scoped
// search path. Nor does it conceal configuration: `/api/v1/_config` keeps
// serving the whole navigation tree and `dashboard:` block to every principal,
// deliberately (root CLAUDE.md, "The configuration is not a secret; the data
// is"). The goal is only to keep things a user cannot act on out of their way.
//
// Policy, keyed on the configured ACL implementation, mirroring
// authorizeCommand (DEC-EIHQSU):
//
//   - No `permission:` → show. The overwhelmingly common case, short-circuited
//     before any ACL work so an unconfigured menu costs nothing.
//   - [acl.NopACL] and [acl.ReadOnlyACL] → show. NEITHER carries a policy, so
//     there is no permission model to consult and no principal who could be
//     shown to hold anything: "no policy configured ⇒ no restrictions", the
//     same posture the read gate takes. See the read-only note below.
//   - [*acl.Declarative] → the principal must hold the permission.
//   - anything else → hide.
//
// Read-only deserves its own paragraph, because the obvious arm is wrong in
// two different ways and this predicate is a copy target.
//
// It is tempting to mirror authorizeCommand, which denies everything under
// ReadOnlyACL. That is right for commands — they SHELL OUT, a write-shaped act
// — but [acl.ReadOnlyACL] only implements AuthorizeWrite; it restricts no
// reads whatsoever. The gated elements are overwhelmingly read surfaces (list,
// kanban, dashboard, search, dashboard cards), and hiding them would remove
// things an observe-only principal can use perfectly well. It would also hide
// them from EVERYONE, since ReadOnlyACL has no identity to check — so
// `permission:` would silently
// change meaning from "hide from non-holders" to "hide from all" based on a
// process-wide flag about writes. An operator in post-incident forensic mode
// (a documented ReadOnlyACL use case) would lose exactly the audit-log entry
// they came for.
//
// The hazard the deny arm was reaching for is real but belongs elsewhere: under
// ReadOnlyACL no middleware attaches a read gate, so readGateFromContext hands
// back nopReadGate, whose HoldsPermission returns true unconditionally
// (RR-CWWJGW). Falling THROUGH to the gate would therefore show gated entries
// while looking like it had checked something. The explicit arm above is what
// prevents that: the answer is the same, but it is reached deliberately rather
// than by an accident that would keep working if the gate's behavior changed.
//
// The switch is closed by construction: an ACL implementation nobody taught
// this function about hides gated entries rather than showing them. Both value
// and pointer forms of the nop/read-only types are matched, because their
// AuthorizeWrite has a value receiver — matching only the value form would
// drop a `&acl.ReadOnlyACL{}` into the default arm.
func permitsGatedUIElement(ctx context.Context, aclImpl acl.ACL, permission string) bool {
	if permission == "" {
		return true
	}
	// A nil ACL means the handler was wired without one. Hide, for the same
	// fail-closed reason authorizeCommand denies.
	if aclImpl == nil {
		return false
	}

	switch a := aclImpl.(type) {
	// Grouped deliberately: both mean "no policy is configured", so neither
	// can answer whether a principal holds a permission. See the read-only
	// paragraph above before splitting these apart.
	case acl.NopACL, *acl.NopACL, acl.ReadOnlyACL, *acl.ReadOnlyACL:
		return true

	case *acl.Declarative:
		if a == nil {
			return false // misconfigured policy must not fail open
		}
		return readGateFromContext(ctx).HoldsPermission(ctx, permission)

	default:
		return false
	}
}

// navEntryToSidebarItem converts a navigation entry to a sidebar item with count.
func (h *viewsHandler) navEntryToSidebarItem(
	ctx context.Context, entry dataentryconfig.NavigationEntry, counts sidebarCounts,
) v1.SidebarItem {
	s := h.schema()
	item := v1.SidebarItem{
		Label: entry.Label,
	}

	switch {
	case entry.List != "":
		item.Href = "/list/" + entry.List
		item.Icon = "list"
		if list, ok := s.Cfg.Lists[entry.List]; ok {
			count := counts.listCount(ctx, entry.List, list)
			item.Count = &count
		}
	case entry.Kanban != "":
		item.Href = "/kanban/" + entry.Kanban
		item.Icon = "kanban"
		if kanban, ok := s.Cfg.Kanbans[entry.Kanban]; ok {
			count := counts.kanbanCount(ctx, entry.Kanban, kanban)
			item.Count = &count
		}
	case entry.Calendar != "":
		item.Href = "/calendar/" + entry.Calendar
		item.Icon = "calendar"
		// Deliberately no count. A list or board counts the set it displays; a
		// calendar displays one period, so an unwindowed total ("847" beside a
		// grid showing 12) is true, unactionable, and never changes as the user
		// navigates. A period-scoped count is not available either — the
		// sidebar is rendered server-side and does not know which month is on
		// screen. SidebarItem.Count is a *int, so absent needs no wire change.
	case entry.Dashboard:
		item.Href = "/"
		item.Icon = "dashboard"
	case entry.Search:
		item.Href = "/search"
		item.Icon = "search"
	case entry.Settings:
		item.Href = "/settings"
		item.Icon = "settings"
	case entry.Document != "":
		// Standalone documents only — validateNavEntry rejects an
		// entity-anchored document here, since this href has no entity id
		// segment to fill.
		item.Href = "/document/" + entry.Document
		item.Icon = "document"
	case entry.Action != "":
		item.Action = entry.Action
		// Href stays empty — frontend renders this as a button
	}

	// An authored icon wins over the kind-derived default set above. Applied
	// after the switch so it covers every branch — including `action`, which
	// derives no icon of its own and would otherwise be the one entry kind
	// that could never have one.
	if entry.Icon != "" {
		item.Icon = entry.Icon
	}

	return item
}

// handleV1Views handles GET /api/v1/_views/{entityType}/{entityId}.
// Returns JSON with executed view data including entry and sections.
//
// View configs are looked up by entry.type. When no explicit ViewConfig
// is registered for entityType, a default is synthesized from the
// metamodel (see buildDefaultViewConfig) and executed through the same
// pipeline so the response shape is identical.
//
//nolint:gocognit,funlen // routes the views sub-API over method and path shape; each branch is a distinct view endpoint, not shared logic to factor out.
func (h *viewsHandler) handleV1Views(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeV1Error(w, r, http.StatusMethodNotAllowed, "method_not_allowed", "Method not allowed", "")
		return
	}

	// Parse path: /api/v1/_views/{entityType}/{entityId}
	path := strings.TrimPrefix(r.URL.Path, "/api/v1/_views/")
	parts := strings.SplitN(path, "/", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		writeV1Error(w, r, http.StatusBadRequest, "invalid_path", "Path must be /_views/{entityType}/{entityId}", "")
		return
	}

	entityType, entityID := parts[0], parts[1]
	s := h.schema()

	if _, ok := s.Meta.GetEntityDef(entityType); !ok {
		writeV1Error(w, r, http.StatusNotFound, "entity_type_not_found", "Entity type not found", entityType)
		return
	}

	// ACL gate (TKT-BNX2PN): _views is an entity-read chokepoint just like
	// GET /{plural}/{id} — it serves _title, properties, and content body via
	// executeView + serializeEntityForWire. Gate BEFORE executeView so a hidden
	// id is indistinguishable from a missing one (404, no oracle) and the view
	// pipeline never runs for a denied principal.
	if !h.gateRead(w, r, entityType, entityID) {
		return
	}

	viewCfg, ok := findViewByEntityType(s.Cfg.Views, entityType)
	if !ok {
		viewCfg, ok = buildDefaultViewConfig(s.Meta, entityType)
		if !ok {
			// Cannot happen — entity type already validated above —
			// but handled defensively to keep the contract clear.
			writeV1Error(w, r, http.StatusNotFound, "entity_type_not_found", "Entity type not found", entityType)
			return
		}
	}

	// Execute view
	result, err := h.executeView(r.Context(), viewCfg, entityID)
	if err != nil {
		writeV1Error(w, r, http.StatusUnprocessableEntity, "view_execution_failed", "View execution failed", err.Error())
		return
	}

	// Build sections
	sections := h.buildSections(r.Context(), viewCfg.Sections, result)

	// Build response
	entityDef := s.Meta.Entities[result.Entry.Type]
	plural := entityDef.GetPlural(result.Entry.Type)

	entryRels := h.reader.outgoingRelations(r.Context(), result.Entry.ID)
	resp := v1.ViewResponse{
		Entry:    h.serializer.forWire(r.Context(), result.Entry, entryRels, h.schema().Meta, plural),
		Sections: make([]v1.ViewSection, 0, len(sections)),
	}

	for _, sec := range sections {
		v1Sec := v1.ViewSection{
			Heading:      sec.Heading,
			SectionID:    sec.SectionID,
			Display:      sec.Display,
			IsEmpty:      sec.IsEmpty,
			EmptyMessage: sec.EmptyMessage,
			IsGrouped:    sec.IsGrouped,
			Content:      sec.Content,
			HasContent:   sec.HasContent,
		}

		// Convert fields
		for _, f := range sec.Fields {
			v1Sec.Fields = append(v1Sec.Fields, v1.SectionField(f))
		}

		// Convert entities
		for _, e := range sec.Entities {
			v1Sec.Entities = append(v1Sec.Entities, sectionEntityToV1(e))
		}

		// Convert columns
		for _, col := range sec.Columns {
			v1Sec.Columns = append(v1Sec.Columns, v1.ViewColumn{
				Property: col.Property,
				Label:    col.Label,
				Relation: col.Relation,
				Link:     col.Link,
			})
		}

		// Convert rows
		for _, row := range sec.Rows {
			v1Row := v1.ViewRow{
				EntityID:   row.EntityID,
				EntityType: row.EntityType,
				EditFormID: row.EditFormID,
				Content:    row.Content,
			}
			for _, cell := range row.Cells {
				v1Row.Cells = append(v1Row.Cells, v1.ViewCell(cell))
			}
			v1Sec.Rows = append(v1Sec.Rows, v1Row)
		}

		// Convert groups
		for _, grp := range sec.Groups {
			v1Grp := v1.ViewGroup{
				GroupName: grp.GroupName,
			}
			for _, row := range grp.Rows {
				v1Row := v1.ViewRow{
					EntityID:   row.EntityID,
					EntityType: row.EntityType,
					EditFormID: row.EditFormID,
					Content:    row.Content,
				}
				for _, cell := range row.Cells {
					v1Row.Cells = append(v1Row.Cells, v1.ViewCell(cell))
				}
				v1Grp.Rows = append(v1Grp.Rows, v1Row)
			}
			for _, e := range grp.Entities {
				v1Grp.Entities = append(v1Grp.Entities, sectionEntityToV1(e))
			}
			v1Sec.Groups = append(v1Sec.Groups, v1Grp)
		}

		resp.Sections = append(resp.Sections, v1Sec)
	}

	resp.Mentions = collectMentions(
		r.Context(), h.store, h.viewReader, s.Meta, viewContentBlobs(result.Entry, sections)...)

	writeV1JSON(w, http.StatusOK, resp)
}

// viewContentBlobs gathers every markdown body that will be rendered by
// the SPA for a single view response: the entry's content, every section's
// own content, and every entity card's content (sections with display
// "content"/"cards" surface related entities, each carrying its own
// `Content` markdown that EntityDetail.vue renders with the same
// `refResolver`). Used to scope the mentions scan to text the user
// actually sees on this screen.
func viewContentBlobs(entry *entityPkg.Entity, sections []SectionData) []string {
	blobs := make([]string, 0, 1+len(sections))
	if entry != nil && entry.Content != "" {
		blobs = append(blobs, entry.Content)
	}
	for _, sec := range sections {
		if sec.HasContent && sec.Content != "" {
			blobs = append(blobs, sec.Content)
		}
		for _, ent := range sec.Entities {
			if ent.HasContent && ent.Content != "" {
				blobs = append(blobs, ent.Content)
			}
		}
		for _, grp := range sec.Groups {
			for _, ent := range grp.Entities {
				if ent.HasContent && ent.Content != "" {
					blobs = append(blobs, ent.Content)
				}
			}
		}
	}
	return blobs
}

// editFormForType returns the first edit form ID configured for the given entity type,
// or "" if no edit form is found. Forms with explicit mode="edit" are preferred.
func (h *viewsHandler) editFormForType(entityType string) string {
	s := h.schema()
	ids := make([]string, 0, len(s.Cfg.Forms))
	for id := range s.Cfg.Forms {
		ids = append(ids, id)
	}
	natsort.Strings(ids)
	// First pass: look for explicit edit mode
	for _, id := range ids {
		f := s.Cfg.Forms[id]
		if f.EntityType == entityType && f.Mode == "edit" {
			return id
		}
	}
	// Second pass: fall back to forms with no mode specified
	for _, id := range ids {
		f := s.Cfg.Forms[id]
		if f.EntityType == entityType && f.Mode == "" {
			return id
		}
	}
	return ""
}

// inlineCreateForms maps each entity type the principal may create inline to
// the form id to create it with. A type is present only when BOTH conditions of
// TKT-OMUD56 hold — a create form resolves for it, and the ACL permits
// `create` — so presence alone is the affordance and the SPA performs no
// permission arithmetic of its own.
//
// The form lookup runs first because it is a pure config read: a type nothing
// can create has no ACL question worth asking, so this also keeps the ACL
// evaluations to the types that could actually be offered.
//
// The `create` verdict is the same OpCreate question computeCollectionActions
// asks for a list response, so a type's verdict here cannot diverge from the
// one on GET /api/v1/{plural}. UI hint only: the write endpoint re-authorizes
// (affordances_contract_test.go pins that invariant).
//
// This loops over entity types, so it resolves the principal's Request ONCE and
// reuses it. Going through the top-level ACL entry point per type would rebuild
// that scope every iteration — and role resolution walks `member-of` through
// the store, so an N-type metamodel would pay N graph traversals on every app
// load. Reusing the scope is exactly what it exists for.
func (h *viewsHandler) inlineCreateForms(ctx context.Context) map[string]string {
	s := h.schema()

	// The middleware attaches a per-request scope; fall back to the unscoped
	// path when one is absent (tests and any non-HTTP caller), which is
	// correct-but-slower rather than a different answer.
	scope := acl.FromContext(ctx)
	mayCreate := func(entityType string) bool {
		req := translateVerb("create", entityType, "")
		if scope != nil {
			return scope.AuthorizeWrite(ctx, req).Allow
		}
		return h.currentACL().AuthorizeWrite(ctx, req).Allow
	}

	var out map[string]string
	for name := range s.Meta.Entities {
		// Form lookup first: it is a pure config read, so a type nothing can
		// create never costs an authorization.
		formID := h.createFormForType(name)
		if formID == "" {
			continue
		}
		if !mayCreate(name) {
			continue
		}
		if out == nil {
			out = make(map[string]string)
		}
		out[name] = formID
	}
	return out
}

// createFormForType returns the first form ID that can be used to create an entity
// of the given type. It prefers forms with mode "create" or unset, but falls back
// to edit-mode forms (which work for creation when no entity ID is provided).
func (h *viewsHandler) createFormForType(entityType string) string {
	s := h.schema()
	ids := make([]string, 0, len(s.Cfg.Forms))
	for id := range s.Cfg.Forms {
		ids = append(ids, id)
	}
	natsort.Strings(ids)
	fallback := ""
	for _, id := range ids {
		f := s.Cfg.Forms[id]
		if f.EntityType != entityType {
			continue
		}
		if f.Mode != "edit" {
			return id
		}
		if fallback == "" {
			fallback = id
		}
	}
	return fallback
}

// resolveLinkTarget resolves a link configuration value to a URL.
// Supported values:
//   - "" or empty: no link (returns "")
//   - "detail": link to entity detail view (/entity/{type}/{id})
//   - "document/<name>": link to document preview (/document/<name>/{id})
func resolveLinkTarget(link, entityType, entityID string) string {
	switch {
	case link == "":
		return ""
	case link == "detail":
		return "/entity/" + entityType + "/" + entityID
	case strings.HasPrefix(link, "document/"):
		docName := strings.TrimPrefix(link, "document/")
		return "/document/" + docName + "/" + entityID
	default:
		return ""
	}
}

func findListByEntityType(s *Schema, entries []NavigationEntry, entityType string) string {
	for _, nav := range entries {
		if nav.IsGroup() {
			if found := findListByEntityType(s, nav.Items, entityType); found != "" {
				return found
			}
			continue
		}
		if list, ok := s.Cfg.Lists[nav.List]; ok && list.EntityType == entityType {
			return nav.List
		}
	}
	return ""
}

// resolveRelationColumnValues returns display titles for all targets of the given
// relation type from an entity. Direction controls whether to follow edges pointing
// to the entity (incoming) or from the entity (outgoing, the default).
//
// The targets are routed through viewReader.Filter (DEC-ZBI39P) before their
// titles are derived: a neighbor the principal may not read is dropped
// (row-gate), and a survivor whose display property is hidden has it redacted so
// DisplayTitle falls back to the id (BUG-R9EHKV). Without this, a table
// relation-column leaked a hidden neighbor's title via the raw store read —
// the one builder path the executeView redaction did not cover, since these
// targets are fetched fresh here rather than carried in result.Collections.
func (h *viewsHandler) resolveRelationColumnValues(
	ctx context.Context, entityID, relationType string, direction dataentryconfig.Direction,
) []string {
	svc := h.services()
	q := store.RelationQuery{
		EntityID:  entityID,
		Type:      relationType,
		Direction: relationDirection(direction),
	}

	var targets []*entityPkg.Entity
	for r, err := range svc.Store.ListRelations(ctx, q) {
		if err != nil {
			break
		}
		targetID := r.To
		if direction.IsIncoming() {
			targetID = r.From
		}
		if e, gerr := svc.Store.GetEntity(ctx, targetID); gerr == nil {
			targets = append(targets, e)
		}
	}

	visible := h.viewReader.Filter(ctx, targets)
	titles := make([]string, 0, len(visible))
	for _, e := range visible {
		titles = append(titles, svc.Meta.DisplayTitle(e.ID, e.Type, e.Properties))
	}
	return titles
}
