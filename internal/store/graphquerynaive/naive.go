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
	"cmp"
	"context"
	"errors"
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

// CheckEndpointShape validates a query's EndpointMatch chains: nesting must
// not exceed [DepthCap], and a NESTED hop must not carry an inheritance
// expansion. Exported so every backend enforces the identical bound — see the
// contract note on [store.RelationPredicate.EndpointMatch].
func CheckEndpointShape(q store.GraphQuery) error {
	for _, p := range []*store.RelationPredicate{q.HasInbound, q.HasOutbound} {
		if err := checkEndpointShape(p, 0); err != nil {
			return err
		}
	}
	for _, br := range q.Any {
		if err := checkEndpointShape(br.HasInbound, 0); err != nil {
			return err
		}
	}
	return nil
}

func checkEndpointShape(p *store.RelationPredicate, nesting int) error {
	if p == nil || p.EndpointMatch == nil {
		return nil
	}
	if nesting >= depthCap {
		return fmt.Errorf("graphquerynaive: endpoint match nested deeper than %d hops", depthCap)
	}
	for _, next := range []*store.RelationPredicate{
		p.EndpointMatch.HasInbound, p.EndpointMatch.HasOutbound,
	} {
		if next == nil {
			continue
		}
		if len(next.InheritThrough) > 0 || len(next.EntityInheritThrough) > 0 {
			return errors.New(
				"graphquerynaive: inheritance expansion is not supported on a nested endpoint match")
		}
		if err := checkEndpointShape(next, nesting+1); err != nil {
			return err
		}
	}
	return nil
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
	// Validate the query SHAPE before touching the store, so a refusal does not
	// depend on whether any candidate happens to reach the offending depth.
	// pgstore refuses the same shapes up front in checkGraphQueryScope;
	// validating lazily would make a query error on one backend and succeed on
	// the other purely by data.
	if err := CheckEndpointShape(q); err != nil {
		return func(yield func(*entity.Entity, error) bool) { yield(nil, err) }
	}
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
			vi, oki := sortValue(rows[i], spec.Property)
			vj, okj := sortValue(rows[j], spec.Property)
			if oki != okj {
				// The present value is smaller than the absent one.
				return oki != spec.Descending
			}
			if !oki {
				continue
			}
			si, sj := fmt.Sprint(vi), fmt.Sprint(vj)
			c := compareOrderValues(si, sj, spec.Values)
			if c == 0 {
				continue
			}
			if spec.Descending {
				return c > 0
			}
			return c < 0
		}
		return rows[i].ID < rows[j].ID
	})
}

// compareOrderValues ranks two values by their position in the declared order
// when one is given, and byte-wise otherwise.
//
// A value the schema does not declare sorts after every declared one, matching
// the `ELSE` arm a SQL backend emits for the same spec — so a row holding a
// value that was removed from the enum lands in the same place on every
// backend rather than wherever its text happens to fall.
func compareOrderValues(a, b string, values []string) int {
	if len(values) == 0 {
		return strings.Compare(a, b)
	}
	ia, ib := -1, -1
	for i, v := range values {
		if v == a && ia < 0 {
			ia = i
		}
		if v == b && ib < 0 {
			ib = i
		}
	}
	switch {
	case ia >= 0 && ib >= 0:
		return cmp.Compare(ia, ib)
	case ia >= 0:
		return -1
	case ib >= 0:
		return 1
	default:
		return strings.Compare(a, b)
	}
}

// sortValue reads a sort key the way SQL's `->>` does: a key that is
// missing OR holds JSON null is "no value" (SQL NULL, the largest), never
// the text "<nil>". Without this a null-valued property sorted between
// dates and absent rows in Go while PostgreSQL put it last.
func sortValue(e *entity.Entity, property string) (any, bool) {
	v, ok := e.Properties[property]
	if !ok || v == nil {
		return nil, false
	}
	return v, true
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
	if shapeErr := CheckEndpointShape(q); shapeErr != nil {
		return 0, 0, shapeErr
	}
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
	if err := CheckEndpointShape(q); err != nil {
		return nil, err
	}
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
	// Caller-supplied narrowing, ANDed with everything else including Any.
	// Checked here with the other in-memory predicates, and BEFORE the Any
	// check below, which returns rather than falling through.
	if !matchesNarrowing(e, q.Narrowing) {
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

// matchesNarrowing reports whether at least one caller-supplied branch holds
// (a disjunction), each branch being a conjunction of property predicates.
//
// No branches means no constraint. An EMPTY branch holds, making the whole
// disjunction vacuous — see [store.NarrowBranch]; a caller must drop the
// Narrowing rather than emit one.
func matchesNarrowing(e *entity.Entity, branches []store.NarrowBranch) bool {
	if len(branches) == 0 {
		return true
	}
	for _, br := range branches {
		if matchesProps(e, br.Props) {
			return true
		}
	}
	return false
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
		raw := e.Properties[p.Property]
		// PropNotEqualOrEmpty is PropNotEqual widened to accept an unset
		// property — the Lua `~=` reading. propmatch deliberately answers
		// the filter-DSL question instead (empty is not in the population),
		// so the empty case is decided here rather than by adding a second
		// meaning to propmatch.Decide, which internal/filter also depends on.
		if p.Op == store.PropNotEqualOrEmpty {
			if propmatch.IsEmpty(raw) {
				continue
			}
			if propmatch.Decide(raw, propmatch.OpNotEqual, p.Value) != propmatch.Match {
				return false
			}
			continue
		}
		if p.Op == store.PropGreaterEqual || p.Op == store.PropLessEqual {
			if !matchesOrdered(raw, p.Op, p.Value) {
				return false
			}
			continue
		}
		op := propmatch.OpEqual
		if p.Op == store.PropNotEqual {
			op = propmatch.OpNotEqual
		}
		if propmatch.Decide(raw, op, p.Value) != propmatch.Match {
			return false
		}
	}
	return true
}

// matchesOrdered decides [store.PropGreaterEqual] / [store.PropLessEqual]
// by comparing string forms byte-wise, matching [Order]'s semantics so a
// range predicate and a sort agree about what "larger" means.
//
// Empty and LIST values never match. Emptiness follows the PropNotEqual
// reading (an unset value is in no ordered population) and SQL's NULL
// comparison. Lists are refused because the two backends render them
// differently — Go's fmt.Sprint gives `[a b]` where postgres `->>` gives
// `["a", "b"]` — so any byte-wise answer would be backend-dependent. See
// the [store.PropGreaterEqual] doc.
func matchesOrdered(raw any, op store.PropOp, value string) bool {
	if propmatch.IsEmpty(raw) {
		return false
	}
	switch raw.(type) {
	case []string, []any:
		return false
	}
	s := fmt.Sprint(raw)
	if op == store.PropGreaterEqual {
		return s >= value
	}
	return s <= value
}

func matchesPredicate(
	ctx context.Context, r Reader, e *entity.Entity,
	p store.RelationPredicate, dir store.Direction,
) (bool, error) {
	return matchesPredicateAt(ctx, r, e, p, dir, 0)
}

// matchesPredicateAt is [matchesPredicate] carrying the current
// EndpointMatch NESTING level, which is distinct from the transitive-walk
// Depth the two InheritThrough expansions use: nesting counts hops the CALLER
// wrote literally, depth counts steps the store takes through one hop.
func matchesPredicateAt(
	ctx context.Context, r Reader, e *entity.Entity,
	p store.RelationPredicate, dir store.Direction, nesting int,
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

	found, err := hasMatchingRelation(
		ctx, r, candidates, dir, typeSet, endpointSet, anyEndpoint, p.EndpointMatch, nesting)
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
//
// match, when non-nil, additionally requires the entity on the far side of the
// edge to satisfy it. It is checked LAST, after the cheap type and id tests,
// because it is the only one that loads another entity.
func hasMatchingRelation(
	ctx context.Context, r Reader, candidates []string, dir store.Direction,
	typeSet, endpointSet map[string]bool, anyEndpoint bool, match *store.EndpointPredicate,
	nesting int,
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
			other := rel.To
			if dir == store.DirectionIncoming {
				other = rel.From
			}
			if !anyEndpoint && !endpointSet[other] {
				continue
			}
			if match == nil {
				return true, nil
			}
			ok, err := matchesEndpoint(ctx, r, other, match, nesting)
			if err != nil {
				return false, err
			}
			if ok {
				return true, nil
			}
		}
	}
	return false, nil
}

// matchesEndpoint reports whether the entity identified by id satisfies p:
// its type, its own properties, and any chained relation predicates.
//
// A missing endpoint entity is NOT a match. A dangling edge names a row that
// does not exist, and an absent row satisfies no property constraint — the
// same reading [matchesOrdered] gives an unset value, and the one the SQL
// backends give via an inner JOIN.
func matchesEndpoint(
	ctx context.Context, r Reader, id string, p *store.EndpointPredicate, nesting int,
) (bool, error) {
	// Bound the chain the CALLER wrote. Without this a hand-built query nests
	// without limit: each level costs a lookup here and, on the SQL backends, a
	// further JOIN whose alias grows one segment longer — so the emitted
	// statement grows QUADRATICALLY (measured: depth 1000 produced 6 MB of
	// SQL). The condition compiler caps authored chains, but store.GraphQuery
	// is a public type that any caller can build, so the floor belongs here
	// where every backend inherits it.
	if nesting >= depthCap {
		return false, fmt.Errorf(
			"graphquerynaive: endpoint match nested deeper than %d hops", depthCap)
	}
	// The SQL backends cannot emit an inheritance expansion on a NESTED hop
	// (see pgstore.checkEndpointNesting). This backend could — expandSet is
	// right there — but doing so would make the same policy and principal gate
	// differently per backend, which for an ACL-folded predicate is worse than
	// refusing. Refuse in both, so the store contract has one answer.
	for _, next := range []*store.RelationPredicate{p.HasInbound, p.HasOutbound} {
		if next == nil {
			continue
		}
		if len(next.InheritThrough) > 0 || len(next.EntityInheritThrough) > 0 {
			return false, errors.New(
				"graphquerynaive: inheritance expansion is not supported on a nested endpoint match")
		}
	}
	var found *entity.Entity
	for e, err := range r.ListEntities(ctx, store.EntityQuery{Type: p.EntityType, IDs: []string{id}}) {
		if err != nil {
			return false, err
		}
		found = e
		break
	}
	if found == nil {
		return false, nil
	}
	if !matchesProps(found, p.Props) {
		return false, nil
	}
	if p.HasInbound != nil {
		ok, err := matchesPredicateAt(ctx, r, found, *p.HasInbound, store.DirectionIncoming, nesting+1)
		if err != nil || !ok {
			return false, err
		}
	}
	if p.HasOutbound != nil {
		ok, err := matchesPredicateAt(ctx, r, found, *p.HasOutbound, store.DirectionOutgoing, nesting+1)
		if err != nil || !ok {
			return false, err
		}
	}
	return true, nil
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
