package dataentry

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/Sourcehaven-BV/rela/internal/dataentryconfig"
	entityPkg "github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/transform"
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

	columns := h.exportListColumns(query.Get("list"), typeName)

	// The WHOLE ACL-scoped, filtered, sorted set — pre-pagination. Reuses the
	// exact read path the list view uses, so export can't widen past the view.
	entities, err := h.scopedEntities(ctx, typeName, query)
	if err != nil {
		writeListPipelineError(w, r, err)
		return
	}
	total := len(entities)
	truncated := false
	if total > listExportCap {
		entities = entities[:listExportCap]
		truncated = true
	}

	renderer := h.listTableRenderer(entities, columns, total, truncated)
	h.convertAndWrite(w, r, reg, name, renderer, typeName+"-list", "type", typeName)
}

// exportListColumns returns the columns for the export. It prefers the named
// list's columns, falls back to the type's default list, and finally to a
// minimal [id, title] pair so an export always produces a sensible table even
// when no list is configured.
func (h *exportHandler) exportListColumns(listID, typeName string) []dataentryconfig.ListColumn {
	cfg := h.cfg()
	if listID != "" {
		if l, ok := cfg.Lists[listID]; ok && l.EntityType == typeName && len(l.Columns) > 0 {
			return l.Columns
		}
	}
	if def := h.findListForType(typeName); def != "" {
		if l, ok := cfg.Lists[def]; ok && len(l.Columns) > 0 {
			return l.Columns
		}
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
			t = transform.DisplayTitle(meta, node)
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
