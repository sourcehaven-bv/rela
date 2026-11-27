package dataentry

import (
	"log/slog"
	"net/http"
	"sort"

	v1 "github.com/Sourcehaven-BV/rela/internal/apiwire/v1"
	"github.com/Sourcehaven-BV/rela/internal/dataentryconfig"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// attachPips fills each node's pips: the entities related to it that the
// principal may read, drawn as dots beside the node.
//
// Cost follows the diagram, not the pip type: per type with pips, ONE relation
// query over all of that type's node ids, then ONE gated, redacted read bounded
// to the candidate peer ids (the collection-reads rule; never a lookup per
// node, never a scan of the pip type).
//
// Order is a security property, as in loadNodes: a peer is row-gated and
// redacted before its title or tone is read, and the "+n" count is taken over
// the peers that survived the gate, so it cannot reveal an entity the principal
// cannot read. Pips are per-principal like the rest of the response.
func (h *sankeyHandler) attachPips(
	r *http.Request, s *Schema, cfg dataentryconfig.Sankey, nodes []v1.SankeyNode, faces map[string]entity.Face,
) *ganttError {
	typeNames := make([]string, 0, len(cfg.Types))
	for name := range cfg.Types {
		typeNames = append(typeNames, name)
	}
	sort.Strings(typeNames)
	for _, typeName := range typeNames {
		p := cfg.Types[typeName].Pips
		if p == nil {
			continue
		}
		if gerr := h.attachTypePips(r, s, typeName, p, nodes, faces); gerr != nil {
			return gerr
		}
	}
	return nil
}

func (h *sankeyHandler) attachTypePips(
	r *http.Request, s *Schema, typeName string, p *dataentryconfig.SankeyPips,
	nodes []v1.SankeyNode, faces map[string]entity.Face,
) *ganttError {
	ctx := r.Context()
	byID := map[string]int{}
	ids := []string{}
	for i, n := range nodes {
		if n.Type == typeName {
			byID[n.ID] = i
			ids = append(ids, n.ID)
		}
	}
	if len(ids) == 0 {
		return nil
	}
	pipDef, ok := s.Meta.GetEntityDef(p.Type)
	if !ok {
		return nil
	}
	label := p.Label
	if label == "" {
		label = pipDef.GetPrimaryProperty()
	}

	cands, gerr := h.pipEdges(r, p, byID, ids)
	if gerr != nil {
		return gerr
	}
	peerIDs := make([]string, 0, len(cands))
	seen := map[string]bool{}
	for _, c := range cands {
		if !seen[c.peer] {
			seen[c.peer] = true
			peerIDs = append(peerIDs, c.peer)
		}
	}
	peers, gerr := h.nodes.loadGanttTypeByIDs(ctx, p.Type, peerIDs)
	if gerr != nil {
		return gerr
	}
	peerByID := make(map[string]*entity.Entity, len(peers))
	for _, e := range peers {
		if _, dup := peerByID[e.ID]; !dup {
			peerByID[e.ID] = e
		}
	}
	found := keepVisiblePipEdges(s, cands, peerByID, faces)

	for nodeID, set := range found {
		peerIDs := make([]string, 0, len(set))
		for id := range set {
			peerIDs = append(peerIDs, id)
		}
		sort.Strings(peerIDs)
		n := &nodes[byID[nodeID]]
		for i, id := range peerIDs {
			if i >= p.Max {
				n.PipsMore = len(peerIDs) - p.Max
				break
			}
			e := peerByID[id]
			pip := v1.SankeyPip{ID: e.ID, Type: p.Type, Face: string(e.Face)}
			if label != "" {
				pip.Title = e.GetString(label)
			}
			if p.ColorBy != "" {
				pip.Tone = e.GetString(p.ColorBy)
			}
			n.Pips = append(n.Pips, pip)
		}
	}
	return nil
}

// pipEdge is one relation between a node and a candidate peer, before the
// peer has passed the read gate.
type pipEdge struct {
	node, peer string
	nodeIsFrom bool
	rel        *entity.Relation
}

// pipEdges lists the relations between the nodes of one type and their
// candidate peers. It is ONE relation query over all of the type's node ids.
func (h *sankeyHandler) pipEdges(
	r *http.Request, p *dataentryconfig.SankeyPips, byID map[string]int, ids []string,
) ([]pipEdge, *ganttError) {
	q := store.RelationQuery{Type: p.Relation, Direction: store.DirectionBoth, EntityIDs: ids}
	var out []pipEdge
	for rel, err := range h.store.ListRelations(r.Context(), q) {
		if err != nil {
			slog.Error("sankey: pip relation list failed", "relation", p.Relation, "error", err)
			return nil, &ganttError{http.StatusInternalServerError, "internal", "Failed to list relations", ""}
		}
		nodeID, peerID, nodeIsFrom := rel.From, rel.To, true
		if _, isNode := byID[nodeID]; !isNode {
			nodeID, peerID, nodeIsFrom = rel.To, rel.From, false
		}
		if _, isNode := byID[nodeID]; !isNode || nodeID == peerID {
			continue
		}
		out = append(out, pipEdge{node: nodeID, peer: peerID, nodeIsFrom: nodeIsFrom, rel: rel})
	}
	return out, nil
}

// keepVisiblePipEdges drops the edges whose peer the principal may not read,
// or whose content-scoped edge belongs to another face. It returns node id ->
// peer ids. The "+n" count is taken over this result, after the gate.
func keepVisiblePipEdges(
	s *Schema, cands []pipEdge, peerByID map[string]*entity.Entity, faces map[string]entity.Face,
) map[string]map[string]bool {
	found := map[string]map[string]bool{}
	for _, c := range cands {
		peer, visible := peerByID[c.peer]
		if !visible {
			continue
		}
		tail := faces[c.node]
		if !c.nodeIsFrom {
			tail = peer.Face
		}
		if !ownedByFace(s.Meta, c.rel, tail) {
			continue
		}
		if found[c.node] == nil {
			found[c.node] = map[string]bool{}
		}
		found[c.node][c.peer] = true
	}
	return found
}
