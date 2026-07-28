package dataentry

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/Sourcehaven-BV/rela/internal/dataentryconfig"
	entityPkg "github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/lua"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/transform"
	"github.com/Sourcehaven-BV/rela/internal/visibility"
)

// listExportCap bounds how many rows a single list export renders. Past this the
// table is truncated and a visible notice is appended (RR-6ZDPTQ) so the export
// never silently drops rows and a huge graph can't OOM the converter. A
// per-view configurable cap is v2. A var (not const) so tests can lower it
// without seeding thousands of rows.
var listExportCap = 5000

// handleV1ExportList serves GET /api/v1/{plural}/_export?transform=<name>[&list=<id>].
//
// It exports the WHOLE ACL-scoped, filtered set for the type (not just a page)
// as a markdown table of the list view's columns, converted by the named
// transform. Relation-column cells show VISIBLE neighbor titles only — the same
// visibility gate the on-screen list applies — so a hidden neighbor never leaks
// into the export (RR-T3PDHN). The set is capped at [listExportCap]; past the
// cap the table is truncated with a visible notice.
func (h *exportHandler) handleV1ExportList(w http.ResponseWriter, r *http.Request, typeName string) {
	if r.Method != http.MethodGet {
		writeV1Error(w, r, http.StatusMethodNotAllowed, "method_not_allowed", "Method not allowed", "")
		return
	}
	ctx := r.Context()
	query := r.URL.Query()

	name, reg, ok := h.resolveTransform(w, r)
	if !ok {
		return
	}

	// Resolve the effective list ONCE and derive both the columns and the
	// render override from it — see resolveEffectiveList for why these must
	// not be looked up independently.
	effListID, effList, haveList := h.resolveEffectiveList(query.Get("list"), typeName)
	columns := listColumnsOf(effList, haveList)

	// The WHOLE ACL-scoped, filtered, sorted set — pre-pagination. Reuses the
	// exact read path the list view uses, so export can't widen past the view.
	entities, err := h.scopedEntities(ctx, typeName, query)
	if err != nil {
		writeListPipelineError(w, r, err)
		return
	}
	// Field-redact every row through the visibility seam before any cell is
	// rendered (the #1188 IB-review finding): a hidden property renders as
	// an empty cell and a hidden display property falls back to the ID.
	// The row-gate inside Filter is idempotent over the already-scoped set
	// (one extra batched probe for the type — structural consistency over
	// micro-optimization).
	entities = h.visReader.Filter(ctx, entities)
	total := len(entities)
	truncated := false
	if total > listExportCap {
		entities = entities[:listExportCap]
		truncated = true
	}

	// Override selection happens HERE — after the ACL-scoped read, the
	// field-redaction pass, and the cap. That ordering is load-bearing: a
	// render override can only ever be handed rows that already survived
	// every gate, and never more of them than the cap allows.
	//
	// haveList needs no test of its own: resolveEffectiveList returns a zero
	// List when it finds none, and a zero List has no ExportRender.
	renderer := h.listTableRenderer(entities, columns, total, truncated)
	if effList.ExportRender != "" {
		renderer = h.listOverrideRenderer(listOverride{
			listID: effListID, script: effList.ExportRender, typeName: typeName,
			rows: entities, total: total, query: query,
		})
	}
	h.convertAndWrite(w, r, reg, name, renderer, typeName+"-list", "type", typeName)
}

// listOverride is what rendering a list THROUGH A SCRIPT needs: which list,
// which script, and the resolved read it renders. Deliberately not a bundle of
// everything the handler happens to hold — the built-in table path takes its
// own arguments, so neither branch carries fields the other needs.
type listOverride struct {
	listID   string
	script   string
	typeName string
	rows     []*entityPkg.Entity
	total    int // pre-cap count; len(rows) is what the script can reach
	query    map[string][]string
}

// listOverrideRenderer builds the markdown renderer for a list export that
// configures an `export_render:` Lua script — the per-list render OVERRIDE —
// so exporting that list uses the operator's document instead of the built-in
// column table.
//
// The rows were already resolved through the ACL read path, field-redacted,
// and capped by the caller, so the override sees exactly the ROWS the
// on-screen view resolved, with the same property redaction. The script is a
// fixed config value, never request input: a request may choose a transform
// NAME, never a renderer.
//
// It does NOT see only what the on-screen table showed. A row reaches the
// script as a full entity table, so a script can render the entity BODY, which
// the column table never displays. That is intended — a report override wants
// bodies, and the entity export path already exposes them — but note that
// `visibility.Redact` currently leaves `Content` verbatim (the body-redaction
// TODO in internal/visibility/policyreader.go). When body redaction lands, this
// is one of the paths that must route through it.
//
// Unlike the entity path there is no failure mode to report here — a list the
// caller may not read resolves to an empty set and renders an empty document,
// which is what the built-in table does too. A 404 would leak the difference
// between "no rows" and "no access".
func (h *exportHandler) listOverrideRenderer(in listOverride) transform.Renderer {
	lrc := lua.ListRenderContext{
		ListID: in.listID,
		Rows:   entitySliceRows(in.rows),
		Query:  buildListQuery(in),
	}
	// ConfigID is synthetic and namespaced: the "list:" infix keeps it from
	// colliding with the entity path's "export:<type>" for a list whose id
	// happens to equal an entity type name.
	//
	// No isSafePathSegment guard here, unlike the entity path (see
	// exportRenderer): that one validates because an entity id is REQUEST
	// input that reaches a document cache filename. This id is neither — it
	// is a key that matched cfg.Lists, so it is operator-authored config, and
	// RenderListMarkdown neither caches nor shells out, so the ConfigID's only
	// consumers are a Lua string and an error message. If a list render ever
	// gains a disk cache, this needs the guard.
	cfg := documentRenderConfig{ConfigID: "export:list:" + in.listID, Script: in.script}

	return transform.RendererFunc(func(ctx context.Context) ([]byte, error) {
		md, err := h.documents.RenderListMarkdown(ctx, cfg, lrc)
		if err != nil {
			return nil, err
		}
		return []byte(md), nil
	})
}

// entitySliceRows adapts an already-resolved entity slice to lua.ListRows.
// The laziness that matters is on the Lua side — one entity TABLE is
// materialized at a time — so a plain indexed slice is exactly the right
// backing store here.
type entitySliceRows []*entityPkg.Entity

func (r entitySliceRows) Len() int { return len(r) }

func (r entitySliceRows) At(i int) *entityPkg.Entity {
	if i < 0 || i >= len(r) {
		return nil
	}
	return r[i]
}

// buildListQuery flattens the request into the read-only context a render
// override sees: which list, over which type, under which filters and sort,
// and how the cap applied. Only what the caller already supplied is echoed
// back — this is context for titling and annotating an export, not a channel
// for changing what it contains.
func buildListQuery(in listOverride) lua.ListQuery {
	q := lua.ListQuery{
		EntityType: in.typeName,
		Q:          queryGet(in.query, "q"),
		Total:      in.total,
	}

	// filter[<key>]=<value> → {key: value}, parsed with the SAME helper the
	// filter pipeline uses. Rolling our own TrimPrefix/TrimSuffix here would
	// reintroduce RR-6RF60V: `filter[status][ne]` would key the table on
	// "status][ne" instead of "status", so a script would see a filter name
	// that never matches what was actually filtered on. The operator segment
	// is intentionally dropped — this context reports WHICH properties were
	// filtered, and the last value wins, matching applyV1Filters.
	for k, vals := range in.query {
		if !strings.HasPrefix(k, "filter[") || len(vals) == 0 {
			continue
		}
		key, _, ok := parseRelationFilterKey(k)
		if !ok {
			continue
		}
		if q.Filters == nil {
			q.Filters = map[string]string{}
		}
		q.Filters[key] = vals[len(vals)-1]
	}

	// sort=-created,title, parsed by the same helper applyV1Sorting uses, so
	// the sort reported to a script cannot drift from the sort applied.
	// ListSortSpec aliases filter.SortSpec, so these pass straight through.
	q.Sort = parseSortParam(in.query)
	return q
}

// resolveEffectiveList returns the list configuration governing this export:
// the named ?list= when it exists and matches the type, else the type's
// default list from navigation, else ok=false.
//
// BOTH the column set and the render override MUST resolve through this. They
// used to be one lookup because columns were the only thing a list
// contributed; now that a list can also carry an `export_render:` script,
// resolving them independently would let an export take its columns from one
// list and its override from another — or, worse, silently skip an override
// that is plainly configured.
//
// Note what this does NOT test: `len(Columns) > 0`. That predicate lives in
// listColumnsOf, where it belongs — it is a statement about whether a list
// can supply columns, not about which list this export is. A list configured
// with `export_render:` and no `columns:` is perfectly valid (the script
// renders whatever it likes), and folding the column check in here would make
// its override silently not apply.
func (h *exportHandler) resolveEffectiveList(
	listID, typeName string,
) (string, dataentryconfig.List, bool) {
	cfg := h.cfg()
	if listID != "" {
		if l, ok := cfg.Lists[listID]; ok && l.EntityType == typeName {
			return listID, l, true
		}
	}
	if def := h.findListForType(typeName); def != "" {
		// The EntityType re-check is belt-and-braces: findListForType only
		// returns lists of this type, but it makes the contract total, so a
		// future navigation change cannot hand back a mismatched list.
		if l, ok := cfg.Lists[def]; ok && l.EntityType == typeName {
			return def, l, true
		}
	}
	return "", dataentryconfig.List{}, false
}

// listColumnsOf returns the columns for the export: the effective list's
// columns when it has any, else a minimal [id, title] pair so an export always
// produces a sensible table even when no list is configured.
//
// Takes the ALREADY-resolved list rather than resolving one itself, so a
// single request resolves exactly once and the columns can never come from a
// different list than the render override — see resolveEffectiveList.
func listColumnsOf(l dataentryconfig.List, haveList bool) []dataentryconfig.ListColumn {
	if haveList && len(l.Columns) > 0 {
		return l.Columns
	}
	return []dataentryconfig.ListColumn{
		{Property: "id", Label: "ID"},
		{Property: "title", Label: "Title"},
	}
}

// listTableRenderer builds a [transform.Renderer] that emits a markdown table of
// the given rows over the given columns, appending a truncation notice when the
// full set exceeded the cap. Neighbor titles for relation columns are gated
// through the visibility gate (hidden neighbors excluded).
func (h *exportHandler) listTableRenderer(
	entities []*entityPkg.Entity, columns []dataentryconfig.ListColumn, total int, truncated bool,
) transform.Renderer {
	return transform.RendererFunc(func(ctx context.Context) ([]byte, error) {
		meta := h.meta()
		rels := h.resolveListRelations(ctx, meta, entities, columns)

		var b strings.Builder

		// Header row.
		b.WriteString("|")
		for _, c := range columns {
			fmt.Fprintf(&b, " %s |", transform.EscapeInline(columnLabel(c)))
		}
		b.WriteString("\n|")
		for range columns {
			b.WriteString(" --- |")
		}
		b.WriteString("\n")

		// Body rows.
		for _, e := range entities {
			b.WriteString("|")
			for _, c := range columns {
				fmt.Fprintf(&b, " %s |", transform.EscapeInline(columnCell(meta, e, c, rels)))
			}
			b.WriteString("\n")
		}

		if truncated {
			fmt.Fprintf(&b, "\nShowing %d of %d rows (truncated).\n", len(entities), total)
		}
		return []byte(b.String()), nil
	})
}

// listRelationTitles holds pre-resolved relation-column cell values for a list
// export: [entityID][columnKey] -> visible neighbor titles. Resolving them once
// up front (a single batched visibility gate + one load per distinct neighbor)
// keeps list export off the per-cell store+ACL fan-out that the
// relation-visibility batching contract forbids (RR-A9U1NQ).
type listRelationTitles map[string]map[string][]string

// relColKey identifies a relation column (type + direction) so two columns on
// the same relation type but opposite directions don't collide.
func relColKey(c dataentryconfig.ListColumn) string {
	if c.Direction == dataentryconfig.DirectionIncoming {
		return c.Relation + "\x00in"
	}
	return c.Relation + "\x00out"
}

// cellPeers is the ordered peer IDs for one (row, relation-column) pair.
type cellPeers struct {
	rowID, colKey string
	peers         []string
}

// resolveListRelations resolves every relation column's neighbor titles for the
// whole set of exported rows in one pass: it loads each row's edges once, gathers
// ALL neighbor IDs across every row, runs a SINGLE batched visibility gate, loads
// each visible neighbor once, then assembles per-row/per-column visible titles.
func (h *exportHandler) resolveListRelations(
	ctx context.Context, meta *metamodel.Metamodel, entities []*entityPkg.Entity, columns []dataentryconfig.ListColumn,
) listRelationTitles {
	out := listRelationTitles{}

	relCols := make([]dataentryconfig.ListColumn, 0, len(columns))
	for _, c := range columns {
		if c.Relation != "" {
			relCols = append(relCols, c)
		}
	}
	if len(relCols) == 0 {
		return out
	}

	perCell, allPeerIDs := h.gatherListPeers(ctx, entities, relCols)
	if len(perCell) == 0 {
		return out
	}

	// ONE batched visibility gate for the whole export, and one title load per
	// distinct visible neighbor (memoized in titleFor).
	visible := visibleRelationIDs(ctx, h.reader, h.visibleReader, allPeerIDs)
	titleFor := h.memoNeighborTitle(ctx, meta, visible)

	for _, pc := range perCell {
		titles := make([]string, 0, len(pc.peers))
		for _, id := range pc.peers {
			if t, ok := titleFor(id); ok {
				titles = append(titles, t)
			}
		}
		if len(titles) == 0 {
			continue
		}
		if out[pc.rowID] == nil {
			out[pc.rowID] = map[string][]string{}
		}
		out[pc.rowID][pc.colKey] = titles
	}
	return out
}

// gatherListPeers loads each row's edges ONCE and returns, per (row, relation
// column), the ordered peer IDs, plus the flat list of every peer ID (for the
// single batched visibility gate). Only the directions some column actually
// uses are fetched — an all-outgoing column set skips the incoming lookup for
// every row.
func (h *exportHandler) gatherListPeers(
	ctx context.Context, entities []*entityPkg.Entity, relCols []dataentryconfig.ListColumn,
) (perCell []cellPeers, allPeerIDs []string) {
	needIn, needOut := false, false
	for _, c := range relCols {
		if c.Direction == dataentryconfig.DirectionIncoming {
			needIn = true
		} else {
			needOut = true
		}
	}
	for _, e := range entities {
		var outgoing, incoming []*entityPkg.Relation
		if needOut {
			outgoing = h.reader.outgoingRelations(ctx, e.ID)
		}
		if needIn {
			incoming = h.reader.incomingRelations(ctx, e.ID)
		}
		for _, c := range relCols {
			inbound := c.Direction == dataentryconfig.DirectionIncoming
			src := outgoing
			if inbound {
				src = incoming
			}
			peers := relationPeers(src, c.Relation, inbound)
			if len(peers) == 0 {
				continue
			}
			perCell = append(perCell, cellPeers{rowID: e.ID, colKey: relColKey(c), peers: peers})
			allPeerIDs = append(allPeerIDs, peers...)
		}
	}
	return perCell, allPeerIDs
}

// relationPeers extracts the peer IDs of edges of relType from rels in the given
// direction (inbound → sources, else targets).
func relationPeers(rels []*entityPkg.Relation, relType string, inbound bool) []string {
	var peers []string
	for _, rel := range rels {
		if rel.Type != relType {
			continue
		}
		if inbound {
			peers = append(peers, rel.From)
		} else {
			peers = append(peers, rel.To)
		}
	}
	return peers
}

// memoNeighborTitle returns a function that resolves a neighbor id to its display
// title, once per distinct id, returning ok=false for hidden (not in visible) or
// unloadable neighbors.
func (h *exportHandler) memoNeighborTitle(
	ctx context.Context, meta *metamodel.Metamodel, visible map[string]bool,
) func(id string) (string, bool) {
	titleByID := map[string]string{}
	return func(id string) (string, bool) {
		if !visible[id] {
			return "", false
		}
		if t, done := titleByID[id]; done {
			return t, t != ""
		}
		t := ""
		if node, ok := h.reader.getEntity(ctx, id); ok {
			// Redact BEFORE deriving the title (RR-5N4K35 class): a visible
			// neighbor with a hidden display property renders as its ID.
			t = transform.DisplayTitle(meta, visibility.Redact(ctx, h.redactor, node))
		}
		titleByID[id] = t
		return t, t != ""
	}
}

// columnCell resolves one cell value for an entity: a property value, or the
// pre-resolved comma-separated VISIBLE neighbor titles for a relation column.
func columnCell(
	meta *metamodel.Metamodel, e *entityPkg.Entity, c dataentryconfig.ListColumn, rels listRelationTitles,
) string {
	if c.Relation != "" {
		if byCol := rels[e.ID]; byCol != nil {
			return strings.Join(byCol[relColKey(c)], ", ")
		}
		return ""
	}
	switch c.Property {
	case "id":
		return e.ID
	case "title":
		return transform.DisplayTitle(meta, e)
	default:
		return transform.FormatValue(e.Properties[c.Property])
	}
}

// columnLabel returns a column's display label, defaulting to the property or
// relation name when no explicit label is set.
func columnLabel(c dataentryconfig.ListColumn) string {
	if c.Label != "" {
		return c.Label
	}
	if c.Relation != "" {
		return c.Relation
	}
	return c.Property
}
