package dataentry

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"hash"
	"log/slog"
	"net/http"
	"slices"
	"strconv"
	"strings"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	v1 "github.com/Sourcehaven-BV/rela/internal/apiwire/v1"
	"github.com/Sourcehaven-BV/rela/internal/audit"
	"github.com/Sourcehaven-BV/rela/internal/canonical"
	"github.com/Sourcehaven-BV/rela/internal/dataentryconfig"
	entityPkg "github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/filter"
	"github.com/Sourcehaven-BV/rela/internal/lua"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/project"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/visibility"
)

// --- API v1 Types ---

// toV1EntityType renders one entity type for the wire.
//
// Shared by BOTH schema surfaces — the `entities` map of `/api/v1/_schema`
// and the single-type `/api/v1/_schema/types/{name}` — which previously each
// built the shape inline. Two copies of a serializer is two places to add a
// field and one place to forget it: the `faces` key (TKT-WRLDAPI item 3)
// would have been exactly such a field, since a client that discovers a
// type's content states from one endpoint and not the other has no way to
// tell which answer is authoritative. Extracted so they cannot drift.
//
// A free function, like its sibling toV1CustomType: it is a pure transform
// over its arguments and touches no App state. toV1PropertyDef was converted
// alongside it for the same reason — it never used its receiver either, and
// App sits at its plimsoll method cap precisely because methods accrete there
// by habit rather than by need.
func toV1EntityType(
	meta *metamodel.Metamodel, name string, def metamodel.EntityDef,
) v1.EntityType {
	et := v1.EntityType{
		Label:       def.Label,
		LabelPlural: strings.TrimSpace(def.LabelPlural),
		Plural:      def.GetPlural(name),
		Description: def.Description,
		Primary:     def.GetPrimaryProperty(),
		IDType:      def.GetIDType(),
		Properties:  make(map[string]v1.PropertyDef, len(def.Properties)),
		Faces:       schemaFaceDefs(def),
		// Whether this type accepts comments (TKT-FIO205). Policy, not
		// permission: it tells the SPA whether to offer a comment affordance
		// at all, never that the caller may use it.
		Commentable: metamodel.NewCommentPolicy(meta).Commentable(name),
		Namespace:   metamodel.NamespaceOf(name),
		Extends:     def.Extends,
	}
	if prefixes := def.GetIDPrefixes(); len(prefixes) > 0 {
		et.IDPrefix = prefixes[0]
		et.IDPrefixes = prefixes
	}
	for propName, propDef := range def.Properties {
		et.Properties[propName] = toV1PropertyDef(meta, propDef)
	}
	return et
}

// toV1AbstractTypes describes meta's abstract types, or nil when there are
// none.
func toV1AbstractTypes(meta *metamodel.Metamodel) map[string]v1.AbstractType {
	abstracts := metamodel.AbstractTypes(meta)
	if len(abstracts) == 0 {
		return nil
	}
	out := make(map[string]v1.AbstractType, len(abstracts))
	for _, a := range abstracts {
		at := v1.AbstractType{
			Label:       a.Def.Label,
			Description: a.Def.Description,
			Variants:    a.Variants,
			Properties:  make(map[string]v1.PropertyDef, len(a.Def.Properties)),
		}
		for name, pd := range a.Def.Properties {
			at.Properties[name] = toV1PropertyDef(meta, pd)
		}
		out[a.Name] = at
	}
	return out
}

func toV1PropertyDef(meta *metamodel.Metamodel, propDef metamodel.PropertyDef) v1.PropertyDef {
	pd := v1.PropertyDef{
		Label:       propDef.Label,
		Type:        propDef.Type,
		Required:    propDef.Required,
		Default:     propDef.Default,
		Description: propDef.Description,
		List:        propDef.List,
		Min:         propDef.Min,
		Max:         propDef.Max,
		Scale:       propDef.Scale,
		System:      propDef.System,
	}
	if d := propDef.Display; d != nil {
		pd.Display = &v1.PropertyDisplay{
			Currency: d.Currency, Image: d.Image,
			LinkDisplay: v1.LinkDisplay{Link: d.Link, LinkText: d.LinkText},
		}
	}
	// Labels mirror Values: a property naming a custom type inherits that
	// type's values and labels; an inline `labels` map on a custom-typed
	// property is ignored, exactly as inline `values` is.
	if ct, ok := meta.Types[propDef.Type]; ok {
		pd.Values = ct.Values
		pd.Labels = ct.Labels
	} else if len(propDef.Values) > 0 {
		pd.Values = propDef.Values
		pd.Labels = propDef.Labels
	}
	return pd
}

// toV1CustomType is the single serialization site for a metamodel custom type
// onto the wire, so the _schema types map and any other consumer stay in sync.
//
// It intentionally projects only Values/Labels/Default. The documentation-only
// fields — CustomType.Descriptions (per-value meaning), CustomType.Transitions,
// and TransitionDef.Help (TKT-0YBFT8) — are NOT serialized: their consumer is the
// offline `rela docs` generator (FEAT-G4VO53), not the SPA. Wiring any of them to
// the wire is a deliberate frontend-contract change (see internal/dataentry/
// CLAUDE.md), not a "helpful" one-liner here. TestToV1CustomType_OmitsDocFields
// pins this boundary.
func toV1CustomType(ct metamodel.CustomType) v1.CustomType {
	return v1.CustomType{
		Values:  ct.Values,
		Labels:  ct.Labels,
		Default: ct.Default,
	}
}

// --- API v1 Router ---

// registerAPIV1Routes registers all /api/v1/ routes.
// Note: /api/v1/_events is registered separately in NewRouter as it needs to be
// outside the reload-lock middleware (SSE long-lived connection).
//
// When adding a route, add a probe to the route table in
// router_walk_test.go so registration stays covered.
func (a *App) registerAPIV1Routes(mux *http.ServeMux) {
	// System endpoints (underscore prefix)
	mux.HandleFunc("/api/v1/_schema", a.schemaAPI.handleV1Schema)
	mux.HandleFunc("/api/v1/_schema/", a.schemaAPI.handleV1SchemaRoutes)
	mux.HandleFunc("/api/v1/_config", a.schemaAPI.handleV1Config)
	configure := serveConfigure(a)
	mux.HandleFunc(ConfigurePath, configure)
	mux.HandleFunc(ConfigurePath+"/", configure)
	mux.HandleFunc("/api/v1/_me", func(w http.ResponseWriter, r *http.Request) { handleV1Me(a, w, r) })
	mux.HandleFunc(TenantStatusPath, handleV1TenantStatus)
	mux.HandleFunc("/api/v1/_feeds/", a.handleV1Feed)
	if routes := newCalDAVRoutes(a); routes != nil {
		routes.register(mux)
	}
	mux.HandleFunc("/api/v1/_search", a.entities.handleV1Search)
	mux.HandleFunc("/api/v1/_position", a.entities.handleV1EntityPosition)
	mux.HandleFunc("/api/v1/_analyze", a.analyzeAPI.handleV1Analyze)
	mux.HandleFunc("/api/v1/_next_action", a.nextActions.handleV1NextAction)
	mux.HandleFunc("/api/v1/_git/status", a.gitAPI.handleGitStatus)
	mux.HandleFunc("/api/v1/_git/sync", a.gitAPI.handleGitSync)
	registerAppearanceRoutes(mux, a)
	mux.HandleFunc("/api/v1/_sidepanel/", a.views.handleV1SidePanel)
	mux.HandleFunc("/api/v1/_sidebar", a.views.handleV1Sidebar)
	mux.HandleFunc("/api/v1/_nav_status", func(w http.ResponseWriter, r *http.Request) { handleV1NavStatus(a, w, r) })
	mux.HandleFunc("/api/v1/_nav_items", func(w http.ResponseWriter, r *http.Request) { handleV1NavItems(a, w, r) })
	mux.HandleFunc("/api/v1/_page_status", func(w http.ResponseWriter, r *http.Request) { handleV1PageStatus(a, w, r) })
	mux.HandleFunc("/api/v1/_dashboard", a.views.handleV1Dashboard)
	mux.HandleFunc("/api/v1/_dashboard/card", func(w http.ResponseWriter, r *http.Request) {
		handleV1DashboardCard(a, w, r)
	})
	mux.HandleFunc("/api/v1/_conflicts", a.conflictsAPI.handleV1Conflicts)
	mux.HandleFunc("/api/v1/_conflicts/", a.conflictsAPI.handleV1ConflictRoutes)
	mux.HandleFunc("/api/v1/_documents/", a.handleV1Documents)
	mux.HandleFunc("/api/v1/_history/", func(w http.ResponseWriter, r *http.Request) { handleV1History(a, w, r) })
	mux.HandleFunc("/api/v1/_changes/", func(w http.ResponseWriter, r *http.Request) { handleV1Changes(a, w, r) })
	mux.HandleFunc("/api/v1/_audit", func(w http.ResponseWriter, r *http.Request) { handleV1Audit(a, w, r) })
	mux.HandleFunc("/api/v1/_relation_history/",
		func(w http.ResponseWriter, r *http.Request) { handleV1RelationHistory(a, w, r) })
	mux.HandleFunc(openAPISpecPath, a.schemaAPI.handleV1OpenAPI)
	mux.HandleFunc("/api/v1/_commands", a.commands.handleV1Commands)
	mux.HandleFunc("/api/v1/_transforms", a.export.handleV1Transforms)
	mux.HandleFunc("/api/v1/_comments/", a.comments.handleV1Comments)
	mux.HandleFunc("/api/v1/_piles", a.piles.handleV1Piles)
	mux.HandleFunc("/api/v1/_piles/", a.piles.handleV1Piles)
	mux.HandleFunc(inboxPath, func(w http.ResponseWriter, r *http.Request) {
		a.channels.handleV1Inbox(w, r, a.State().Meta)
	})
	mux.HandleFunc(inboxSummaryPath, func(w http.ResponseWriter, r *http.Request) {
		a.channels.handleV1InboxSummary(w, r)
	})
	mux.HandleFunc(leasesPath, a.leases.handleV1Leases)
	mux.HandleFunc(batchPath, a.batch.handleV1Batch)
	mux.HandleFunc(reconcilePath, a.reconcile.handleV1Reconcile)
	mux.HandleFunc(leasesPath+"/", a.leases.handleV1Leases)
	registerRecordsRoutes(mux, a)
	mux.HandleFunc(flowsPath, a.flows.handleV1Flows)
	mux.HandleFunc(flowsPath+"/", a.flows.handleV1Flows)
	mux.HandleFunc(remotesPath, a.remotes.handleV1Remotes)
	mux.HandleFunc(remotesPath+"/", a.remotes.handleV1Remotes)
	mux.HandleFunc("/api/v1/_templates/", a.schemaAPI.handleV1Templates)
	mux.HandleFunc("/api/v1/_views/", a.views.handleV1Views)
	a.registerCopyRoutes(mux)
	if a.conversions != nil {
		mux.HandleFunc("/api/v1/_conversions/", a.conversions.handleV1Conversions)
	}
	if a.taskItems != nil {
		mux.HandleFunc("/api/v1/_task_item_conversions/", a.taskItems.handleV1TaskItemConversions)
	}
	registerDiagramRoutes(mux, a)
	mux.HandleFunc("/api/v1/_action/", a.write.handleV1Action)
	mux.HandleFunc("/api/v1/_apps/", a.handleV1App)

	// Dynamic entity routes are handled by a catch-all
	mux.HandleFunc("/api/v1/", a.handleV1DynamicRoutes)
}

// registerDiagramRoutes registers the read-only gantt and sankey endpoints.
func registerDiagramRoutes(mux *http.ServeMux, a *App) {
	mux.HandleFunc("/api/v1/_gantts/", a.gantt.handleV1Gantt)
	mux.HandleFunc("/api/v1/_sankeys/", a.sankey.handleV1Sankey)
}

// registerAppearanceRoutes registers the settings, palette and theme routes.
func registerAppearanceRoutes(mux *http.ServeMux, a *App) {
	mux.HandleFunc("/api/v1/_settings", a.appearance.handleAPISettingsCRUD)
	mux.HandleFunc("/api/v1/_palette", a.appearance.handleAPIPaletteCRUD)
	mux.HandleFunc("/api/v1/_theme/logo", a.appearance.handleAPIThemeLogo)
	mux.HandleFunc("/api/v1/_theme/export", a.appearance.handleAPIThemeExport)
	mux.HandleFunc("/api/v1/_theme/import", a.appearance.handleAPIThemeImport)
}

// Path-segment counts for the dynamic route dispatcher below. A
// four-segment path selects a sub-resource collection
// (relations/_actions/_attachments); a five-segment path targets a
// single member within one of those.
const (
	segmentsSubResource   = 4 // /{plural}/{id}/{sub}/{key}
	segmentsSubResourceID = 5 // /{plural}/{id}/{sub}/{key}/{member}
)

// handleV1DynamicRoutes routes requests to the appropriate entity handler
// based on URL. Read operations work against the snapshot returned by
// a.State() with no locking.
func (a *App) handleV1DynamicRoutes(w http.ResponseWriter, r *http.Request) {
	// Skip system routes (already handled)
	path := strings.TrimPrefix(r.URL.Path, "/api/v1/")
	if strings.HasPrefix(path, "_") {
		http.NotFound(w, r)
		return
	}

	// Parse path: {plural}[/{id}[/relations[/{relType}[/{targetId}]]]]
	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) == 0 || parts[0] == "" {
		writeV1Error(w, r, http.StatusNotFound, "not_found", "Resource not found", "")
		return
	}

	plural := parts[0]

	typeName := entityTypeForPlural(a.State().Meta, plural)
	if typeName == "" {
		writeV1Error(w, r, http.StatusNotFound, "unknown_type", "Unknown entity type", "")
		return
	}

	// /{plural}/{ref}/_channels[/...] (TKT-ZU7P13) has routes of several
	// depths, so it is matched before the switch on depth.
	if rest, ok := splitChannelRoute(parts); ok {
		a.channels.serve(w, r, typeName, parts[1], rest)
		return
	}

	switch len(parts) {
	case 1:
		// /{plural} - collection
		a.entities.handleV1EntityCollection(w, r, typeName, plural)
	case 2:
		// /{plural}/_export (collection export) or /{plural}/{id} (single entity).
		// _export is a reserved segment; no entity id may collide (ids never
		// start with the reserved underscore).
		if parts[1] == "_export" {
			a.export.handleV1ExportList(w, r, typeName)
		} else {
			a.entities.handleV1SingleEntity(w, r, typeName, plural, parts[1])
		}
	case 3:
		// /{plural}/{id}/relations, /{plural}/{id}/_export, /{plural}/{id}/restore,
		// /{plural}/{id}/mentioned-by, /{plural}/{id}/_lease or /{plural}/{id}/_presence
		switch parts[2] {
		case "relations":
			a.entities.handleV1EntityRelations(w, r, typeName, parts[1])
		case "_lease":
			a.leases.handleClaim(w, r, typeName, parts[1])
		case "_presence":
			handlePresence(w, r, a.visibleReader, a.live.presence, typeName, parts[1])
		case "_export":
			a.export.handleV1ExportEntity(w, r, typeName, parts[1])
		case "restore":
			a.write.handleV1RestoreEntity(w, r, typeName, parts[1])
		case "mentioned-by":
			handleV1MentionedBy(a, w, r, typeName, parts[1])
		default:
			writeV1Error(w, r, http.StatusNotFound, "not_found", "Resource not found", "")
		}
	case segmentsSubResource:
		// /{plural}/{id}/relations/{relType}, /{plural}/{id}/_actions/{action},
		// or /{plural}/{id}/_attachments/{property}
		switch parts[2] {
		case "relations":
			a.entities.handleV1EntityRelationType(w, r, typeName, parts[1], parts[3])
		case "_actions":
			a.entities.handleV1EntityAction(w, r, typeName, parts[1], parts[3])
		case "_attachments":
			// The segment is an ADDRESS: bytes are per entity, but the files
			// a face lists and serves are its own (BUG-CTUW2N).
			a.attachments.handleV1AttachmentRoute(w, r, typeName, plural, parts[1], parts[3])
		default:
			writeV1Error(w, r, http.StatusNotFound, "not_found", "Resource not found", "")
		}
	case segmentsSubResourceID:
		// /{plural}/{id}/relations/{relType}/{targetId} or
		// /{plural}/{id}/_attachments/{property}/{fileName}
		switch parts[2] {
		case "relations":
			a.entities.handleV1RelationTarget(w, r, typeName, parts[1], parts[3], parts[4])
		case "_attachments":
			a.attachments.handleV1AttachmentFileRoute(w, r, typeName, parts[1], parts[3], parts[4])
		default:
			writeV1Error(w, r, http.StatusNotFound, "not_found", "Resource not found", "")
		}
	default:
		writeV1Error(w, r, http.StatusNotFound, "not_found", "Resource not found", "")
	}
}

// --- List Pipeline Errors ---

// errACLListQuery wraps a store.GraphQuery failure during ACL list
// filtering so call sites can route it through writeGateError (500
// acl_query_failed / 504 / silent-on-cancel) instead of mislabeling
// it as a free-text search failure.
var errACLListQuery = errors.New("acl list query failed")

// errBadFilter marks a filter[...] query param the pipeline refuses to
// evaluate: malformed key shape or an operator outside the supported set.
// Mapped to HTTP 400 by writeListPipelineError. A rejected request beats
// the old behavior of dropping the clause with only a server-side log —
// that silently returned the UNFILTERED superset, which let a config typo
// (`=~`, BUG: active_tickets) masquerade as an empty/wrong result set for
// months. Malformed wire format is a hard 400 per DEC-HWZHA.
var errBadFilter = errors.New("invalid filter parameter")

// errListLoad wraps a store iterator failure while loading the
// unfiltered (AllowAll) list. Surfaced as 500 list_load_failed at the
// call sites: before TKT-VMD8 a mid-stream error silently truncated
// the result, which under ACL would make the AllowAll and Query paths
// observably asymmetric (truncated-200 vs 500) for the same backend
// fault — and a truncated list with authoritative-looking pagination
// is worse than an error either way.
var errListLoad = errors.New("list load failed")

// relationFilterClassifier returns a predicate that decides, for a bare filter
// param name (the property/relation segment, no operator), whether it should be
// handled by the direction-aware relation pass rather than applyV1Filters.
//
// Relation filtering is restricted to configured filter_controls (RR-B0JPPL):
// a `filter[<rel>]` param routes to the relation pass ONLY when a relation
// FilterControl on a list or kanban of this entity type configures it. An arbitrary
// metamodel relation with no control is NOT filterable and falls through to
// applyV1Filters (where, absent a matching property, it fails closed rather
// than silently widening the set).
//
// The config discriminator also resolves the property/relation NAME COLLISION
// (RR-0HWAS0): properties (per-type) and relations (global) are disjoint
// namespaces, so a property can share a name with a relation. An explicit
// property FilterControl for the name forces the property pass; a name that is
// only ever a relation control routes to the relation pass. When a name is both
// a property of this type and a relation control with no disambiguating
// property control, the property wins (safe, backward-compatible) and
// CollectConfigWarnings flags the ambiguity at load.
//
// A free function (not an App method) taking the already-loaded meta/cfg
// snapshot: it needs no other App state, and keeping it off the receiver keeps
// App under its plimsoll method cap.
func relationFilterClassifier(
	meta *metamodel.Metamodel,
	cfg *dataentryconfig.Config,
	typeName string,
) func(string) bool {
	return func(key string) bool {
		// Explicit property control → property pass.
		if cfg.HasPropertyFilterControl(typeName, key) {
			return false
		}
		// A relation control is required for relation filtering.
		if _, ok := cfg.RelationFilterDirection(typeName, key); !ok {
			return false
		}
		// Relation control exists, but the collision rule: if the name is
		// also a property of this type (and no property control disambiguated
		// above), the property wins.
		if entDef, ok := meta.GetEntityDef(typeName); ok {
			if _, isProp := entDef.Properties[key]; isProp {
				return false
			}
		}
		return true
	}
}

// matchRelationFilterMany evaluates a relation filter for every row at once
// (TKT-1U8XYN): one relation query for all rows, one content-free header
// read for every distinct neighbor, one gate probe per neighbor type. It
// replaced a per-row helper that ran one relation query plus one entity load
// per neighbor for each row. The verdict per row is unchanged — the row
// matches when at least one READABLE neighbor over the relation carries the
// wanted display title (RR-HK1XNO) — but the cost no longer grows with the
// type's row count times its fan-out. A free function taking its read seam,
// not an App method — App sits at its load line (see worldneighbors.go).
//
// A store failure is returned, never folded into an empty verdict map: an
// empty map reads as "no row matches", which for a `ne` filter admits EVERY
// row — a backend fault would silently widen a filter whose job is to narrow.
//
// The title compared is the one the principal is shown: each served neighbor
// is redacted once with red before its display title is taken. The served
// headers are gated but not redacted, so comparing their raw title would let
// `filter[<relation>]=<guess>` confirm a display property that `visible:`
// hides on the neighbor (BUG-WJSYQT review). The field gate does not cover
// this key, because it names a relation, not a property of the listed type.
func matchRelationFilterMany(
	ctx context.Context, svc Services, visible visibleReader, red visibility.FieldRedactor,
	rows []*entityPkg.Entity, relation string, direction dataentryconfig.Direction, want string,
) (map[string]bool, error) {
	matched := make(map[string]bool, len(rows))
	if len(rows) == 0 {
		return matched, nil
	}
	edges, neighborIDs, err := relationFilterEdges(ctx, svc, rows, relation, direction)
	if err != nil || len(neighborIDs) == 0 {
		return matched, err
	}

	// Each neighbor is read at the face the principal is served: the ACL
	// trims its faces, then the world ranks what is left (servedIDsErr).
	// Only that face's title is compared. Comparing another face's title,
	// or a face the reader may not read, turns the filter into an oracle on
	// that face (BUG-ISJHML), and a per-id "some face is readable" verdict
	// is not enough to rule that out once grants differ per face. The
	// served face is also the one an incoming content-scoped edge must be
	// owned by.
	served, err := visible.servedIDsErr(ctx, neighborIDs)
	if err != nil {
		return nil, err
	}
	served = redactHeaders(ctx, red, served)
	for _, re := range edges {
		h, ok := served[re.targetID]
		if !ok || svc.Meta.DisplayTitle(h.ID, h.Type, h.Properties) != want {
			continue
		}
		if direction.IsIncoming() && !ownedByFace(svc.Meta, re.rel, h.Face) {
			continue
		}
		matched[re.rowID] = true
	}
	return matched, nil
}

// redactHeaders redacts each header once with red, priming any `related(...)`
// verdicts for the whole batch first so the cost does not grow per header.
func redactHeaders(
	ctx context.Context, red visibility.FieldRedactor, headers map[string]store.EntityHeader,
) map[string]store.EntityHeader {
	probes := make([]*entityPkg.Entity, 0, len(headers))
	for _, h := range headers {
		probes = append(probes, &entityPkg.Entity{ID: h.ID, Type: h.Type, Face: h.Face, Properties: h.Properties})
	}
	ctx = visibility.PrimeTraversals(ctx, red, probes)
	out := make(map[string]store.EntityHeader, len(headers))
	for id, h := range headers {
		out[id] = visibility.RedactHeader(ctx, red, h)
	}
	return out
}

// relationFilterEdge is one edge of a relation filter, oriented from the
// filtered row to its neighbor.
type relationFilterEdge struct {
	rowID, targetID string
	rel             *entityPkg.Relation
}

// relationFilterEdges reads the edges of every row over relation in ONE
// query, and returns them with the distinct neighbor ids. An outgoing
// content-scoped edge of another face of the row is dropped: matching on it
// would tell a reader what a face they were not served links to
// (BUG-ISJHML). Incoming edges are checked by the caller, which knows the
// neighbor's face.
func relationFilterEdges(
	ctx context.Context, svc Services, rows []*entityPkg.Entity,
	relation string, direction dataentryconfig.Direction,
) ([]relationFilterEdge, []string, error) {
	ids := make([]string, 0, len(rows))
	rowFace := make(map[string]entityPkg.Face, len(rows))
	for _, e := range rows {
		ids = append(ids, e.ID)
		rowFace[e.ID] = e.Face
	}
	q := store.RelationQuery{EntityIDs: ids, Type: relation, Direction: relationDirection(direction)}
	var edges []relationFilterEdge
	var neighborIDs []string
	seen := make(map[string]struct{})
	for r, err := range svc.Store.ListRelations(ctx, q) {
		if err != nil {
			return nil, nil, err
		}
		rowID, targetID := r.From, r.To
		if direction.IsIncoming() {
			rowID, targetID = r.To, r.From
		} else if !ownedByFace(svc.Meta, r, rowFace[rowID]) {
			continue
		}
		edges = append(edges, relationFilterEdge{rowID: rowID, targetID: targetID, rel: r})
		if _, dup := seen[targetID]; !dup {
			seen[targetID] = struct{}{}
			neighborIDs = append(neighborIDs, targetID)
		}
	}
	return edges, neighborIDs, nil
}

// filterKey is a parsed `filter[...]` query key.
type filterKey struct {
	// property is the property or relation the key names.
	property string
	// operator defaults to "eq".
	operator string
	// arrayForm records an explicit `[]` suffix: each repeated param is one
	// member, verbatim. Without it, a lone param is the comma form. The
	// suffix, not the number of params, is the signal, since a one-element
	// array is indistinguishable from a comma list by count.
	arrayForm bool
}

// parseFilterKey parses `filter[property]`, `filter[property][operator]` or
// `filter[property][operator][]`. It is the ONE parser of the key shape: the
// field gate, the Go filter, the relation pass and the pushdown all read keys
// through it, so the gate checks exactly the property each of them evaluates
// (BUG-WJSYQT).
//
// A malformed key like `filter[prop][][weird]` produces three segments; more
// than two segments, or an empty property or operator, returns errBadFilter.
// On an empty operator the property is still set, so a caller can route a
// relation key before refusing.
func parseFilterKey(key string) (filterKey, error) {
	inner := strings.TrimSuffix(strings.TrimPrefix(key, "filter["), "]")
	trimmed := strings.TrimSuffix(inner, "][") // was "...[]"
	fk := filterKey{operator: "eq", arrayForm: trimmed != inner}
	parts := strings.Split(trimmed, "][")
	if len(parts) > 2 {
		return filterKey{}, fmt.Errorf("%w: key %q has too many segments", errBadFilter, key)
	}
	fk.property = parts[0]
	if fk.property == "" {
		return filterKey{}, fmt.Errorf("%w: key %q has an empty property", errBadFilter, key)
	}
	if len(parts) == 2 {
		if parts[1] == "" {
			return fk, fmt.Errorf("%w: key %q has an empty operator segment", errBadFilter, key)
		}
		fk.operator = parts[1]
	}
	return fk, nil
}

// parseRelationFilterKey is [parseFilterKey] for a caller that only needs to
// know whether the key is well formed. Returns (relation, operator, ok) with
// operator defaulting to "eq"; an operator segment is recognized rather than
// being swallowed into the relation name (RR-6RF60V).
func parseRelationFilterKey(key string) (relation, operator string, ok bool) {
	fk, err := parseFilterKey(key)
	if err != nil {
		return "", "", false
	}
	return fk.property, fk.operator, true
}

// queryGet returns the first value for key from a raw query map, or "".
// url.Values.Get over a plain map[string][]string without allocating.
func queryGet(query map[string][]string, key string) string {
	if vals, ok := query[key]; ok && len(vals) > 0 {
		return vals[0]
	}
	return ""
}

// setListHeaders sets the pagination headers of a list response: the Link
// header (RFC 5988) and the X-Total-Count, X-Page and X-Per-Page counts.
func setListHeaders(w http.ResponseWriter, r *http.Request, page, perPage, total int, plural string) {
	addPaginationLinks(w, r, page, perPage, total, plural)
	w.Header().Set("X-Total-Count", strconv.Itoa(total))
	w.Header().Set("X-Page", strconv.Itoa(page))
	w.Header().Set("X-Per-Page", strconv.Itoa(perPage))
}

// --- Read Path Errors ---

// entityNotFoundTitle is the title for EVERY read-path 404 — the ACL
// deny path ([gateReadOr]), a genuinely missing entity, and a
// missing attachment all share it so a denied read is byte-identical to
// a nonexistent one (the RR-NGMI indistinguishability invariant). Any
// handler that 404s on the read path MUST use this const, not a fresh
// literal, or the bodies drift and existence leaks.
const entityNotFoundTitle = "Entity not found"

// gateReadOr runs the per-entity ACL read gate (PermitsRead). On deny it
// calls deny, which writes the miss answer; on a store or timeout failure it
// writes the 5xx via writeGateError. It returns true when the handler should
// continue, false when a response was already written.
//
// deny must answer every denied read the same way as a nonexistent entity
// (RR-NGMI). The `_views` route passes the uniform 404, or a redirect for a
// retired id whose successor the caller may read ([redirectRetired]).
//
//archplan:guard=authz reason="per-entity read gate: a denied read is the uniform miss"
func gateReadOr(w http.ResponseWriter, r *http.Request, typeName, entityID string, deny func()) bool {
	ok, err := readGateFromContext(r.Context()).PermitsRead(r.Context(), typeName, entityID)
	if err != nil {
		writeGateError(w, r, err)
		return false
	}
	if !ok {
		deny()
		return false
	}
	return true
}

// writeListPipelineError maps a scopedSortedEntities / resolveScope
// error to the right HTTP shape: ACL query failures route through
// writeGateError, store-load failures surface as list_load_failed,
// and anything else is the free-text search failure the pipeline
// always surfaced. Shared by the list and _position handlers so the
// two consumers of the pipeline can't drift.
//
// All branches log the raw error server-side and keep it out of the
// response body (RR-372L / IB-review PR939 #1): a store or search
// backend error string can name tables, columns, or index paths.
func writeListPipelineError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case requestCanceled(r, err):
		// The client went away: nobody reads the answer, and a cancel is not
		// a fault (TKT-T2I7P2). The access log records it as 499.
		return
	case errors.Is(err, errBadFilter):
		// Server-side log stays alongside the 400 (RR-6RF60V's logging
		// intent, CONTROL-8-15): the response informs the caller, the log
		// informs operators — a broken config or a caller probing filter
		// params is an anomalous event worth a trace even when no one
		// watches the client.
		slog.Warn("dataentry: rejected invalid filter parameter",
			"err", err, "path", r.URL.Path, "method", r.Method)
		// Client-caused: echo the detail — it only restates the caller's own
		// filter key/operator, never store internals.
		writeV1Error(w, r, http.StatusBadRequest, "invalid_filter",
			"Invalid filter parameter", err.Error())
	case errors.Is(err, errPropertyNotVisible):
		// Client-caused: the caller filtered or sorted on a property its
		// roles do not show on every row (BUG-WJSYQT). The detail names the
		// property and the type, which are config, not secrets, and depends
		// on no row (see listfieldgate.go).
		problem := v1.Error{
			Type:     "https://rela.dev/errors/property_not_visible",
			Title:    "Cannot filter or sort on this property",
			Status:   http.StatusBadRequest,
			Detail:   err.Error(),
			Instance: r.URL.Path,
		}
		if refusal, ok := errors.AsType[*propertyNotVisibleError](err); ok {
			problem.Use, problem.Property = refusal.use, refusal.property
		}
		w.Header().Set("Content-Type", "application/problem+json")
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(problem)
	case errors.Is(err, errBadViewID):
		slog.Warn("dataentry: rejected request naming two views",
			"path", r.URL.Path, "method", r.Method)
		writeV1Error(w, r, http.StatusBadRequest, "invalid_view",
			"Invalid view parameter", err.Error())
	case errors.Is(err, errBadQueryScope):
		// Client-caused, like errBadFilter above: the caller named a scope
		// that does not resolve, or repeated the parameter. Echo the detail
		// — it names only the caller's own scope name and the entity type,
		// both of which are operator-authored config rather than secrets.
		slog.Warn("dataentry: rejected invalid query_scope",
			"err", err, "path", r.URL.Path, "method", r.Method)
		writeV1Error(w, r, http.StatusBadRequest, "invalid_query_scope",
			"Invalid query_scope parameter", err.Error())
	case errors.Is(err, errPageAnchorNotFound):
		// The same response an entity GET gives for a hidden or missing id.
		writeV1Error(w, r, http.StatusNotFound, "not_found", entityNotFoundTitle, "")
	case errors.Is(err, errACLListQuery):
		writeGateError(w, r, err)
	case errors.Is(err, acl.ErrTraversalUnsupported):
		// The scope's related() cannot be evaluated under this principal's
		// grants (a named-face or inherited read, a field the client ceiling
		// hides, a chain the gate cannot place). The cause is the policy and
		// the schema, never a row, so the detail names config only, which is
		// not confidential (see CLAUDE.md), and helps the operator fix it.
		slog.Warn("dataentry: query scope traversal unsupported for principal",
			"err", err, "path", r.URL.Path, "method", r.Method)
		writeV1Error(w, r, http.StatusUnprocessableEntity, "query_scope_unsupported",
			"Query scope cannot be evaluated for this principal", err.Error())
	case errors.Is(err, errListLoad):
		slog.Warn("dataentry: list load failed",
			"err", err, "path", r.URL.Path, "method", r.Method)
		writeV1Error(w, r, http.StatusInternalServerError, "list_load_failed",
			"Loading entities failed", "check server logs")
	default:
		slog.Warn("dataentry: list free-text search failed",
			"err", err, "path", r.URL.Path, "method", r.Method)
		writeV1Error(w, r, http.StatusInternalServerError, "search_failed",
			"Free-text search failed", "check server logs")
	}
}

// writeGateError maps a readGate.PermitsRead / ReadableFacesMany error
// to the right HTTP shape: client-disconnect emits nothing,
// deadline-exceeded is 504, everything else is 500 with the
// acl_query_failed code (RR-89XK). Centralized so every gate call
// site handles the error shape the same way.
//
// The raw error is logged server-side and never echoed in the
// response body (same RR-372L rationale as attachACLRequest): a
// backend error string can carry table/column names or other
// internals an API client has no business seeing.
func writeGateError(w http.ResponseWriter, r *http.Request, err error) {
	if requestCanceled(r, err) {
		return
	}
	if writeNoIdentity(w, r, err) {
		return
	}
	slog.Warn("acl: read-gate query failed",
		"err", err, "path", r.URL.Path, "method", r.Method)
	if errors.Is(err, context.DeadlineExceeded) {
		writeV1Error(w, r, http.StatusGatewayTimeout, "acl_query_timeout",
			"ACL read-permission check timed out", "check server logs")
		return
	}
	writeV1Error(w, r, http.StatusInternalServerError, "acl_query_failed",
		"ACL read-permission check failed", "check server logs")
}

// writeNoIdentity answers 401 when err reports that the request reached code
// below the edge with no bound identity ([acl.ErrNoIdentity]). It reports
// whether it wrote a response.
//
// Every shared error mapper in this package calls it first (writeGateError,
// writeInternalProblem, writeInternalJSONError, and writeIdentityFaultDeny for
// a denied write), so the mapping lives in one place rather than in each
// handler. The body is the one the JWT gate sends for a missing assertion.
// It carries no WWW-Authenticate challenge: that names the JWT gate's
// credential header, and a deployment without the gate has no scheme to name.
//
// In production attachACLRequest binds a Request on every /api request or
// fails it, so reaching this means a route or a ctx lost the binding. That is
// a wiring fault, hence the Error log, but the client is told what is true for
// it: the request is not authenticated.
//
// A mismatched or foreign identity ([acl.ErrIdentityMismatch],
// [acl.ErrForeignIdentity]) is not handled here. The caller did authenticate;
// the server attached the wrong Request, which is a server fault and stays 500.
//
// Nothing here resolves the principal as a fallback. The edge binds it or the
// request fails.
func writeNoIdentity(w http.ResponseWriter, r *http.Request, err error) bool {
	if !errors.Is(err, acl.ErrNoIdentity) {
		return false
	}
	slog.ErrorContext(r.Context(), "acl: request reached a handler with no bound identity",
		"err", err, "path", r.URL.Path, "method", r.Method)
	writeV1Error(w, r, http.StatusUnauthorized, "unauthenticated", "Authentication required", "")
	return true
}

// conflictBlockTitle is the title of every conflict_block problem.
const conflictBlockTitle = "Content holds a git conflict block"

// writeConflictBlock answers a write the file store refused because its
// content holds a git conflict block ([store.ErrConflictBlock]), and reports
// whether it did. The store cannot persist that content, which the
// write-validation policy answers with a 422. The detail is the sentinel's
// fixed text: it names the fix and never echoes the content (BUG-FJ1ITQT).
// [writeInternalProblem] calls it, so no write path answers it with a 500.
func writeConflictBlock(w http.ResponseWriter, r *http.Request, err error) bool {
	if !errors.Is(err, store.ErrConflictBlock) {
		return false
	}
	writeV1Error(w, r, http.StatusUnprocessableEntity, "conflict_block", conflictBlockTitle,
		store.ErrConflictBlock.Error())
	return true
}

// requestCanceled reports whether the client went away: the request's own
// context was canceled. Once it is, no one reads the response, so the error
// is answered with silence and not logged.
//
// The request context decides, not the error. The error does not always say
// it is a cancel (pgx reports a statement stopped by a cancel request as
// SQLSTATE 57014, which does not wrap context.Canceled), and an error that
// does wrap context.Canceled may come from another context, such as a
// shared singleflight call, while this client still waits for an answer.
//
// A deadline (context.DeadlineExceeded) is not a cancel: the server gave
// up, and that is still logged and answered.
func requestCanceled(r *http.Request, _ error) bool {
	return errors.Is(r.Context().Err(), context.Canceled)
}

// internalErrorDetail is the detail of every 500 this package answers.
const internalErrorDetail = "check server logs"

// writeInternalError answers a 500 for an internal failure. The cause goes
// to the server log and the client gets [internalErrorDetail]: a store, file
// or database error text can name paths, tables or rows the caller may not
// read (GitHub #1774). TestNoInternalErrorDetail rejects a 500 whose detail
// is not a literal.
func writeInternalError(w http.ResponseWriter, r *http.Request, code, title string, err error) {
	writeInternalErrorDetail(w, r, code, title, internalErrorDetail, err)
}

// writeInternalErrorDetail is [writeInternalError] with a detail the caller
// built from the request alone, never from err.
func writeInternalErrorDetail(w http.ResponseWriter, r *http.Request, code, title, detail string, err error) {
	writeInternalProblem(w, r, code, v1.Error{Title: title, Detail: detail}, err)
}

// writeInternalProblem is [writeInternalErrorDetail] for a problem with
// extension fields. Like the detail, they are built from the request and
// what it wrote, never from err. It sets the type, status and instance.
func writeInternalProblem(w http.ResponseWriter, r *http.Request, code string, problem v1.Error, err error) {
	if writeNoIdentity(w, r, err) || writeConflictBlock(w, r, err) {
		return
	}
	if !logInternalError(r, code, err) {
		return
	}
	problem.Type = "https://rela.dev/errors/" + code
	problem.Status = http.StatusInternalServerError
	problem.Instance = r.URL.Path
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(problem.Status)
	_ = json.NewEncoder(w).Encode(problem)
}

// writeInternalJSONError is [writeInternalError] for the settings and theme
// endpoints, which answer with the plain {"error": message} body.
func writeInternalJSONError(w http.ResponseWriter, r *http.Request, message string, err error) {
	if writeNoIdentity(w, r, err) {
		return
	}
	if !logInternalError(r, message, err) {
		return
	}
	writeJSONError(w, http.StatusInternalServerError, message+"; "+internalErrorDetail)
}

// logInternalError logs the cause of a 500. It reports false when the client
// has gone away, in which case there is no one to answer.
func logInternalError(r *http.Request, code string, err error) bool {
	if requestCanceled(r, err) {
		return false
	}
	slog.ErrorContext(r.Context(), "dataentry: request failed",
		"code", code, "err", err, "path", r.URL.Path, "method", r.Method)
	return true
}

// --- Relation Helpers ---

// relationMetaStrip is a deferred relation-meta redaction (TKT-B1F5Q1): the built
// wire `rel` map plus what is needed to resolve the source's `visible:` grants.
// The strip runs AFTER sortRelationGroup because the sort reads the managed order
// property out of `meta`, so redacting a (possibly hidden) order key first would
// break ordering. An outgoing edge's source is the path entity; an incoming edge's
// source is the peer's row at the edge's tail (peer), resolved fail-closed at
// strip time.
type relationMetaStrip struct {
	rel      map[string]any
	incoming bool
	peer     entityPkg.Ref // incoming only: the source (from) row, at the edge's tail
}

// sortRelationGroup sorts a relation group in place by a numeric meta key,
// with [metamodel.CompareOrderKeys] so the entity page and every
// relation-ordered list agree on one order.
func sortRelationGroup(group []map[string]any, prop string) {
	if len(group) < 2 || prop == "" {
		return
	}
	key := func(m map[string]any) metamodel.OrderKey {
		k := metamodel.OrderKey{}
		if meta, ok := m["meta"].(map[string]any); ok {
			k.Value = meta[prop]
		}
		k.Peer, _ = m["id"].(string)
		return k
	}
	slices.SortStableFunc(group, func(x, y map[string]any) int {
		return metamodel.CompareOrderKeys(key(x), key(y))
	})
}

// resolveRelationEndpoints returns the from/to entity IDs for a relation operation,
// swapping them when direction is incoming.
func resolveRelationEndpoints(entityID, peerID, direction string) (from, to string) {
	if direction == string(DirectionIncoming) {
		return peerID, entityID
	}
	return entityID, peerID
}

// --- Search Helpers ---

// maxSearchLimit bounds the `limit` parameter of /_search. It matches the
// list endpoint's per_page ceiling.
const maxSearchLimit = 100

// parseSearchLimit reads the optional `limit` parameter of /_search. An empty
// value means no limit and returns 0. The second result is false for a value
// that is not an integer in [1, maxSearchLimit].
//
// The editor's mention menu is the motivating caller: `sort:modified:desc`
// with a small limit lists recently modified entities without shipping every
// visible row to the browser.
func parseSearchLimit(raw string) (int, bool) {
	if raw == "" {
		return 0, true
	}
	n, err := strconv.Atoi(raw)
	if err != nil || n < 1 || n > maxSearchLimit {
		return 0, false
	}
	return n, true
}

// --- Helper Functions ---

// applyV1Filters applies `filter[...]` query params to the entity slice. Pure
// data transform — a free function, not App behavior (TKT-N26KLB M5.5). It
// takes a [gatedListQuery], so it never compares a property the principal
// cannot see on every row (BUG-WJSYQT).
// A malformed key or unsupported operator returns errBadFilter (HTTP 400)
// instead of silently dropping the clause, which returned the unfiltered
// superset and hid config/caller typos.
//
// The query's isRelationKey identifies filter keys that target a metamodel
// relation rather than an entity property; those are skipped here and handled
// by the relation pass in scopedSortedEntities (which has ctx + config for
// direction-aware traversal). A relation key would otherwise match no property
// and nuke the whole result set (TKT-5U7QBR). A nil predicate treats every key
// as a property filter (the pre-TKT-5U7QBR behavior).
//
//nolint:gocognit // dispatches over every supported filter operator and value type; each branch is one operator's semantics, not extractable shared logic.
func applyV1Filters(entities []*entityPkg.Entity, query gatedListQuery) ([]*entityPkg.Entity, error) {
	filtered := entities

	for key, values := range query.params {
		if !strings.HasPrefix(key, "filter[") || len(values) == 0 {
			continue
		}

		fk, err := parseFilterKey(key)
		// Relation filter keys are handled by the direction-aware relation
		// pass in scopedSortedEntities. Skip them here so a relation key
		// (which matches no property) doesn't nuke the result set. A key with
		// too many segments has no property and is refused below.
		if fk.property != "" && query.isRelationKey != nil && query.isRelationKey(fk.property) {
			continue
		}
		// Reject a malformed key: the old skip-with-a-log "fail closed" still
		// returned the unfiltered superset, which is fail-open at the result
		// level.
		if err != nil {
			return nil, err
		}
		property, operator, isArrayForm := fk.property, fk.operator, fk.arrayForm

		// Reject unknown operators BEFORE the per-entity loop. A typo like
		// `filter[status][equals]=done` used to be skipped with only a log —
		// which returned every entity, the very silent inclusion the skip
		// claimed to prevent. A 400 makes the typo visible to the caller
		// (and to a config author whose list filter reached the wire).
		switch operator {
		case "eq", "ne", "contains", "in", "lt", "lte", "gt", "gte":
			// known
		default:
			return nil, fmt.Errorf("%w: unknown operator %q in key %q (supported: eq, ne, contains, in, lt, lte, gt, gte)",
				errBadFilter, operator, key)
		}

		// Multi-value support: `in`/`ne` collect ALL repeated values from the
		// query (e.g. `filter[tags][in][]=a&filter[tags][in][]=b`). Other
		// operators stay last-write-wins on values[len-1] for predictability.
		//
		// setValues keeps the repeated params as DISTINCT members rather than
		// joining them into one comma string: a value may legitimately contain
		// a comma ("Legal, Risk & Compliance"), and join-then-split would tear
		// it into two members that match the wrong rows — and make `ne` fail to
		// exclude the row it names, the same wrong-answer class as BUG-AMK38R.
		// A single param still splits on commas, which is the documented
		// `filter[x][in]=a,b` form; that form simply cannot express a value
		// containing a comma, so the repeated form is the one to use for those.
		var value string
		var setValues []string
		if operator == "in" || operator == "ne" {
			value = resolveFilterVariablesInList(strings.Join(values, ","), query.user, query.now)
			setValues = filterSetValues(values, isArrayForm, query.user, query.now)
		} else {
			value = resolveFilterVariable(values[len(values)-1], query.user, query.now)
		}

		var newFiltered []*entityPkg.Entity
		for _, e := range filtered {
			propVal, ok := e.Properties[property]
			if !ok {
				if operator == "eq" && value == "" {
					newFiltered = append(newFiltered, e)
				}
				continue
			}

			// A list-typed property is compared element-wise: ANY element
			// matching satisfies eq/in/contains, and ne is the exact
			// complement (NO element matches). A scalar is a one-element
			// list, so scalar behavior is unchanged. See propertyElements.
			switch operator {
			case "eq":
				if anyElement(propVal, func(el string) bool { return el == value }) {
					newFiltered = append(newFiltered, e)
				}
			case "ne":
				// Support comma-separated values as NOT IN
				excluded := anyElement(propVal, func(el string) bool {
					return slices.Contains(setValues, el)
				})
				if !excluded {
					newFiltered = append(newFiltered, e)
				}
			case "contains":
				needle := strings.ToLower(value)
				matches := anyElement(propVal, func(el string) bool {
					return strings.Contains(strings.ToLower(el), needle)
				})
				if matches {
					newFiltered = append(newFiltered, e)
				}
			case "in":
				matches := anyElement(propVal, func(el string) bool {
					return slices.Contains(setValues, el)
				})
				if matches {
					newFiltered = append(newFiltered, e)
				}
			case "lt", "lte", "gt", "gte":
				// Ordered comparison is scalar-only. What it means to order a
				// LIST against a bound is a genuine design question, so this
				// declines to answer rather than guessing: the old code
				// compared the Go slice literal, which made `tags < "zzz"`
				// true for every row (including empty and null ones) — a
				// confident wrong answer, the same class as BUG-AMK38R.
				// Excluding the row is the fail-closed direction and leaves
				// the semantics free to be defined later.
				if isListValue(propVal) {
					slog.Warn("ordered filter operator on a list property; no rows match",
						"property", property, "operator", operator)
					continue
				}
				match, err := compareValues(fmt.Sprintf("%v", propVal), value, operator)
				if err != nil {
					// Type mismatch (e.g. property is a date, filter value isn't).
					// Exclude the entity rather than silently lying via lexicographic
					// fallback. Log so users notice.
					slog.Warn("filter compare error", "property", property, "error", err)
					continue
				}
				if match {
					newFiltered = append(newFiltered, e)
				}
			}
		}
		filtered = newFiltered
	}

	return filtered, nil
}

// parseSortParam parses the `sort=` query param into ordered sort specs:
// "-created,title" means descending created, then ascending title. Returns nil
// when no sort was requested.
//
// Shared by every consumer of the grammar — the list pipeline that APPLIES the
// sort and the export context that REPORTS it — so the two cannot drift into
// describing different orderings.
func parseSortParam(query map[string][]string) []filter.SortSpec {
	sortParam := queryGet(query, "sort")
	if sortParam == "" {
		return nil
	}
	var specs []filter.SortSpec
	for part := range strings.SplitSeq(sortParam, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		spec := filter.SortSpec{Direction: "asc"}
		if after, found := strings.CutPrefix(part, "-"); found {
			spec.Direction = "desc"
			part = after
		}
		spec.Property = part
		specs = append(specs, spec)
	}
	return specs
}

// applyV1Sorting applies `sort=` query params to the entity slice. Pure data
// transform — a free function, not App behavior (TKT-N26KLB M5.5).
//
// The ordering rule lives in [filter.QuerySort] so this path, the search and
// dashboard path, and every store backend share one definition
// (TKT-9OFGH4). It is store.GraphQuery.OrderBy's contract: byte-wise on the
// text form, enum values by declared position, absent rows as the largest
// value, id ascending as the final tiebreak — so a request reads the same
// whether or not it was pushed into the database.
//
// meta may be nil; that yields byte-wise comparison for every property, which
// is the right answer when no schema is available. The query is a
// [gatedListQuery], so a sort key is never a property hidden from the
// principal on some row (BUG-WJSYQT).
func applyV1Sorting(
	entities []*entityPkg.Entity, query gatedListQuery, meta *metamodel.Metamodel,
) []*entityPkg.Entity {
	sortSpecs := parseSortParam(query.params)
	if len(sortSpecs) == 0 {
		return entities
	}

	sorted := make([]*entityPkg.Entity, len(entities))
	copy(sorted, entities)
	filter.QuerySortApply(newEntityQuerySort(sortSpecs, sorted, meta), sorted, entityRecord, sortSpecs)
	return sorted
}

// newEntityQuerySort resolves declared value orders for the types actually
// present in rows, so a mixed-type result set ranks each property against
// every definition that declares it.
func newEntityQuerySort(
	specs []filter.SortSpec, rows []*entityPkg.Entity, meta *metamodel.Metamodel,
) *filter.QuerySort {
	if meta == nil {
		return filter.NewQuerySort(specs, nil, nil)
	}
	defs := make(map[string]*metamodel.EntityDef)
	for _, e := range rows {
		if _, seen := defs[e.Type]; seen {
			continue
		}
		if def, ok := meta.GetEntityDef(e.Type); ok {
			defs[e.Type] = def
		}
	}
	return filter.NewQuerySort(specs, defs, meta)
}

func parseV1Pagination(query map[string][]string) (page, perPage int) {
	page = 1
	perPage = 25

	if vals, ok := query["page"]; ok && len(vals) > 0 {
		if p, err := strconv.Atoi(vals[0]); err == nil && p > 0 {
			page = p
		}
	}

	if vals, ok := query["per_page"]; ok && len(vals) > 0 {
		if pp, err := strconv.Atoi(vals[0]); err == nil && pp > 0 && pp <= 100 {
			perPage = pp
		}
	}

	return page, perPage
}

// addPaginationLinks writes RFC 8288 Link headers for the collection page.
// Pure data transform over its args — a free function (TKT-N26KLB M5.5).
func addPaginationLinks(w http.ResponseWriter, _ *http.Request, page, perPage, total int, plural string) {
	totalPages := (total + perPage - 1) / perPage
	if totalPages == 0 {
		totalPages = 1
	}

	baseURL := "/api/v1/" + plural
	var links []string

	// First
	links = append(links, fmt.Sprintf("<%s?page=1&per_page=%d>; rel=\"first\"", baseURL, perPage))

	// Prev
	if page > 1 {
		links = append(links, fmt.Sprintf("<%s?page=%d&per_page=%d>; rel=\"prev\"", baseURL, page-1, perPage))
	}

	// Next
	if page < totalPages {
		links = append(links, fmt.Sprintf("<%s?page=%d&per_page=%d>; rel=\"next\"", baseURL, page+1, perPage))
	}

	// Last
	links = append(links, fmt.Sprintf("<%s?page=%d&per_page=%d>; rel=\"last\"", baseURL, totalPages, perPage))

	w.Header().Set("Link", strings.Join(links, ", "))
}

// etagUnresolved is the validator for an entity whose edges could not be
// resolved. See the sentinel rationale on [entityETagWithEdges].
func etagUnresolved(ctx context.Context, aff affordanceService, e *entityPkg.Entity) string {
	slog.Warn("dataentry: ETag: world-scoped edge read failed; "+
		"folding a sentinel so the validator matches nothing",
		"entity", e.ID, "world", worldFromContext(ctx).name)
	return servedETag(ctx, aff, e, []*entityPkg.Relation{
		{Type: "!unresolved", To: "!unresolved"},
	})
}

// servedETag is the validator for e as the caller sees it: e's properties
// minus those the caller's `visible:` grants hide, together with the edges
// the response carried. Every entity ETag goes through it, so a GET, an
// If-Match check and a PATCH response agree for one caller.
//
// Hashing a hidden value would make the ETag an offline guessing oracle for
// it: a caller could hash candidate values and compare. It would also change
// the ETag when only a hidden value changed, so the caller's next If-Match
// would fail for a change it cannot see.
//
// It runs the markdown formatter over the body (through the canonical view
// hash), which is fine for one entity per request. Do not use it on a list
// path.
func servedETag(
	ctx context.Context, aff affordanceService, e *entityPkg.Entity, edges []*entityPkg.Relation,
) string {
	return entityETagWithEdges(e, aff.hiddenProperties(ctx, e), edges)
}

// entityETagWithEdges hashes e together with the edges the response actually
// carried, leaving out the properties named in hidden. Callers use
// [servedETag], which supplies hidden from the caller's grants.
//
// Taking the edges as a PARAMETER rather than reading them is what keeps the
// validator and the body describing the same document: the GET computes them
// once, for both.
//
// A package function, not a method: it needs nothing from App, and App is
// pinned at its plimsoll cap (see the App type doc).
func entityETagWithEdges(
	e *entityPkg.Entity, hidden map[string]struct{}, edges []*entityPkg.Relation,
) string {
	// The view hash covers the served FACE, not the world NAME. Two worlds
	// that resolve the same id to different faces get different validators,
	// so a published-world GET cannot answer 304 against a draft-derived one
	// (the cross-world cache poisoning this fold exists for). Two worlds
	// serving the SAME face share one, which is correct: the bytes are the
	// same. And a write, which addresses the stored face directly and refuses
	// `?world=`, computes the same fold — so an If-Match taken from a GET that
	// showed the bare face matches the PATCH of it, while one taken from a GET
	// that showed `published` refuses a write aimed at the draft. Folding the
	// world name instead made every world-bound If-Match a permanent 412.
	return viewETag(canonical.SumEntity(entityView(e, hidden)), e.Content, edges)
}

// viewETag is the validator for a row whose view hash is view (see
// [canonical.Digest]), served with the raw body rawBody and the edges edges.
// The entity and the single-relation reads both use it.
//
// The raw body is folded because the view hash formats the body: a backend
// that stores the raw body would otherwise keep the validator across a
// formatting-only edit, answering 304 with stale bytes or passing an If-Match
// against a different base text.
func viewETag(view canonical.Digest, rawBody string, edges []*entityPkg.Relation) string {
	h := sha256.New()
	_, _ = h.Write(view[:])
	etagField(h, rawBody)

	// Fold outgoing relations into the hash so PATCHes that only change
	// edges also change the ETag — otherwise If-Match / If-None-Match
	// round-trips poison client caches.
	//
	// The edges are the caller's, and must be the ones the response BODY
	// carried. Until TKT-WRLDAPI item 4 a world-bound response carried no
	// relations at all, so nothing was folded under a world; now it carries
	// world-resolved ones, and folding nothing would mean an edge change under
	// a world never moved the ETag — a stale 304 on a response whose links had
	// changed. Folding DEFAULT-world edges instead is the mirror bug: a draft
	// edit would invalidate a published validator, and publishing a face would
	// not.
	edgeKeys := make([][2]string, 0, len(edges))
	for _, edge := range edges {
		edgeKeys = append(edgeKeys, [2]string{edge.Type, edge.To})
	}
	slices.SortFunc(edgeKeys, func(a, b [2]string) int {
		if c := strings.Compare(a[0], b[0]); c != 0 {
			return c
		}
		return strings.Compare(a[1], b[1])
	})
	for _, k := range edgeKeys {
		etagField(h, k[0])
		etagField(h, k[1])
	}

	sum := h.Sum(nil)
	return fmt.Sprintf("\"%s\"", base64.StdEncoding.EncodeToString(sum[:8]))
}

// etagField writes s length-prefixed, so no value can forge another field
// boundary.
func etagField(h hash.Hash, s string) {
	var n [8]byte
	binary.BigEndian.PutUint64(n[:], uint64(len(s)))
	_, _ = h.Write(n[:])
	_, _ = h.Write([]byte(s))
}

// entityView returns e as a caller sees it: e without the properties named in
// hidden or in e.Redacted. It never mutates e.
//
// e.Redacted is the redacting reader's own record of what it withheld. It
// should agree with hidden; removing both keeps a hidden value out of the
// hash if the two ever diverge.
func entityView(e *entityPkg.Entity, hidden map[string]struct{}) entityPkg.Entity {
	view := *e
	if len(hidden) == 0 && len(e.Redacted) == 0 {
		return view
	}
	view.Properties = make(map[string]any, len(e.Properties))
	for k, v := range e.Properties {
		if _, h := hidden[k]; h || e.IsRedacted(k) {
			continue
		}
		view.Properties[k] = v
	}
	return view
}

// validateCreateIDOpts enforces that `id` is only accepted for manual-ID types
// and that `prefix` is only accepted for non-manual types with a known prefix.
// For manual-ID types that declare one or more prefixes, the `id` must start
// with one of them AND include a non-empty suffix. Surrounding whitespace on
// the inputs is trimmed at the boundary so the error message lines up with
// what the user actually typed. Returns an empty string when valid.
func validateCreateIDOpts(def *metamodel.EntityDef, id, prefix string) string {
	id = strings.TrimSpace(id)
	prefix = strings.TrimSpace(prefix)

	if id != "" && !def.IsManualID() {
		return "id not accepted for non-manual ID type; use 'prefix' instead"
	}
	if id != "" && def.IsManualID() {
		if prefixes := def.GetIDPrefixes(); len(prefixes) > 0 {
			matched := false
			for _, p := range prefixes {
				// Reject the bare prefix (id == p) — the entity needs a
				// distinguishing suffix or it conflicts with the prefix
				// concept itself. (RR-…) The prefix compares with case
				// folded, like metamodel.EntityDef.MatchesID (BUG-4OPP525).
				if len(id) > len(p) && strings.EqualFold(id[:len(p)], p) {
					matched = true
					break
				}
			}
			if !matched {
				return fmt.Sprintf("id %q must start with one of %v and include a suffix", id, prefixes)
			}
		}
	}
	if prefix == "" {
		return ""
	}
	if def.IsManualID() {
		return "prefix not applicable to manual ID type"
	}
	if slices.Contains(def.GetIDPrefixes(), prefix) {
		return ""
	}
	return fmt.Sprintf("prefix %q is not valid; allowed: %v", prefix, def.GetIDPrefixes())
}

func writeV1JSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(data)
}

func writeV1Error(w http.ResponseWriter, r *http.Request, status int, errType, title, detail string) {
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(status)

	err := v1.Error{
		Type:     "https://rela.dev/errors/" + errType,
		Title:    title,
		Status:   status,
		Detail:   detail,
		Instance: r.URL.Path,
	}

	_ = json.NewEncoder(w).Encode(err)
}

// --- Side Panel API ---

// --- Sidebar API ---

// resolveConflictPath contains the caller-supplied conflict-file path
// to the project root. On failure it writes the error response (404
// when the path is inside the project but missing, 403 when it escapes
// the root) and returns ok=false.
func resolveConflictPath(paths *project.Context, w http.ResponseWriter, r *http.Request, p string) (string, bool) {
	resolved, err := containedProjectPath(paths.Root, p)
	switch {
	case errors.Is(err, errPathNotFound):
		writeV1Error(w, r, http.StatusNotFound, "conflict_not_found", "Conflicted file not found", "")
		return "", false
	case err != nil:
		writeV1Error(w, r, http.StatusForbidden, "path_outside_project", "Path is outside the project root", "")
		return "", false
	}
	return resolved, true
}

// conflictAuditSubject derives the audit subject for a resolved
// conflict write. Exactly one of e / rel is non-nil ([conflict.Resolve]
// errors otherwise).
func conflictAuditSubject(e *entityPkg.Entity, rel *entityPkg.Relation) *audit.Subject {
	if rel != nil {
		return &audit.Subject{
			Kind:         "relation",
			RelationType: rel.Type,
			FromID:       rel.From,
			ToID:         rel.To,
		}
	}
	return &audit.Subject{Kind: "entity", Type: e.Type, ID: e.ID}
}

// --- Documents API ---

// handleV1Documents handles GET /api/v1/_documents/{docName}/{entityId} and
// GET /api/v1/_documents/{docName}. Returns JSON with rendered HTML content
// for Vue SPA consumption.
//
// The two path shapes serve the two document kinds
// (dataentryconfig.DocumentConfig): the two-segment form renders an
// entity-anchored document, the one-segment form a standalone one. Each shape
// REJECTS the other kind rather than inventing or ignoring an entry id — see
// handleV1StandaloneDocument.
//
//archplan:auth=authz
func (a *App) handleV1Documents(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeV1Error(w, r, http.StatusMethodNotAllowed, "method_not_allowed", "Method not allowed", "")
		return
	}

	// Parse path: /api/v1/_documents/{docName}[/{entityId}][/_export], then
	// dispatch on the shape. The branches live in separate functions so none
	// grows another's guards by accident — they differ in their ACL story, not
	// just in which segments are present.
	//
	// The trailing `_export` segment is matched EXACTLY and only in final
	// position, so the four shapes are disjoint:
	//
	//	{doc}                     render, standalone
	//	{doc}/_export             export, standalone
	//	{doc}/{entityId}          render, anchored
	//	{doc}/{entityId}/_export  export, anchored
	//
	// The two-segment forms cannot collide: an entity id can never be
	// "_export" because entity.ValidateID rejects a leading underscore. The
	// other side of that premise — a document literally named "_export" — is
	// closed at config load by validateDocuments. See exportSegment.
	path := strings.TrimPrefix(r.URL.Path, "/api/v1/_documents/")
	parts := strings.Split(path, "/")

	// Trailing empty segment from a trailing slash ("/_documents/foo/") is not
	// a missing entity id; drop it so the shape matches the one-segment form.
	if len(parts) > 1 && parts[len(parts)-1] == "" {
		parts = parts[:len(parts)-1]
	}

	// Any REMAINING empty segment is rejected outright. This is not defensive
	// tidying — without it "/_documents/sales//_export" splits to
	// ["sales", "", "_export"], matches the 3-segment export case, and calls
	// the export handler with an empty entity id, which dispatches to the
	// STANDALONE resolver. That serves a standalone document at the anchored
	// URL shape, precisely what this package's CLAUDE.md forbids ("Never let
	// one URL shape serve the other kind... do not fall back to rendering with
	// an empty or guessed entry id").
	//
	// net/http's ServeMux happens to redirect "//" before a request reaches
	// here, but that is a property of the mux, not of this function: tests and
	// any future direct mount call it without that normalization, and a
	// percent-encoded %2F survives ServeMux cleaning entirely. Reject here so
	// the guarantee belongs to the router itself.
	if slices.Contains(parts, "") {
		writeV1Error(w, r, http.StatusBadRequest, "invalid_path",
			"Path must be /_documents/{docName}/{entityId}", "")
		return
	}

	switch {
	case len(parts) == 1:
		handleV1StandaloneDocument(a, w, r, parts[0])
	case len(parts) == 2 && parts[1] == exportSegment:
		handleV1ExportDocument(a, w, r, parts[0], "")
	case len(parts) == 2:
		handleV1AnchoredDocument(a, w, r, parts[0], parts[1])
	case len(parts) == 3 && parts[2] == exportSegment:
		handleV1ExportDocument(a, w, r, parts[0], parts[1])
	default:
		writeV1Error(w, r, http.StatusNotFound, "not_found", "Resource not found", "")
	}
}

// handleV1AnchoredDocument serves GET /api/v1/_documents/{docName}/{entityId}
// — a document declared WITH an `entity_type:`, rendered about one entity.
//
// The gate chain lives in resolveAnchoredDocument, shared with the export
// route so the two cannot drift; its ordering is load-bearing and documented
// there. What stays HERE is everything specific to producing HTML: the
// return_to rewrite, the refresh flag, and the disk cache — see the note in
// resolveAnchoredDocument about why the cache must not move into it.
//
// A plain function taking *App for the same reason handleV1StandaloneDocument
// is one: App sits on its plimsoll load line.
func handleV1AnchoredDocument(a *App, w http.ResponseWriter, r *http.Request, docName, entityID string) {
	resolved, ok := resolveAnchoredDocument(a, w, r, docName, entityID)
	if !ok {
		return // the resolver already wrote the response
	}
	renderCfg := resolved.cfg

	// Check for refresh param - skip cache if present
	forceRefresh := r.URL.Query().Get("refresh") == "true"
	// A read at an instant never touches the disk cache: the cache holds
	// renders of today's state (TKT-6KLUT3U).
	if _, past := asOfFromContext(r.Context()); past {
		forceRefresh = true
	}

	// return_to is the URL the caller is currently on. The rewriter uses
	// it to inject a `return_to` query param into form links so the form
	// redirects back here after submit. isSafeReturnPath enforces the
	// open-redirect guard — it rejects protocol-relative (//evil.com),
	// backslash-tricks (/\evil.com), and any absolute/schemed URLs, and
	// returns the normalised same-origin path on success.
	returnPath := isSafeReturnPath(r.URL.Query().Get("return_to"))

	// Try to get cached content (unless refresh requested). Disk cache
	// is only populated for command: renders (see doRender); skip the
	// read for script: docs so we don't serve a stale command:-era file
	// after a doc is switched to a Lua script.
	//
	// That condition is also what keeps GetCached safe: its key carries
	// neither the ConfigID nor the principal, so it may only ever serve a
	// principal-independent command: render. Never widen this to script:
	// docs, and never call GetCached from a per-principal path (RR-2QSGLU).
	if !forceRefresh && renderCfg.Script == "" {
		result := a.documents.GetCached(r.Context(), resolved.entryID)
		if result != nil {
			html := RewriteDocumentLinks(result.HTML, returnPath, nil)
			writeV1JSON(w, http.StatusOK, v1.DocumentResponse{
				HTML:      html,
				Cached:    true,
				EntityIDs: extractEntityIDs(result.Entities),
			})
			return
		}
	}

	// Render the document
	result, err := a.documents.Render(r.Context(), resolved.entryID, renderCfg)
	if err != nil {
		var se *lua.ScriptError
		if errors.As(err, &se) {
			correlationID := newCorrelationID()
			slog.Warn("document render failed",
				"document", docName, "entity", entityID,
				"correlation", correlationID, "error", err)
			writeV1ScriptError(w, se, a.allowFullScriptDetail(r), correlationID)
			return
		}
		writeInternalError(w, r, "render_failed", "Document rendering failed", err)
		return
	}

	html := RewriteDocumentLinks(result.HTML, returnPath, nil)
	writeV1JSON(w, http.StatusOK, v1.DocumentResponse{
		HTML:      html,
		Cached:    false,
		EntityIDs: extractEntityIDs(result.Entities),
	})
}

// extractEntityIDs extracts IDs from a slice of entities.
func extractEntityIDs(entities []*entityPkg.Entity) []string {
	ids := make([]string, len(entities))
	for i, e := range entities {
		ids[i] = e.ID
	}
	return ids
}

// --- Views API ---

// sectionEntityToV1 lifts a section's row entity (template-side data)
// onto the wire shape. Centralizes the `v1.ViewEntity` construction so
// the typed `_props` and per-row `_fields` (TKT-IHC7D) stay consistent
// across both the top-level entities path and the (currently dormant)
// grouped-card entities path.
func sectionEntityToV1(e SectionEntityData) v1.ViewEntity {
	v1Ent := v1.ViewEntity{
		ID:         e.ID,
		Title:      e.Title,
		Type:       e.Type,
		EditFormID: e.EditFormID,
		Content:    e.Content,
		HasContent: e.HasContent,
		Props:      e.Props,
	}
	for _, f := range e.Fields {
		v1Ent.Fields = append(v1Ent.Fields, v1.SectionField(f))
	}
	if e.FieldVerdicts != nil {
		fa := e.FieldVerdicts
		v1Ent.FieldAffordances = &fa
	}
	v1Ent.World = e.World
	v1Ent.Self = e.Self
	return v1Ent
}

// pluralOf maps an entity type to its route segment in meta, or to "" for a
// type meta does not declare.
func pluralOf(meta *metamodel.Metamodel) func(entityType string) string {
	return func(entityType string) string {
		def, ok := meta.GetEntityDef(entityType)
		if !ok {
			return ""
		}
		return def.GetPlural(entityType)
	}
}

// entityTypeForPlural returns the entity type whose plural is plural, or ""
// when none matches.
func entityTypeForPlural(meta *metamodel.Metamodel, plural string) string {
	for name, def := range meta.Entities {
		if def.GetPlural(name) == plural {
			return name
		}
	}
	return ""
}

// worldSwitcherWire resolves an unset app.world_switcher to its default, so
// the SPA carries no default of its own.
func worldSwitcherWire(v string) string {
	if v == "" {
		return dataentryconfig.WorldSwitcherHeader
	}
	return v
}
