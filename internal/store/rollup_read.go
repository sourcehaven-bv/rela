package store

import (
	"context"
	"maps"
	"slices"

	"github.com/Sourcehaven-BV/rela/internal/entity"
)

// readPerSource reads the identity-scoped edges of rel at each source's
// near ids, before any gate, outgoing or, with incoming, incoming. A
// source holding more than hopCap edges is over, and no source reads more
// than hopCap+1: one read serves every source until together they pass
// that bound, and then each source is read alone and stops there. edges
// holds each other source's edges.
func readPerSource(
	ctx context.Context, st RollupReader, rel string, incoming bool, nears map[string][]string, hopCap int,
) (edges map[string][]*entity.Relation, over map[string]bool, err error) {
	identity := entity.Face("")
	rq := RelationQuery{Type: rel, FromFace: &identity, Direction: DirectionOutgoing}
	nearOf := func(r *entity.Relation) string { return r.From }
	if incoming {
		rq.Direction = DirectionIncoming
		nearOf = func(r *entity.Relation) string { return r.To }
	}
	srcsOf := map[string][]string{}
	for src, ids := range nears {
		for _, id := range ids {
			srcsOf[id] = appendNew(srcsOf[id], src)
		}
	}
	if len(srcsOf) == 0 {
		return map[string][]*entity.Relation{}, map[string]bool{}, nil
	}
	rq.EntityIDs = slices.Sorted(maps.Keys(srcsOf))
	edges, full, err := readEdges(ctx, st, rq, nearOf, srcsOf, len(nears)*hopCap)
	if err != nil {
		return nil, nil, err
	}
	if !full {
		edges = map[string][]*entity.Relation{}
		for _, src := range slices.Sorted(maps.Keys(nears)) {
			if len(nears[src]) == 0 {
				continue
			}
			rq.EntityIDs = slices.Sorted(slices.Values(nears[src]))
			one, _, err := readEdges(ctx, st, rq, nearOf, map[string][]string{}, hopCap)
			if err != nil {
				return nil, nil, err
			}
			edges[src] = one[""]
		}
	}
	over = map[string]bool{}
	for src, es := range edges {
		if len(es) > hopCap {
			over[src] = true
			delete(edges, src)
		}
	}
	return edges, over, nil
}

// readEdges reads rq's edges per source of their near end, and stops past
// budget (source, edge) rows; full says it read them all. With no sources
// for a near end, the edge goes to the source "".
func readEdges(
	ctx context.Context, st RollupReader, rq RelationQuery, nearOf func(*entity.Relation) string,
	srcsOf map[string][]string, budget int,
) (edges map[string][]*entity.Relation, full bool, err error) {
	edges, read := map[string][]*entity.Relation{}, 0
	for r, err := range st.ListRelations(ctx, rq) {
		if err != nil {
			return nil, false, err
		}
		srcs, ok := srcsOf[nearOf(r)]
		if !ok {
			srcs = []string{""}
		}
		for _, src := range srcs {
			edges[src] = append(edges[src], r)
			read++
		}
		if read > budget {
			return edges, false, nil
		}
	}
	return edges, true, nil
}
