package dataentry

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
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
func (a *App) handleV1ExportList(w http.ResponseWriter, r *http.Request, typeName string) {
	if r.Method != http.MethodGet {
		writeV1Error(w, r, http.StatusMethodNotAllowed, "method_not_allowed", "Method not allowed", "")
		return
	}
	ctx := r.Context()
	query := r.URL.Query()

	name := query.Get("transform")
	if name == "" {
		writeV1Error(w, r, http.StatusBadRequest, "missing_transform",
			"A transform is required", "pass ?transform=<name>")
		return
	}
	reg := transform.RegistryFromMetamodel(a.Meta())
	if _, ok := reg[name]; !ok {
		writeV1Error(w, r, http.StatusNotFound, "unknown_transform",
			"Unknown transform", "no such export format is configured")
		return
	}

	columns := a.exportListColumns(query.Get("list"), typeName)

	// The WHOLE ACL-scoped, filtered, sorted set — pre-pagination. Reuses the
	// exact read path the list view uses, so export can't widen past the view.
	entities, err := a.scopedSortedEntities(ctx, typeName, query)
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

	renderer := a.listTableRenderer(entities, columns, total, truncated)

	eng, err := transform.NewEngine(reg)
	if err != nil {
		slog.Warn("dataentry: build transform engine failed", "err", err)
		writeV1Error(w, r, http.StatusInternalServerError, "export_failed", "Export failed", "check server logs")
		return
	}
	res, err := eng.Run(ctx, name, renderer)
	if err != nil {
		var unknown transform.UnknownTransformError
		if errors.As(err, &unknown) {
			writeV1Error(w, r, http.StatusNotFound, "unknown_transform", "Unknown transform", "")
			return
		}
		slog.Warn("dataentry: transform list export failed", "err", err, "type", typeName, "transform", name)
		writeV1Error(w, r, http.StatusInternalServerError, "export_failed", "Export failed", "check server logs")
		return
	}

	writeExportResponse(w, res, exportFilename(typeName+"-list", res.Produces))
}

// exportListColumns returns the columns for the export. It prefers the named
// list's columns, falls back to the type's default list, and finally to a
// minimal [id, title] pair so an export always produces a sensible table even
// when no list is configured.
func (a *App) exportListColumns(listID, typeName string) []dataentryconfig.ListColumn {
	s := a.State()
	if listID != "" {
		if l, ok := s.Cfg.Lists[listID]; ok && l.EntityType == typeName && len(l.Columns) > 0 {
			return l.Columns
		}
	}
	if def := a.findListByEntityType(s, s.Cfg.Navigation, typeName); def != "" {
		if l, ok := s.Cfg.Lists[def]; ok && len(l.Columns) > 0 {
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
func (a *App) listTableRenderer(
	entities []*entityPkg.Entity, columns []dataentryconfig.ListColumn, total int, truncated bool,
) transform.Renderer {
	return transform.RendererFunc(func(ctx context.Context) ([]byte, error) {
		meta := a.Meta()

		var b strings.Builder

		// Header row.
		b.WriteString("|")
		for _, c := range columns {
			fmt.Fprintf(&b, " %s |", escapeTableCell(columnLabel(c)))
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
				fmt.Fprintf(&b, " %s |", escapeTableCell(a.columnCell(ctx, meta, e, c)))
			}
			b.WriteString("\n")
		}

		if truncated {
			fmt.Fprintf(&b, "\nShowing %d of %d rows (truncated).\n", len(entities), total)
		}
		return []byte(b.String()), nil
	})
}

// columnCell resolves one cell value for an entity: a property value, or the
// comma-separated VISIBLE neighbor titles for a relation column (honoring the
// column's direction).
func (a *App) columnCell(
	ctx context.Context, meta *metamodel.Metamodel, e *entityPkg.Entity, c dataentryconfig.ListColumn,
) string {
	if c.Relation != "" {
		return strings.Join(a.visibleNeighborTitles(ctx, meta, e, c.Relation, c.Direction == dataentryconfig.DirectionIncoming), ", ")
	}
	switch c.Property {
	case "id":
		return e.ID
	case "title":
		return displayTitleOrTitle(meta, e)
	default:
		return formatCellValue(e.Properties[c.Property])
	}
}

// visibleNeighborTitles returns the display titles of the entities related to e
// by relType in the given direction, filtered to those visible to the caller.
func (a *App) visibleNeighborTitles(
	ctx context.Context, meta *metamodel.Metamodel, e *entityPkg.Entity, relType string, incoming bool,
) []string {
	var rels []*entityPkg.Relation
	if incoming {
		rels = a.reader.incomingRelations(ctx, e.ID)
	} else {
		rels = a.reader.outgoingRelations(ctx, e.ID)
	}

	var peerIDs []string
	for _, rel := range rels {
		if rel.Type != relType {
			continue
		}
		if incoming {
			peerIDs = append(peerIDs, rel.From)
		} else {
			peerIDs = append(peerIDs, rel.To)
		}
	}
	if len(peerIDs) == 0 {
		return nil
	}

	visible := visibleRelationIDs(ctx, a.reader, a.visibleReader, peerIDs)
	titles := make([]string, 0, len(peerIDs))
	for _, id := range peerIDs {
		if !visible[id] {
			continue
		}
		if node, ok := a.reader.getEntity(ctx, id); ok {
			titles = append(titles, displayTitleOrTitle(meta, node))
		}
	}
	return titles
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

// formatCellValue renders a property value as a single-line cell string.
func formatCellValue(v any) string {
	switch t := v.(type) {
	case nil:
		return ""
	case string:
		return t
	case []any:
		parts := make([]string, 0, len(t))
		for _, e := range t {
			parts = append(parts, formatCellValue(e))
		}
		return strings.Join(parts, ", ")
	case []string:
		return strings.Join(t, ", ")
	default:
		return fmt.Sprintf("%v", t)
	}
}

// escapeTableCell collapses newlines and escapes pipe/backslash so a cell value
// can't break out of its column or inject table structure.
func escapeTableCell(s string) string {
	s = strings.ReplaceAll(s, "\r\n", " ")
	s = strings.ReplaceAll(s, "\n", " ")
	s = strings.ReplaceAll(s, "\r", " ")
	s = strings.ReplaceAll(s, "\\", "\\\\")
	s = strings.ReplaceAll(s, "|", "\\|")
	return s
}
