package dataentry

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/Sourcehaven-BV/rela/internal/dataentryconfig"
	entityPkg "github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/lua"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/store"
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
//
//archplan:auth=required
func (h *exportHandler) handleV1ExportList(w http.ResponseWriter, r *http.Request, typeName string) {
	if r.Method != http.MethodGet {
		writeV1Error(w, r, http.StatusMethodNotAllowed, "method_not_allowed", "Method not allowed", "")
		return
	}
	name, reg, ok := h.resolveTransform(w, r)
	if !ok {
		return
	}
	renderer, err := h.listExportRenderer(r.Context(), typeName, r.URL.Query())
	if err != nil {
		writeListPipelineError(w, r, err)
		return
	}
	h.convertAndWrite(w, r, reg, name, renderer, typeName+"-list", "type", typeName)
}

// listExportRenderer is the list export pipeline without the response: the
// WHOLE ACL-scoped, filtered and conditioned set of typeName for query,
// field-redacted, capped, and wrapped in the list's renderer. The export
// route and rela.attach_render share it. An error is one
// [writeListPipelineError] answers.
// Rows are field-redacted before any cell renders, and the override is chosen
// only after the read, redaction and cap.
func (h *exportHandler) listExportRenderer(
	ctx context.Context, typeName string, query map[string][]string,
) (transform.Renderer, error) {
	q := url.Values(query)
	effListID, effList, haveList := h.resolveEffectiveList(q.Get("list"), typeName)
	columns := withSchemaLabels(h.meta(), []string{typeName}, listColumnsOf(effList, haveList))

	cond, err := h.listCondition(effListID)
	if err != nil {
		return nil, err
	}
	entities, err := h.scopedEntities(ctx, typeName, query)
	if err != nil {
		return nil, err
	}
	if cond != nil {
		if ctx, err = h.bindConditionIdentity(ctx, typeName, query); err != nil {
			return nil, err
		}
		ctx = visibility.PrimeTraversals(ctx, h.redactor, entities)
		if entities, err = applyViewCondition(ctx, entities, cond, h.redactForCondition, h.store); err != nil {
			return nil, err
		}
	}
	entities = h.visReader.Filter(ctx, entities)
	total := len(entities)
	truncated := false
	if total > listExportCap {
		entities = entities[:listExportCap]
		truncated = true
	}

	renderer := h.listTableRenderer(entities, columns, total, truncated)
	if effList.ExportRender != "" {
		if err := h.loadBodies(ctx, entities); err != nil {
			return nil, fmt.Errorf("%w: %w", errListLoad, err)
		}
		renderer = h.listOverrideRenderer(listOverride{
			listID: effListID, script: effList.ExportRender, typeName: typeName,
			rows: entities, total: total, query: query,
		})
	}
	return renderer, nil
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

// listOverrideRenderer builds the renderer for a list export whose list sets an
// `export_render:` Lua script. The caller has already ACL-scoped, field-redacted
// and capped the rows. The script is config, never request input. The script
// sees whole entities, including the body, which `visibility.Redact` leaves
// verbatim; route it through body redaction when that lands. An unreadable list
// renders an empty document, never a 404, so "no rows" and "no access" look alike.
func (h *exportHandler) listOverrideRenderer(in listOverride) transform.Renderer {
	lrc := lua.ListRenderContext{
		ListID: in.listID,
		Rows:   entitySliceRows(in.rows),
		Query:  buildListQuery(in),
	}
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

	q.Sort = parseSortParam(in.query)
	return q
}

// resolveEffectiveList returns the list configuration governing this export:
// the named ?list= when it exists and matches the type, else the type's
// default list from navigation, else ok=false. Both the column set and the
// render override must resolve through this so they come from the same list.
// It does not require columns: a list with only `export_render:` is valid.
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
		cols := make([]dataentryconfig.ListColumn, 0, len(l.Columns))
		for _, c := range l.Columns {
			if c.Busy == "" {
				cols = append(cols, c)
			}
		}
		return cols
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
		rels, err := h.resolveListRelations(ctx, meta, entities, columns)
		if err != nil {
			return nil, err
		}

		var b strings.Builder

		b.WriteString("|")
		for _, c := range columns {
			fmt.Fprintf(&b, " %s |", transform.EscapeInline(columnLabel(c)))
		}
		b.WriteString("\n|")
		for range columns {
			b.WriteString(" --- |")
		}
		b.WriteString("\n")

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
// whole set of exported rows in one pass: it loads every row's edges in one
// query, gathers ALL neighbor IDs across every row, runs a SINGLE batched
// visibility gate, loads each visible neighbor once, then assembles
// per-row/per-column visible titles.
func (h *exportHandler) resolveListRelations(
	ctx context.Context, meta *metamodel.Metamodel, entities []*entityPkg.Entity, columns []dataentryconfig.ListColumn,
) (listRelationTitles, error) {
	out := listRelationTitles{}

	relCols := make([]dataentryconfig.ListColumn, 0, len(columns))
	for _, c := range columns {
		if c.Relation != "" {
			relCols = append(relCols, c)
		}
	}
	if len(relCols) == 0 {
		return out, nil
	}

	perCell, allPeerIDs, err := h.gatherListPeers(ctx, entities, relCols)
	if err != nil {
		return nil, err
	}
	if len(perCell) == 0 {
		return out, nil
	}

	visible := visibleRelationIDs(ctx, h.visibleReader, allPeerIDs)
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
	return out, nil
}

// gatherListPeers returns, per (row, relation column), the ordered peer IDs,
// plus the flat list of every peer ID (for the single batched visibility
// gate).
//
// The edges of every row come from ONE store query, not one per row
// (TKT-VEIMO3), and only in the directions some column uses. A cell lists
// its neighbors in the store's relation order: every backend sorts relations
// by key whatever the filter, so batching does not reorder a cell.
//
// A store error fails the export rather than rendering a table that looks
// complete but lacks edges (the RR-4TFZNL reasoning of the entity export).
func (h *exportHandler) gatherListPeers(
	ctx context.Context, entities []*entityPkg.Entity, relCols []dataentryconfig.ListColumn,
) (perCell []cellPeers, allPeerIDs []string, err error) {
	needIn, needOut := false, false
	for _, c := range relCols {
		if c.Direction == dataentryconfig.DirectionIncoming {
			needIn = true
		} else {
			needOut = true
		}
	}
	dir := store.DirectionBoth
	switch {
	case !needIn:
		dir = store.DirectionOutgoing
	case !needOut:
		dir = store.DirectionIncoming
	}
	outByRow, inByRow, err := h.reader.pageRelationsDir(ctx, entities, dir)
	if err != nil {
		return nil, nil, fmt.Errorf("list export: reading relation columns: %w", err)
	}
	meta := h.meta()
	for i, e := range entities {
		var outgoing, incoming []*entityPkg.Relation
		if needOut {
			outgoing = edgesOwnedBy(meta, outByRow[i], e.Face)
		}
		if needIn {
			incoming = incomingOwnedAtZero(meta, inByRow[i], e)
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
	return perCell, allPeerIDs, nil
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
// unloadable neighbors. Every visible neighbor's row is loaded in one query up
// front.
func (h *exportHandler) memoNeighborTitle(
	ctx context.Context, meta *metamodel.Metamodel, visible map[string]bool,
) func(id string) (string, bool) {
	ids := make([]string, 0, len(visible))
	for id, ok := range visible {
		if ok {
			ids = append(ids, id)
		}
	}
	rows := h.reader.defaultWorldHeaders(ctx, ids)
	titleByID := map[string]string{}
	return func(id string) (string, bool) {
		if !visible[id] {
			return "", false
		}
		if t, done := titleByID[id]; done {
			return t, t != ""
		}
		t := ""
		if node, ok := rows[id]; ok {
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
	if c.Face {
		return metamodel.FaceLabel(meta, e.Type, string(e.Face))
	}
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
// relation name when neither the list nor the schema sets one (the caller
// fills schema labels with withSchemaLabels).
func columnLabel(c dataentryconfig.ListColumn) string {
	if c.Label != "" {
		return c.Label
	}
	if c.Face {
		return "Face"
	}
	if c.Relation != "" {
		return c.Relation
	}
	return c.Property
}

// listCondition resolves the effective list's compiled `condition:`, or nil
// when the seam is unwired or the list declares none. An error means the list
// declares a condition that has no compiled matcher, so the export fails
// rather than exporting rows the condition would exclude.
func (h *exportHandler) listCondition(listID string) (ViewConditionMatcher, error) {
	if h.viewCondition == nil {
		return nil, nil //nolint:nilnil // nil is the documented "no condition"
	}
	return h.viewCondition(listID)
}

// bindConditionIdentity stamps the query identity a `current_user` condition
// evaluates against, once for the whole export.
//
// Nil seam is not an error: only a condition naming current_user needs the
// stamp, and such a condition then fails loudly rather than matching
// everything — the safe direction for a narrowing.
func (h *exportHandler) bindConditionIdentity(
	ctx context.Context, typeName string, query map[string][]string,
) (context.Context, error) {
	if h.stampQueryIdentity == nil {
		return ctx, nil
	}
	return h.stampQueryIdentity(ctx, typeName, query)
}
