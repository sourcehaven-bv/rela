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
	"fmt"
	"iter"
	"slices"
	"sort"
	"strings"

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
// Exported so a caller that supplies a GraphQuery.HasInbound.Depth
// (or EntityDepth) can pin its own cap to the same value when it
// wants symmetric behavior. Backends that implement GraphQuery
// via natural-termination primitives (recursive-CTE UNION, etc.)
// are free to ignore this cap unless they'd expand past it.
const DepthCap = 5

// depthCap is the unexported alias for in-package use.
const depthCap = DepthCap

// Run executes q against r and yields matching entities. Errors abort
// the iterator.
func Run(ctx context.Context, r Reader, q store.GraphQuery) iter.Seq2[*entity.Entity, error] {
	return func(yield func(*entity.Entity, error) bool) {
		candidates, err := collectByType(ctx, r, q)
		if err != nil {
			yield(nil, err)
			return
		}
		paged := len(q.OrderBy) > 0 || q.Limit > 0 || q.Offset > 0
		var matched []*entity.Entity
		for _, e := range candidates {
			ok, err := matches(ctx, r, e, q)
			if err != nil {
				if !yield(nil, err) {
					return
				}
				continue
			}
			if !ok {
				continue
			}
			if !paged {
				if !yield(e, nil) {
					return
				}
				continue
			}
			matched = append(matched, e)
		}
		if !paged {
			return
		}
		// Ordering and paging apply to the matched set as a whole, so they
		// wait until every candidate has been judged.
		Order(matched, q.OrderBy)
		for _, e := range Page(matched, q.Offset, q.Limit) {
			if !yield(e, nil) {
				return
			}
		}
	}
}

// Order sorts rows in place by specs with GraphQuery.OrderBy's semantics:
// byte-wise on each property's string form, a row missing the property
// sorting as the largest value (last ascending, first descending — SQL's
// default null placement), id ascending as the final tiebreak. A stable
// sort, so equal keys keep store order.
func Order(rows []*entity.Entity, specs []store.OrderSpec) {
	if len(specs) == 0 {
		return
	}
	sort.SliceStable(rows, func(i, j int) bool {
		for _, spec := range specs {
			vi, oki := rows[i].Properties[spec.Property]
			vj, okj := rows[j].Properties[spec.Property]
			if oki != okj {
				// The present value is smaller than the absent one.
				return oki != spec.Descending
			}
			if !oki {
				continue
			}
			si, sj := fmt.Sprint(vi), fmt.Sprint(vj)
			if si == sj {
				continue
			}
			if spec.Descending {
				return si > sj
			}
			return si < sj
		}
		return rows[i].ID < rows[j].ID
	})
}

// Page returns the window rows[offset : offset+limit] (limit 0 = to the
// end), clamped to the slice.
func Page[T any](rows []T, offset, limit int) []T {
	if offset < 0 {
		offset = 0
	}
	if offset >= len(rows) {
		return nil
	}
	end := len(rows)
	if limit > 0 && offset+limit < end {
		end = offset + limit
	}
	return rows[offset:end]
}

// Count returns (matched, total) for q against r.
func Count(ctx context.Context, r Reader, q store.GraphQuery) (matched, total int, err error) {
	candidates, err := collectByType(ctx, r, q)
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
	for e, err := range r.ListEntities(ctx, store.EntityQuery{Type: q.EntityType, World: q.World, FaceIn: q.FaceIn}) {
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
//
// FaceIn travels for the same reason: it is the ACL's face allowlist, and a
// backend that drops it FAILS OPEN (store.EntityQuery.FaceIn) — a principal
// granted `read: [page@published]` would match the draft face here while the
// plain list beside it correctly hid it.
func collectByType(ctx context.Context, r Reader, q store.GraphQuery) ([]*entity.Entity, error) {
	if len(q.Any) > 0 && !q.World.IsDefaultWorld() {
		return collectBranchPrimes(ctx, r, q)
	}
	var out []*entity.Entity
	for e, err := range r.ListEntities(ctx, store.EntityQuery{
		Type: q.EntityType, World: q.World, FaceIn: q.FaceIn,
	}) {
		if err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, nil
}

// collectBranchPrimes is collectByType for a world-scoped query carrying
// [store.GraphQuery.Any]: every stored state of the type is a candidate,
// each branch's face set is applied to the CANDIDATES (the same
// before-the-rank position FaceIn takes, so a face a branch withholds falls
// through to the next chain coordinate rather than vanishing), and the
// survivors resolve through [store.ResolveWorldPrimes]. The relation half of
// a branch is a property of the entity, not of a face, so it is evaluated
// once per id.
//
// Ranking in the store's ListEntities cannot be used here because a branch
// filter is per (entity, face) and must run BEFORE that ranking — the SQL
// backends put it in the same WHERE clause the world's DISTINCT ON ranks
// over, and this is the in-Go equivalent.
func collectBranchPrimes(ctx context.Context, r Reader, q store.GraphQuery) ([]*entity.Entity, error) {
	rows := map[string]*entity.Entity{}
	var cands []store.WorldCandidate
	branchHolds := map[string][]bool{} // per id, per branch: the relation half
	for e, err := range r.ListEntities(ctx, store.EntityQuery{
		Type: q.EntityType, AllStates: true, FaceIn: q.FaceIn,
	}) {
		if err != nil {
			return nil, err
		}
		holds, seen := branchHolds[e.ID]
		if !seen {
			holds = make([]bool, len(q.Any))
			for i, br := range q.Any {
				ok := true
				if br.HasInbound != nil {
					var mErr error
					ok, mErr = matchesPredicate(ctx, r, e, *br.HasInbound, store.DirectionIncoming)
					if mErr != nil {
						return nil, mErr
					}
				}
				holds[i] = ok
			}
			branchHolds[e.ID] = holds
		}
		eligible := false
		for i, br := range q.Any {
			if holds[i] && (len(br.FaceIn) == 0 || slices.Contains(br.FaceIn, e.Face)) {
				eligible = true
				break
			}
		}
		if !eligible {
			continue
		}
		rows[e.ID+entity.StateRefSeparator+e.Face.String()] = e
		cands = append(cands, store.WorldCandidate{ID: e.ID, Type: e.Type, Face: e.Face})
	}
	primes := store.ResolveWorldPrimes(q.World, cands)
	out := make([]*entity.Entity, 0, len(primes))
	for id, res := range primes {
		if e, ok := rows[id+entity.StateRefSeparator+res.Face.String()]; ok {
			out = append(out, e)
		}
	}
	slices.SortFunc(out, func(a, b *entity.Entity) int { return strings.Compare(a.ID, b.ID) })
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
	if len(q.Any) > 0 {
		// Under a world the candidates were already branch-filtered before
		// ranking (collectBranchPrimes); re-checking the prime here is a
		// no-op there and the whole check for the default world.
		return matchesAny(ctx, r, e, q.Any)
	}
	return true, nil
}

// matchesAny reports whether at least one branch holds for e's stored face.
func matchesAny(ctx context.Context, r Reader, e *entity.Entity, branches []store.GraphBranch) (bool, error) {
	for _, br := range branches {
		if len(br.FaceIn) > 0 && !slices.Contains(br.FaceIn, e.Face) {
			continue
		}
		if br.HasInbound != nil {
			ok, err := matchesPredicate(ctx, r, e, *br.HasInbound, store.DirectionIncoming)
			if err != nil {
				return false, err
			}
			if !ok {
				continue
			}
		}
		return true, nil
	}
	return false, nil
}

// matchesProps reports whether every predicate holds (AND). Emptiness
// and equality are delegated to internal/propmatch so this agrees
// exactly with internal/filter and with the pgstore pushdown.
func matchesProps(e *entity.Entity, props []store.PropPredicate) bool {
	for _, p := range props {
		if p.Scalar && p.Op == store.PropEqual && p.Value != "" {
			value, ok := e.Properties[p.Property].(string)
			if !ok || value != p.Value {
				return false
			}
			continue
		}
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
