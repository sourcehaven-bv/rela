package dataentry

import (
	"context"

	entityPkg "github.com/Sourcehaven-BV/rela/internal/entity"
)

// neighborIDsOf collects the DISTINCT neighbor entity IDs referenced by a
// page's relation edges: the target (edge.To) of every outgoing edge and the
// source (edge.From) of every incoming edge. Empty IDs are skipped. This is the
// input side of the neighbor-visibility gate — a row-agnostic set for the whole
// page so the gate probe can be batched.
func neighborIDsOf(outgoing, incoming []*entityPkg.Relation) []string {
	seen := make(map[string]struct{}, len(outgoing)+len(incoming))
	var ids []string
	add := func(id string) {
		if id == "" {
			return
		}
		if _, dup := seen[id]; dup {
			return
		}
		seen[id] = struct{}{}
		ids = append(ids, id)
	}
	for _, edge := range outgoing {
		add(edge.To)
	}
	for _, edge := range incoming {
		add(edge.From)
	}
	return ids
}

// visibleRelationIDs returns the neighbor ids, out of candidates, that the
// principal may read. A neighbor id may appear in a row's `relations` map
// ONLY if its entity is visible, so `relations` and `included` never disagree.
// Pass ALL neighbor ids for the whole page at once: the probe is one
// content-free header read plus one batched face gate per neighbor type,
// never one call per id. Missing ids are not visible (fail-closed). The
// result feeds forWireRelated's visibleNeighbors.
func visibleRelationIDs(
	ctx context.Context, visible visibleReader, neighborIDs []string,
) map[string]bool {
	out := map[string]bool{}
	if len(neighborIDs) == 0 {
		return out
	}
	for id := range visible.servedIDs(ctx, neighborIDs) {
		out[id] = true
	}
	return out
}
