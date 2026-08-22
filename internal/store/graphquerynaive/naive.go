// Package graphquerynaive ships the unoptimised, backend-agnostic
// implementation of [store.GraphQueryer]. All three of memstore,
// fsstore, and pgstore delegate to it today — keeping the algorithm
// in one place is the structural defense against behavior diverging
// across backends.
//
// Backends are expected to swap this for push-down implementations
// where the underlying engine can do better (recursive CTE in
// pgstore, index-backed walks in fsstore). Each swap remains
// behavioral-compatible because it is verified against the same
// inputs as the naive impl.
package graphquerynaive

import (
	"context"
	"iter"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/propmatch"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// Reader is the narrow read surface graphquerynaive needs. Declared
// here so backend implementations can pass `s` directly without
// constructing an adapter.
type Reader interface {
	ListEntities(ctx context.Context, q store.EntityQuery) iter.Seq2[*entity.Entity, error]
	ListRelations(ctx context.Context, q store.RelationQuery) iter.Seq2[*entity.Relation, error]
}

// DepthCap bounds every transitive walk performed by the naive
// implementation, as a safety backstop. The primary termination
// mechanism is the visited-set inside [expandSet]; the cap defends
// against pathological inputs (huge fan-out, deep chains) where
// even bounded BFS would be too expensive.
//
// Exported so a caller that supplies a [GraphQuery.HasInbound.Depth]
// (or EntityDepth) can pin its own cap to the same value when it
// wants symmetric behavior. Backends that implement [GraphQuery]
// via natural-termination primitives (recursive-CTE UNION, etc.)
// are free to ignore this cap unless they'd expand past it.
const DepthCap = 5

// depthCap is the unexported alias for in-package use.
const depthCap = DepthCap

// Run executes q against r and yields matching entities. Errors abort
// the iterator.
func Run(ctx context.Context, r Reader, q store.GraphQuery) iter.Seq2[*entity.Entity, error] {
	return func(yield func(*entity.Entity, error) bool) {
		candidates, err := collectByType(ctx, r, q.EntityType, q.World)
		if err != nil {
			yield(nil, err)
			return
		}
		for _, e := range candidates {
			ok, err := matches(ctx, r, e, q)
			if err != nil {
				if !yield(nil, err) {
					return
				}
				continue
			}
			if ok {
				if !yield(e, nil) {
					return
				}
			}
		}
	}
}

// Count returns (matched, total) for q against r.
func Count(ctx context.Context, r Reader, q store.GraphQuery) (matched, total int, err error) {
	candidates, err := collectByType(ctx, r, q.EntityType, q.World)
	if err != nil {
		return 0, 0, err
	}
	total = len(candidates)
	for _, e := range candidates {
		ok, mErr := matches(ctx, r, e, q)
		if mErr != nil {
			return matched, total, mErr
		}
		if ok {
			matched++
		}
	}
	return matched, total, nil
}

// MatchingIDs returns a map keyed by every input id with bool value
// indicating whether that id satisfies q's predicates. Ids not in the
// store, or in the store but of the wrong type, map to false. The
// returned map always has len(ids) keys (after dedup).
func MatchingIDs(ctx context.Context, r Reader, q store.GraphQuery, ids []string) (map[string]bool, error) {
	out := make(map[string]bool, len(ids))
	for _, id := range ids {
		out[id] = false
	}
	if len(out) == 0 {
		return out, nil
	}
	for e, err := range r.ListEntities(ctx, store.EntityQuery{Type: q.EntityType, World: q.World}) {
		if err != nil {
			return nil, err
		}
		if _, want := out[e.ID]; !want {
			continue
		}
		ok, mErr := matches(ctx, r, e, q)
		if mErr != nil {
			return nil, mErr
		}
		out[e.ID] = ok
	}
	return out, nil
}

// collectByType seeds the candidate set: the entities the query may
// RETURN, so it carries the world (store.GraphQuery.World). The relation
// walks in matches() deliberately do NOT — who an entity is related to
// must not depend on the reader's world.
//
// Passing the world here is load-bearing rather than cosmetic. The ACL
// read path swaps an EntityQuery for a GraphQuery as soon as a policy
// query exists (internal/visibility/pushdown.go), so dropping it would
// make a world-scoped list silently degrade to unscoped for exactly the
// gated principals: under `otherwise: exclude` the entities the world
// meant to hide become visible, and a published world serves drafts.
func collectByType(ctx context.Context, r Reader, typ string, w store.WorldScope) ([]*entity.Entity, error) {
	var out []*entity.Entity
	for e, err := range r.ListEntities(ctx, store.EntityQuery{Type: typ, World: w}) {
		if err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, nil
}

func matches(ctx context.Context, r Reader, e *entity.Entity, q store.GraphQuery) (bool, error) {
	// Property predicates first: they are pure in-memory checks on an
	// entity already in hand, so a non-match skips the relation walks
	// (which do I/O per candidate).
	if !matchesProps(e, q.Props) {
		return false, nil
	}
	if q.HasInbound != nil {
		ok, err := matchesPredicate(ctx, r, e, *q.HasInbound, store.DirectionIncoming)
		if err != nil || !ok {
			return ok, err
		}
	}
	if q.HasOutbound != nil {
		ok, err := matchesPredicate(ctx, r, e, *q.HasOutbound, store.DirectionOutgoing)
		if err != nil || !ok {
			return ok, err
		}
	}
	return true, nil
}

// matchesProps reports whether every predicate holds (AND). Emptiness
// and equality are delegated to internal/propmatch so this agrees
// exactly with internal/filter and with the pgstore pushdown.
func matchesProps(e *entity.Entity, props []store.PropPredicate) bool {
	for _, p := range props {
		op := propmatch.OpEqual
		if p.Op == store.PropNotEqual {
			op = propmatch.OpNotEqual
		}
		if propmatch.Decide(e.Properties[p.Property], op, p.Value) != propmatch.Match {
			return false
		}
	}
	return true
}

func matchesPredicate(
	ctx context.Context, r Reader, e *entity.Entity,
	p store.RelationPredicate, dir store.Direction,
) (bool, error) {
	endpoints, err := expandSet(ctx, r, p.Endpoints, p.InheritThrough, p.Depth)
	if err != nil {
		return false, err
	}
	candidates, err := expandSet(ctx, r, []string{e.ID}, p.EntityInheritThrough, p.EntityDepth)
	if err != nil {
		return false, err
	}

	endpointSet := make(map[string]bool, len(endpoints))
	for _, ep := range endpoints {
		endpointSet[ep] = true
	}
	typeSet := make(map[string]bool, len(p.OfTypes))
	for _, t := range p.OfTypes {
		typeSet[t] = true
	}

	// An empty Endpoints list means "any endpoint": the predicate is
	// then purely about the edge existing (optionally of OfTypes), which
	// is what an absence query like "has no implements edge at all"
	// needs. With endpoints named, only those count.
	anyEndpoint := len(p.Endpoints) == 0

	found, err := hasMatchingRelation(ctx, r, candidates, dir, typeSet, endpointSet, anyEndpoint)
	if err != nil {
		return false, err
	}
	if p.Negate {
		return !found, nil
	}
	return found, nil
}

// hasMatchingRelation reports whether any candidate has an edge in dir
// satisfying the type and endpoint constraints.
func hasMatchingRelation(
	ctx context.Context, r Reader, candidates []string, dir store.Direction,
	typeSet, endpointSet map[string]bool, anyEndpoint bool,
) (bool, error) {
	for _, c := range candidates {
		for rel, err := range r.ListRelations(ctx, store.RelationQuery{
			EntityID:  c,
			Direction: dir,
		}) {
			if err != nil {
				return false, err
			}
			if len(typeSet) > 0 && !typeSet[rel.Type] {
				continue
			}
			if anyEndpoint {
				return true, nil
			}
			other := rel.To
			if dir == store.DirectionIncoming {
				other = rel.From
			}
			if endpointSet[other] {
				return true, nil
			}
		}
	}
	return false, nil
}

// expandSet returns seeds plus everything reachable via the given
// relation types up to depth. BFS with visited-set; depth is bounded
// by depthCap.
func expandSet(ctx context.Context, r Reader, seeds, through []string, depth int) ([]string, error) {
	if len(seeds) == 0 {
		return nil, nil
	}
	if depth > depthCap {
		depth = depthCap
	}
	visited := make(map[string]bool, len(seeds))
	order := make([]string, 0, len(seeds))
	for _, s := range seeds {
		if !visited[s] {
			visited[s] = true
			order = append(order, s)
		}
	}
	if len(through) == 0 || depth <= 0 {
		return order, nil
	}
	throughSet := make(map[string]bool, len(through))
	for _, t := range through {
		throughSet[t] = true
	}
	frontier := append([]string(nil), order...)
	for d := 0; d < depth && len(frontier) > 0; d++ {
		var next []string
		for _, n := range frontier {
			for rel, err := range r.ListRelations(ctx, store.RelationQuery{
				EntityID:  n,
				Direction: store.DirectionOutgoing,
			}) {
				if err != nil {
					return order, err
				}
				if !throughSet[rel.Type] {
					continue
				}
				if visited[rel.To] {
					continue
				}
				visited[rel.To] = true
				order = append(order, rel.To)
				next = append(next, rel.To)
			}
		}
		frontier = next
	}
	return order, nil
}
