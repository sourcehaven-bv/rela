package dataentry

import (
	"cmp"
	"log/slog"
	"net/http"
	"slices"
	"strings"
	"time"

	v1 "github.com/Sourcehaven-BV/rela/internal/apiwire/v1"
	"github.com/Sourcehaven-BV/rela/internal/dataentryconfig"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/filter"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// sankeyHandler owns the read-only sankey endpoint. It never mutates.
//
// The pipeline order is a security property, as in the gantt (see
// [ganttHandler]):
//
//  1. row-gate: entities enter through the gantt's gated loader only
//  2. redact once: before any property (title, tone, sort key, where) is read
//  3. filter: `where:` runs on the redacted entity
//  4. cap: MaxNodes and truncated are computed on the filtered set
//
// A link is kept only when both ends are in the capped node set, so a link
// count or a node height can never reveal an entity the principal cannot read.
// The response is per-principal and must not be cached across principals.
type sankeyHandler struct {
	schema func() *Schema
	store  store.Store
	// nodes loads the redacted entities of one type through the row gate.
	nodes *ganttHandler
}

func newSankeyHandler(app *App, st store.Store, nodes *ganttHandler) *sankeyHandler {
	return &sankeyHandler{schema: app.State, store: st, nodes: nodes}
}

// isSankeyPath matches `_sankeys/{id}`, one sankey diagram.
func isSankeyPath(trimmed string) bool {
	id, ok := strings.CutPrefix(trimmed, "_sankeys/")
	return ok && id != "" && !strings.Contains(id, "/")
}

// handleV1Sankey serves GET /api/v1/_sankeys/{id}. {id} is a CONFIG key, so an
// unknown one gets a plain named 404 (config is not a secret).
//
//archplan:auth=required
func (h *sankeyHandler) handleV1Sankey(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeV1Error(w, r, http.StatusMethodNotAllowed, "method_not_allowed", "Method not allowed", "")
		return
	}
	id := strings.TrimPrefix(r.URL.Path, "/api/v1/_sankeys/")
	s := h.schema()
	cfg, ok := s.Cfg.Sankeys[id]
	if !ok || strings.Contains(id, "/") {
		writeV1Error(w, r, http.StatusNotFound, "not_found", "Sankey not found", "")
		return
	}

	nodes, faces, truncated, gerr := h.loadNodes(r, s, cfg)
	if gerr != nil {
		gerr.write(w, r)
		return
	}
	links, gerr := h.loadLinks(r, s, cfg, nodes, faces)
	if gerr != nil {
		gerr.write(w, r)
		return
	}
	if gerr := h.attachPips(r, s, cfg, nodes, faces); gerr != nil {
		gerr.write(w, r)
		return
	}
	writeV1JSON(w, http.StatusOK, v1.SankeyResponse{Nodes: nodes, Links: links, Truncated: truncated})
}

// loadNodes returns the gated, redacted, filtered nodes in column order, and
// the face each was read at.
func (h *sankeyHandler) loadNodes(
	r *http.Request, s *Schema, cfg dataentryconfig.Sankey,
) (outNodes []v1.SankeyNode, outFaces map[string]entity.Face, outTruncated bool, outErr *ganttError) {
	ctx := r.Context()
	now := time.Now()
	out := []v1.SankeyNode{}
	faces := map[string]entity.Face{}
	budget := cfg.MaxNodes
	truncated := false
	for _, col := range cfg.Columns {
		for _, typeName := range col {
			ents, gerr := h.nodes.loadGanttType(ctx, typeName)
			if gerr != nil {
				return nil, nil, false, gerr
			}
			slices.SortFunc(ents, func(a, b *entity.Entity) int { return cmp.Compare(a.ID, b.ID) })
			kept, gerr := h.filterNodes(now, s, typeName, cfg.Types[typeName], ents)
			if gerr != nil {
				return nil, nil, false, gerr
			}
			for _, n := range kept {
				if prev, dup := faces[n.ID]; dup && prev != entity.Face(n.Face) {
					slog.Error("sankey: two faces of one entity in the node set", "entity", n.ID)
					return nil, nil, false, &ganttError{http.StatusInternalServerError, "internal", "Ambiguous entity face", ""}
				}
				if budget == 0 {
					truncated = true
					continue
				}
				budget--
				faces[n.ID] = entity.Face(n.Face)
				out = append(out, n)
			}
		}
	}
	return out, faces, truncated, nil
}

// filterNodes applies the type's where: clauses to ALREADY-REDACTED entities
// and builds their nodes. Filters evaluate post-redact by design: membership
// must not reflect a predicate over a value the principal cannot read.
func (h *sankeyHandler) filterNodes(
	now time.Time, s *Schema, typeName string, t dataentryconfig.SankeyType, ents []*entity.Entity,
) ([]v1.SankeyNode, *ganttError) {
	entDef, ok := s.Meta.GetEntityDef(typeName)
	if !ok {
		return nil, nil
	}
	filters, err := parseWhereAt(t.Where, now)
	if err != nil {
		return nil, &ganttError{http.StatusInternalServerError, "internal", "Invalid sankey filter", ""}
	}
	label := t.Label
	if label == "" {
		label = entDef.GetPrimaryProperty()
	}
	out := make([]v1.SankeyNode, 0, len(ents))
	for _, red := range ents {
		if len(filters) > 0 {
			matched, merr := filter.MatchAll(entityRecord(red), filters, entDef, s.Meta)
			if merr != nil {
				slog.Warn("sankey: where filter errored; entity excluded", "entity", red.ID, "error", merr)
				continue
			}
			if !matched {
				continue
			}
		}
		n := v1.SankeyNode{ID: red.ID, Type: typeName, Face: string(red.Face)}
		if label != "" {
			n.Title = red.GetString(label)
		}
		if t.ColorBy != "" {
			n.Tone = red.GetString(t.ColorBy)
		}
		switch t.Order {
		case "":
		case "id":
			n.SortKey = red.ID
		default:
			n.SortKey = red.GetString(t.Order)
		}
		out = append(out, n)
	}
	return out, nil
}

// loadLinks lists the edge relations among the node set. An edge counts only
// when both ends are in the set, which is strictly narrower than the relation
// gate and needs no extra reads. Edge properties never reach the response.
func (h *sankeyHandler) loadLinks(
	r *http.Request, s *Schema, cfg dataentryconfig.Sankey, nodes []v1.SankeyNode, faces map[string]entity.Face,
) ([]v1.SankeyLink, *ganttError) {
	ctx := r.Context()
	ids := make([]string, 0, len(nodes))
	for _, n := range nodes {
		ids = append(ids, n.ID)
	}
	links := []v1.SankeyLink{}
	for _, relType := range cfg.Edges {
		q := store.RelationQuery{Type: relType, Direction: store.DirectionOutgoing, EntityIDs: ids, PeerIDs: ids}
		for rel, err := range h.store.ListRelations(ctx, q) {
			if err != nil {
				slog.Error("sankey: relation list failed", "relation", relType, "error", err)
				return nil, &ganttError{http.StatusInternalServerError, "internal", "Failed to list relations", ""}
			}
			from, fok := faces[rel.From]
			if _, tok := faces[rel.To]; !fok || !tok || rel.From == rel.To {
				continue
			}
			if !ownedByFace(s.Meta, rel, from) {
				continue
			}
			links = append(links, v1.SankeyLink{Source: rel.From, Target: rel.To, Relation: relType})
		}
	}
	return links, nil
}
