package store

import (
	"context"
	"fmt"
	"maps"
	"slices"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
)

// SiblingOrderProperty is the relation meta key whose value orders the
// siblings of a [SiblingWalk].
const SiblingOrderProperty = metamodel.OrderPropertyIn

// SiblingWalk is an earlier-siblings roll-up (design gap 2). A node's
// siblings under a parent p are the other sources of an identity-scoped
// Relation edge into p. Node n is behind under p when p passes a Parents
// gate and its Counts, and a sibling that passes a Siblings gate and its
// Counts has an order key before n's: [metamodel.CompareOrderKeys] over
// the edges' [metamodel.OrderPropertyIn], the source id and an empty tail.
//
// The nodes of a source are the source and, with Climb, each ancestor
// along Relation within Climb hops that a Parents gate admits; a hidden
// ancestor ends the climb. A node with several edges is judged under each
// parent. A source's gated rows count against RowCap: the climb's
// triples, its nodes' edges into a qualifying parent, and the qualifying
// siblings' edges into those parents. A source over it has a result with
// [RollupResult.OverCap]; each source is capped alone. So does a source
// whose nodes have more than HopCap edges, or whose qualifying parents
// more than HopCap incoming ones, gated or not; no backend reads more
// than HopCap+1 of them per source.
type SiblingWalk struct {
	// Relation is the canonical relation type, ordered on its incoming
	// side.
	Relation string
	// Parents gates the parents per type; Counts is the parent condition.
	// Siblings gates the siblings per type; Counts is the sibling
	// condition. A nil Counts holds on every gated row.
	Parents  map[string]AlongNode
	Siblings map[string]AlongNode
	// Climb is the most hops to an ancestor; zero judges the source alone.
	Climb int
	// CountSiblings makes [RollupResult.Count] the number of distinct
	// (node, sibling) pairs that put a node behind. Otherwise it is the
	// number of nodes that are behind.
	CountSiblings bool
	// RowCap caps the gated rows of each source; zero means [AlongRowCap].
	RowCap int
	// HopCap caps the edges each source reads per step, before any gate;
	// zero means [AlongHopCap]. The climb takes it too.
	HopCap int
}

// validateSiblings refuses a siblings query the backends would answer
// differently.
func validateSiblings(q RollupQuery) error {
	w := q.Siblings
	switch {
	case q.Along != nil || q.Relation != "" || len(q.Path) > 0 || q.Related.EntityType != "":
		return fmt.Errorf("%w: rollup: siblings excludes along, relation, path and related", ErrInvalidQuery)
	case len(q.Require) > 0 || len(q.PickRequire) > 0:
		return fmt.Errorf("%w: rollup: siblings takes its conditions on the gates", ErrInvalidQuery)
	case q.Func != RollupCount:
		return fmt.Errorf("%w: rollup: siblings counts", ErrInvalidQuery)
	case w.Relation == "":
		return fmt.Errorf("%w: rollup: siblings: relation is required", ErrInvalidQuery)
	case w.Climb < 0 || w.Climb > MaxAlongDepth:
		return fmt.Errorf("%w: rollup: siblings climb %d is outside 0 to %d", ErrInvalidQuery, w.Climb, MaxAlongDepth)
	case w.RowCap < 0 || w.HopCap < 0:
		return fmt.Errorf("%w: rollup: siblings row or hop cap is negative", ErrInvalidQuery)
	case w.Climb > 0 && w.CountSiblings:
		return fmt.Errorf("%w: rollup: siblings cannot count while it climbs", ErrInvalidQuery)
	}
	for what, nodes := range map[string]map[string]AlongNode{"parent": w.Parents, "sibling": w.Siblings} {
		for _, typ := range slices.Sorted(maps.Keys(nodes)) {
			node := nodes[typ]
			at := fmt.Sprintf("siblings %s type %q", what, typ)
			if node.Gate.EntityType != typ {
				return fmt.Errorf("%w: rollup: %s: the gate is for type %q", ErrInvalidQuery, at, node.Gate.EntityType)
			}
			if err := validateRollupGate(at, node.Gate); err != nil {
				return err
			}
			if node.Counts != nil {
				if err := ValidateCond(*node.Counts); err != nil {
					return fmt.Errorf("rollup: %s: %w", at, err)
				}
			}
		}
	}
	return nil
}

// SiblingClimb is the along walk of w's climb: Relation repeated up to
// Climb hops through the Parents gates, counting nothing. Every backend
// climbs with it, so the ancestors are the along walk's visits.
func SiblingClimb(w *SiblingWalk) *AlongWalk {
	through := make(map[string]AlongNode, len(w.Parents))
	for typ, node := range w.Parents {
		through[typ] = AlongNode{Gate: node.Gate}
	}
	return &AlongWalk{
		Segments: []AlongSegment{{Relation: w.Relation, Repeat: true, Through: through}},
		MaxDepth: w.Climb, RowCap: w.RowCap, HopCap: w.HopCap,
	}
}

// SiblingEdge is one identity-scoped edge of a sibling walk: its source,
// its parent and its stored order value.
type SiblingEdge struct {
	Node, Parent string
	Order        any
}

// orderKey is e's place among its parent's incoming edges.
func (e SiblingEdge) orderKey() metamodel.OrderKey {
	return metamodel.OrderKey{Value: e.Order, Peer: e.Node}
}

// rollupSiblings is [RollupGeneric] for a [SiblingWalk]: the climb, then
// the relation reads of readPerSource for the nodes' edges, one gated
// header read and one MatchingFaces per parent type, the relation reads for
// the siblings' edges, and one gated header read and one MatchingFaces per
// sibling type.
func rollupSiblings(ctx context.Context, st RollupReader, q RollupQuery) (map[string]RollupResult, error) {
	w := q.Siblings
	hopCap := w.HopCap
	if hopCap == 0 {
		hopCap = AlongHopCap
	}
	nodesOf := map[string][]string{}
	climbed := siblingClimbed{held: map[string]int{}, over: map[string]bool{}}
	if w.Climb > 0 {
		walked, err := walkAlong(ctx, st, SiblingClimb(w), q.Sources)
		if err != nil {
			return nil, err
		}
		climbed = siblingClimbed{held: walked.held, over: walked.over}
		for src, ids := range walked.visited {
			nodesOf[src] = slices.Collect(maps.Keys(ids))
		}
	}
	for _, src := range q.Sources {
		if !climbed.over[src] {
			nodesOf[src] = appendNew(nodesOf[src], src)
		}
	}
	nodeEdges, err := siblingEdges(ctx, st, w.Relation, nodesOf, false, hopCap, climbed.over)
	if err != nil {
		return nil, err
	}
	all := slices.Concat(slices.Collect(maps.Values(nodeEdges))...)
	parents := edgeEnds(all, func(e SiblingEdge) string { return e.Parent })
	qualify, err := siblingGate(ctx, st, w.Parents, w.Relation, parents, edgeEnds(all, SiblingEdge.node), true)
	if err != nil {
		return nil, err
	}
	parentsOf := map[string][]string{}
	for src, es := range nodeEdges {
		for _, e := range es {
			if qualify[e.Parent] {
				parentsOf[src] = appendNew(parentsOf[src], e.Parent)
			}
		}
	}
	sibEdges, err := siblingEdges(ctx, st, w.Relation, parentsOf, true, hopCap, climbed.over)
	if err != nil {
		return nil, err
	}
	sibs := distinctEdges(sibEdges)
	qp := slices.Sorted(maps.Keys(qualify))
	matching, err := siblingGate(ctx, st, w.Siblings, w.Relation, edgeEnds(sibs, SiblingEdge.node), qp, false)
	if err != nil {
		return nil, err
	}
	before := map[string][]SiblingEdge{}
	for _, e := range sibs {
		if matching[e.Node] {
			before[e.Parent] = append(before[e.Parent], e)
		}
	}
	for _, list := range before {
		slices.SortFunc(list, func(a, b SiblingEdge) int {
			return metamodel.CompareOrderKeys(a.orderKey(), b.orderKey())
		})
	}
	return foldSiblings(w, nodeEdges, parentsOf, before, climbed), nil
}

// siblingClimbed is what the climb held per source within its cap, and the
// sources over a cap: the climb's, or a read's of siblingEdges.
type siblingClimbed struct {
	held map[string]int
	over map[string]bool
}

// foldSiblings is each source's result over its nodes' edges: parentsOf
// holds each source's qualifying parents, and before each qualifying
// parent's matching siblings in key order. A source whose gated rows, with
// what its climb held, pass the cap is over it.
func foldSiblings(
	w *SiblingWalk, nodeEdges map[string][]SiblingEdge, parentsOf map[string][]string,
	before map[string][]SiblingEdge, climbed siblingClimbed,
) map[string]RollupResult {
	limit := w.RowCap
	if limit == 0 {
		limit = AlongRowCap
	}
	type pair struct{ node, peer string }
	out := make(map[string]RollupResult, len(parentsOf)+len(climbed.over))
	for src := range climbed.over {
		out[src] = RollupResult{OverCap: true}
	}
	for src, ps := range parentsOf {
		if climbed.over[src] {
			continue
		}
		pairs, behind := map[pair]bool{}, map[string]bool{}
		read := climbed.held[src]
		for _, e := range nodeEdges[src] {
			if !slices.Contains(ps, e.Parent) {
				continue
			}
			read++
			for _, t := range before[e.Parent] {
				if metamodel.CompareOrderKeys(t.orderKey(), e.orderKey()) >= 0 {
					break
				}
				pairs[pair{e.Node, t.Node}] = true
				behind[e.Node] = true
			}
		}
		for _, p := range ps {
			read += len(before[p])
		}
		if read > limit {
			out[src] = RollupResult{OverCap: true}
			continue
		}
		slices.Sort(ps)
		res := RollupResult{Count: len(behind), Parents: ps}
		if w.CountSiblings {
			res.Count = len(pairs)
		}
		out[src] = res
	}
	return out
}

// distinctEdges is the distinct edges of every source in bySrc.
func distinctEdges(bySrc map[string][]SiblingEdge) []SiblingEdge {
	type key struct{ node, parent string }
	seen := map[key]bool{}
	var out []SiblingEdge
	for _, src := range slices.Sorted(maps.Keys(bySrc)) {
		for _, e := range bySrc[src] {
			if k := (key{e.Node, e.Parent}); !seen[k] {
				seen[k] = true
				out = append(out, e)
			}
		}
	}
	return out
}

// node is the source end of e.
func (e SiblingEdge) node() string { return e.Node }

// edgeEnds is the sorted distinct ends of edges that end picks.
func edgeEnds(edges []SiblingEdge, end func(SiblingEdge) string) []string {
	out := make([]string, 0, len(edges))
	for _, e := range edges {
		out = append(out, end(e))
	}
	slices.Sort(out)
	return slices.Compact(out)
}

// siblingEdges reads, per source, the identity-scoped edges of rel at its
// ids in nearsOf: from them, or into them when incoming. A source reading
// more than hopCap edges is marked in over and has none.
func siblingEdges(
	ctx context.Context, st RollupReader, rel string, nearsOf map[string][]string, incoming bool, hopCap int,
	over map[string]bool,
) (map[string][]SiblingEdge, error) {
	edges, overRead, err := readPerSource(ctx, st, rel, incoming, nearsOf, hopCap)
	if err != nil {
		return nil, err
	}
	for src := range overRead {
		over[src] = true
	}
	out := make(map[string][]SiblingEdge, len(edges))
	for src, rs := range edges {
		for _, r := range rs {
			e := SiblingEdge{Node: r.From, Parent: r.To, Order: r.Properties[metamodel.OrderPropertyIn]}
			out[src] = append(out[src], e)
		}
	}
	return out, nil
}

// siblingGate returns the ids among ids that a gate of nodes admits and
// whose selected row meets the node's Counts. Each gate is seeded by the
// far ends of rel: ids hold edges of rel from ends when incoming, to ends
// otherwise.
func siblingGate(
	ctx context.Context, st RollupReader, nodes map[string]AlongNode, rel string, ids, ends []string, incoming bool,
) (map[string]bool, error) {
	out := map[string]bool{}
	if len(ids) == 0 {
		return out, nil
	}
	for _, typ := range slices.Sorted(maps.Keys(nodes)) {
		node := nodes[typ]
		gate := node.Gate
		gate.Related = append(slices.Clone(gate.Related), DirectedRelation{
			Incoming: incoming, Pred: RelationPredicate{OfTypes: []string{rel}, Endpoints: ends},
		})
		selected := map[string]entity.Face{}
		for h, err := range GraphQueryHeaders(ctx, st, gate) {
			if err != nil {
				return nil, err
			}
			if _, want := slices.BinarySearch(ids, h.ID); want {
				selected[h.ID] = h.Face
			}
		}
		if node.Counts == nil {
			for id := range selected {
				out[id] = true
			}
			continue
		}
		if len(selected) == 0 {
			continue
		}
		gate.Where = AndWhere(gate.Where, node.Counts)
		faces, err := st.MatchingFaces(ctx, gate, slices.Sorted(maps.Keys(selected)))
		if err != nil {
			return nil, err
		}
		for id, face := range selected {
			if slices.Contains(faces[id], face) {
				out[id] = true
			}
		}
	}
	return out, nil
}

// appendNew appends s to list unless list holds it.
func appendNew(list []string, s string) []string {
	if slices.Contains(list, s) {
		return list
	}
	return append(list, s)
}
