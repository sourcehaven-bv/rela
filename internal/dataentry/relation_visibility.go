package dataentry

import (
	"context"
	"log/slog"

	entityPkg "github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/store"
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

// visibleRelationIDs computes the set of neighbor entity IDs (out of the given
// candidate IDs) that the request principal may read. It is the ACL gate for
// neighbor IDs on the list wire (RR-HJV8CP): a neighbor's ID may appear in a
// row's `relations` map ONLY if its entity is visible to the caller, so
// `relations` and the `included` map (already filtered by filterVisibleIncludes)
// can never disagree — closing the leak where a hidden incoming source OR
// outgoing target shipped its raw ID in the cell while being absent from
// `included`.
//
// Reuse contract (TKT-5U7QBR, TKT-NC3D08 inherit this):
//
//   - Pass ALL neighbor IDs for the whole page at once (both directions, every
//     row — see neighborIDsOf). The visibility probe is batched by entity type
//     via visibleReader.filterVisible: ONE PermitsReadMany per distinct neighbor
//     type for the entire page, NOT one gate call per id (RR-FRK1). Do NOT call
//     this per-row or per-id inside a loop; collect the page's IDs first.
//   - An entity's visibility is direction-independent, so one pass covers both
//     outgoing targets and incoming sources.
//   - The returned map contains only IDs that resolved to a loadable, readable
//     entity. Missing IDs are treated as not-visible (fail-closed) — the
//     serializer drops them from `relations`, matching how filterVisibleIncludes
//     drops unloadable candidates from `included`.
//
// The returned map is then handed to the serializer (forWireRelated's
// visibleNeighbors param); passing nil there disables filtering (the search /
// per-entity / include shapes that already emit nil relations stay untouched —
// RR-QO01XY).
//
// A free function (not an App method) taking the two read seams it needs, so it
// stays off App's receiver (keeps App under its plimsoll method cap) while the
// reuse contract above is unchanged.
func visibleRelationIDs(
	ctx context.Context, reader entityReader, visible visibleReader, neighborIDs []string,
) map[string]bool {
	if len(neighborIDs) == 0 {
		return map[string]bool{}
	}
	// ONE content-free read for the page's neighbor set (TKT-1U8XYN): the
	// gate needs each neighbor's type and id, never its body, and a header
	// batch is what store.EntityQuery.IDs exists for. An id the store no
	// longer has (a dangling edge) is simply absent, as the per-id lookup's
	// not-found was.
	candidates := make([]store.EntityHeader, 0, len(neighborIDs))
	for h, err := range store.ListEntityHeaders(ctx, reader.store, store.EntityQuery{IDs: neighborIDs}) {
		if err != nil {
			slog.Warn("dataentry: visibleRelationIDs: header batch failed; neighbors dropped fail-closed",
				"neighbors", len(neighborIDs), "err", err)
			return map[string]bool{}
		}
		candidates = append(candidates, h)
	}
	return visible.visibleHeaderIDs(ctx, candidates)
}
