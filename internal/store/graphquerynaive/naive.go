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
	"log/slog"
	"maps"
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"

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

// CheckEndpointShape validates a query's shape: it must carry a valid face
// selection, as must any endpoint that sets its own ([store.ErrInvalidQuery]
// otherwise), EndpointMatch nesting must
// not exceed [DepthCap], and a NESTED hop must not carry an inheritance
// expansion. Exported so every backend enforces the identical bound — see the
// contract note on [store.RelationPredicate.EndpointMatch].
// utcDay is the length of a UTC calendar day; a date-shaped time.Time is
// a UTC midnight.
const utcDay = 24 * time.Hour

func CheckEndpointShape(q store.GraphQuery) error {
	if err := q.Faces.Validate(); err != nil {
		return err
	}
	if err := store.ValidatePropPredicates(q.Props); err != nil {
		return err
	}
	for _, br := range q.Narrowing {
		if err := store.ValidatePropPredicates(br.Props); err != nil {
			return err
		}
	}
	for _, p := range []*store.RelationPredicate{q.HasInbound, q.HasOutbound} {
		if err := checkEndpointShape(p, 0); err != nil {
			return err
		}
	}
	for _, br := range q.Any {
		for _, p := range []*store.RelationPredicate{br.HasInbound, br.HasOutbound} {
			if err := checkEndpointShape(p, 0); err != nil {
				return err
			}
		}
	}
	for i := range q.Related {
		if err := checkEndpointShape(&q.Related[i].Pred, 0); err != nil {
			return err
		}
	}
	if q.Where != nil {
		if err := store.ValidateCond(*q.Where); err != nil {
			return err
		}
		var relErr error
		q.Where.Walk(func(c store.Cond) bool {
			if c.Op == store.CondRelated {
				relErr = checkEndpointShape(&c.Rel.Pred, 0)
			}
			return relErr == nil
		})
		if relErr != nil {
			return relErr
		}
	}
	return nil
}

func checkEndpointShape(p *store.RelationPredicate, nesting int) error {
	if p == nil {
		return nil
	}
	if err := store.ValidatePropPredicates(p.EdgeProps); err != nil {
		return err
	}
	if p.EndpointMatch == nil {
		return nil
	}
	if nesting >= depthCap {
		return fmt.Errorf("graphquerynaive: endpoint match nested deeper than %d hops", depthCap)
	}
	if err := store.ValidatePropPredicates(p.EndpointMatch.Props); err != nil {
		return err
	}
	// An endpoint's own selection is optional (zero inherits the enclosing
	// one), but a set one must be valid: InWorld of an unset scope would
	// otherwise reach the SQL builders as a trivial world.
	if !p.EndpointMatch.Faces.IsZero() {
		if err := p.EndpointMatch.Faces.Validate(); err != nil {
			return err
		}
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
		rctx, candidates, err := answeredCandidates(ctx, r, q)
		if err != nil {
			yield(nil, err)
			return
		}
		paged := len(q.OrderBy) > 0 || q.Limit > 0 || q.Offset > 0
		var matched []*entity.Entity
		for _, e := range candidates {
			ok, err := matches(rctx, r, e, q)
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
	candidates, err := collectByType(ctx, r, q, nil)
	if err != nil {
		return 0, 0, err
	}
	if ctx, err = withRollupAnswers(ctx, r, q, candidates); err != nil {
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

// MatchingFaces returns, per input id, the faces of its selected rows that
// satisfy q's predicates, as [store.GraphQueryer.MatchingFaces] documents.
// Ids not in the store, of the wrong type, or with no matching row are
// absent.
func MatchingFaces(ctx context.Context, r Reader, q store.GraphQuery, ids []string) (map[string][]entity.Face, error) {
	if err := CheckEndpointShape(q); err != nil {
		return nil, err
	}
	out := make(map[string][]entity.Face)
	want := make(map[string]bool, len(ids))
	for _, id := range ids {
		want[id] = true
	}
	if len(want) == 0 {
		return out, nil
	}
	// The same candidates Run ranks, so an Any branch's face set trims the
	// faces BEFORE the world picks the prime here too (BUG-2SKLD3). They are
	// read for the candidate ids only: a world ranks each id's faces on
	// their own, so leaving the other ids out changes no prime, and the read
	// costs len(ids) rather than the whole type (TKT-F2EJQY8).
	cands, err := collectByType(ctx, r, q, slices.Sorted(maps.Keys(want)))
	if err != nil {
		return nil, err
	}
	if ctx, err = withRollupAnswers(ctx, r, q, cands); err != nil {
		return nil, err
	}
	for _, e := range cands {
		if !want[e.ID] {
			continue
		}
		ok, mErr := matches(ctx, r, e, q)
		if mErr != nil {
			return nil, mErr
		}
		if ok {
			out[e.ID] = append(out[e.ID], e.Face)
		}
	}
	for id, faces := range out {
		out[id] = store.SortedFaces(faces)
	}
	return out, nil
}

// collectByType seeds the candidate set: the entities the query may
// RETURN, so it carries the selection (store.GraphQuery.Faces). A non-empty
// ids restricts the candidates to those entities; nil reads the whole type. The relation
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
func collectByType(ctx context.Context, r Reader, q store.GraphQuery, ids []string) ([]*entity.Entity, error) {
	if w, ok := q.Faces.World(); ok && len(q.Any) > 0 && !w.IsTrivial() {
		return collectBranchPrimes(ctx, r, q, w, ids)
	}
	var out []*entity.Entity
	for e, err := range r.ListEntities(ctx, store.EntityQuery{
		Type: q.EntityType, IDs: ids, Faces: q.Faces, FaceIn: q.FaceIn,
	}) {
		if skipUnreadable(q, err) {
			continue
		}
		if err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, nil
}

// skipUnreadable reports whether a candidate row's err is one q leaves out:
// the row's values cannot be read ([store.ErrConflictedRow]) and q is an ACL
// read-grant query ([store.GraphQuery.SkipUnreadable]), so a left-out row is
// hidden. Every other query fails with err. The skip is logged at debug
// level, because a list logs it on every request while the conflict is open.
func skipUnreadable(q store.GraphQuery, err error) bool {
	if !q.SkipUnreadable || !errors.Is(err, store.ErrConflictedRow) {
		return false
	}
	slog.Debug("graphquery: hiding a row with unresolved merge conflicts", "err", err)
	return true
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
func collectBranchPrimes(
	ctx context.Context, r Reader, q store.GraphQuery, w store.WorldScope, ids []string,
) ([]*entity.Entity, error) {
	rows := map[string]*entity.Entity{}
	var cands []store.WorldCandidate
	branchHolds := map[string][]bool{} // per id, per branch: the relation half
	for e, err := range r.ListEntities(ctx, store.EntityQuery{
		Type: q.EntityType, IDs: ids, Faces: store.AllFaces(), FaceIn: q.FaceIn,
	}) {
		if skipUnreadable(q, err) {
			continue
		}
		if err != nil {
			return nil, err
		}
		holds, seen := branchHolds[e.ID]
		if !seen {
			var hErr error
			if holds, hErr = branchRelationHolds(ctx, r, e, q); hErr != nil {
				return nil, hErr
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
	primes := store.ResolveWorldPrimes(w, cands)
	out := make([]*entity.Entity, 0, len(primes))
	for id, res := range primes {
		if e, ok := rows[id+entity.StateRefSeparator+res.Face.String()]; ok {
			out = append(out, e)
		}
	}
	slices.SortFunc(out, func(a, b *entity.Entity) int { return strings.Compare(a.ID, b.ID) })
	return out, nil
}

// branchRelationHolds evaluates the relation half of each branch of q.Any
// for e. A branch without one holds.
func branchRelationHolds(ctx context.Context, r Reader, e *entity.Entity, q store.GraphQuery) ([]bool, error) {
	holds := make([]bool, len(q.Any))
	for i, br := range q.Any {
		ok, err := branchEdgesHold(ctx, r, e, br, q.Faces)
		if err != nil {
			return nil, err
		}
		holds[i] = ok
	}
	return holds, nil
}

// branchEdgesHold reports whether both relation halves of br hold for e. A
// half that is nil holds.
func branchEdgesHold(
	ctx context.Context, r Reader, e *entity.Entity, br store.GraphBranch, sel store.FaceSelection,
) (bool, error) {
	ctx = store.ReadToday(ctx) // an Any branch is an ACL gate (DEC-TDEF3GD)
	if br.HasInbound != nil {
		ok, err := matchesPredicate(ctx, r, e, *br.HasInbound, store.DirectionIncoming, sel)
		if err != nil || !ok {
			return false, err
		}
	}
	if br.HasOutbound != nil {
		ok, err := matchesPredicate(ctx, r, e, *br.HasOutbound, store.DirectionOutgoing, sel)
		if err != nil || !ok {
			return false, err
		}
	}
	return true, nil
}

func matches(ctx context.Context, r Reader, e *entity.Entity, q store.GraphQuery) (bool, error) {
	// Property predicates first: they are pure in-memory checks on an
	// entity already in hand, so a non-match skips the relation walks
	// (which do I/O per candidate).
	if ok, err := matchesProps(e, q.Props); err != nil || !ok {
		return false, err
	}
	// Caller-supplied narrowing, ANDed with everything else including Any.
	// Checked here with the other in-memory predicates, and BEFORE the Any
	// check below, which returns rather than falling through.
	if ok, err := matchesNarrowing(e, q.Narrowing); err != nil || !ok {
		return false, err
	}
	// HasInbound and HasOutbound are the ACL read gate: under a read instant
	// they read today's edges, as the SQL backends' gate does (DEC-TDEF3GD).
	gateCtx := store.ReadToday(ctx)
	if q.HasInbound != nil {
		ok, err := matchesPredicate(gateCtx, r, e, *q.HasInbound, store.DirectionIncoming, q.Faces)
		if err != nil || !ok {
			return ok, err
		}
	}
	if q.HasOutbound != nil {
		ok, err := matchesPredicate(gateCtx, r, e, *q.HasOutbound, store.DirectionOutgoing, q.Faces)
		if err != nil || !ok {
			return ok, err
		}
	}
	for _, rel := range q.Related {
		ok, err := matchesRelated(ctx, r, e, rel, q.Faces)
		if err != nil || !ok {
			return ok, err
		}
	}
	if q.Where != nil {
		ok, err := matchesCond(ctx, r, e, *q.Where, q.Faces)
		if err != nil || !ok {
			return ok, err
		}
	}
	if len(q.Any) > 0 {
		// Under a world the candidates were already branch-filtered before
		// ranking (collectBranchPrimes); re-checking the prime here is a
		// no-op there and the whole check for the default world.
		return matchesAny(ctx, r, e, q.Any, q.Faces)
	}
	return true, nil
}

// matchesRelated decides one [store.DirectedRelation] for e.
func matchesRelated(
	ctx context.Context, r Reader, e *entity.Entity, rel store.DirectedRelation, sel store.FaceSelection,
) (bool, error) {
	dir := store.DirectionOutgoing
	if rel.Incoming {
		dir = store.DirectionIncoming
	}
	return matchesPredicate(ctx, r, e, rel.Pred, dir, sel)
}

// matchesCond decides a [store.Cond] for e, two-valued: every leaf is true
// or false. And and Or short-circuit, so a property leaf ahead of a
// relation leaf can spare the relation walk. The shape was validated by
// [CheckEndpointShape]; an unknown op here is still refused.
func matchesCond(
	ctx context.Context, r Reader, e *entity.Entity, c store.Cond, sel store.FaceSelection,
) (bool, error) {
	switch c.Op {
	case store.CondProp:
		return matchesProp(e.Properties, c.Prop)
	case store.CondRelated:
		return matchesRelated(ctx, r, e, c.Rel, sel)
	case store.CondRollup:
		answers, ok := ctx.Value(rollupAnswersKey{}).(map[*store.RollupCond]map[string]bool)
		if !ok {
			return false, fmt.Errorf("%w: a roll-up leaf outside a graph query", store.ErrInvalidQuery)
		}
		return answers[c.Rollup][e.ID], nil
	case store.CondNot:
		if len(c.Args) != 1 {
			return false, store.ValidateCond(c)
		}
		ok, err := matchesCond(ctx, r, e, c.Args[0], sel)
		return !ok && err == nil, err
	case store.CondAnd:
		for _, a := range c.Args {
			if ok, err := matchesCond(ctx, r, e, a, sel); err != nil || !ok {
				return false, err
			}
		}
		return true, nil
	case store.CondOr:
		for _, a := range c.Args {
			if ok, err := matchesCond(ctx, r, e, a, sel); err != nil || ok {
				return ok && err == nil, err
			}
		}
		return false, nil
	default:
		return false, store.ValidateCond(c)
	}
}

// matchesNarrowing reports whether at least one caller-supplied branch holds
// (a disjunction), each branch being a conjunction of property predicates.
//
// No branches means no constraint. An EMPTY branch holds, making the whole
// disjunction vacuous — see [store.NarrowBranch]; a caller must drop the
// Narrowing rather than emit one.
func matchesNarrowing(e *entity.Entity, branches []store.NarrowBranch) (bool, error) {
	if len(branches) == 0 {
		return true, nil
	}
	for _, br := range branches {
		if ok, err := matchesProps(e, br.Props); err != nil || ok {
			return ok, err
		}
	}
	return false, nil
}

// matchesAny reports whether at least one branch holds for e's stored face.
func matchesAny(
	ctx context.Context, r Reader, e *entity.Entity, branches []store.GraphBranch, sel store.FaceSelection,
) (bool, error) {
	for _, br := range branches {
		if len(br.FaceIn) > 0 && !slices.Contains(br.FaceIn, e.Face) {
			continue
		}
		ok, err := branchEdgesHold(ctx, r, e, br, sel)
		if err != nil {
			return false, err
		}
		if ok {
			return true, nil
		}
	}
	return false, nil
}

// matchesProps reports whether every predicate holds (AND). Emptiness
// and equality are delegated to internal/propmatch so this agrees
// exactly with internal/filter and with the pgstore pushdown.
func matchesProps(e *entity.Entity, props []store.PropPredicate) (bool, error) {
	return MatchProps(e.Properties, props)
}

// MatchProps reports whether a property map satisfies every predicate (AND),
// with the [store.PropPredicate] semantics every backend shares. It is the
// Go form of the store's property test, exported for a caller that holds a
// row the store already returned (an edge's properties in aclstore) and must
// answer exactly as a [store.RelationPredicate.EdgeProps] query would.
func MatchProps(props map[string]any, preds []store.PropPredicate) (bool, error) {
	for _, p := range preds {
		ok, err := matchesProp(props, p)
		if err != nil || !ok {
			return false, err
		}
	}
	return true, nil
}

// matchesProp decides one predicate of [matchesProps]. Every operator has
// its own arm; an unknown one is [store.ErrInvalidQuery] rather than a
// guess.
func matchesProp(props map[string]any, p store.PropPredicate) (bool, error) {
	raw := props[p.Property]
	switch p.Op {
	case store.PropKeyEqual:
		return matchesKey(raw, p.Key, p.Value), nil
	case store.PropEqual:
		if p.Scalar && p.Value != "" {
			value, ok := raw.(string)
			return ok && value == p.Value, nil
		}
		return propmatch.Decide(raw, propmatch.OpEqual, p.Value) == propmatch.Match, nil
	case store.PropNotEqual:
		return propmatch.Decide(raw, propmatch.OpNotEqual, p.Value) == propmatch.Match, nil
	case store.PropNotEqualOrEmpty:
		if p.Scalar && p.Value != "" {
			// The negation of the scalar PropEqual arm: any value that is
			// not this string, including a list or a number.
			value, ok := raw.(string)
			return !ok || value != p.Value, nil
		}
		// PropNotEqualOrEmpty is PropNotEqual widened to accept an unset
		// property — the Lua `~=` reading. propmatch deliberately answers
		// the filter-DSL question instead (empty is not in the population),
		// so the empty case is decided here rather than by adding a second
		// meaning to propmatch.Decide, which internal/filter also depends on.
		return propmatch.IsEmpty(raw) || propmatch.Decide(raw, propmatch.OpNotEqual, p.Value) == propmatch.Match, nil
	case store.PropEqualOrEmpty:
		if propmatch.IsEmpty(raw) {
			return true, nil
		}
		equal := p
		equal.Op = store.PropEqual
		return matchesProp(props, equal)
	case store.PropGreaterEqual, store.PropLessEqual, store.PropGreater, store.PropLess:
		return matchesOrderedKind(raw, p)
	case store.PropIsDate:
		return isDateShape(raw), nil
	default:
		return false, store.UnknownPropOpError(p)
	}
}

// matchesOrderedKind decides an ordered predicate under its
// [store.OrderKind]. A value the kind cannot read never matches.
func matchesOrderedKind(raw any, p store.PropPredicate) (bool, error) {
	switch p.Kind {
	case store.OrderText:
		return matchesOrdered(raw, p.Op, p.Value), nil
	case store.OrderString:
		s, ok := raw.(string)
		return ok && orderHolds(p.Op, strings.Compare(s, p.Value)), nil
	case store.OrderInteger:
		want, err := strconv.ParseInt(p.Value, 10, 64)
		if err != nil {
			return false, store.ValidatePropPredicates([]store.PropPredicate{p})
		}
		n, ok := propmatch.Integer(raw)
		return ok && orderHolds(p.Op, cmp.Compare(n, want)), nil
	default:
		return false, store.ValidatePropPredicates([]store.PropPredicate{p})
	}
}

// orderHolds reports whether an ordered operator holds for a comparison
// result c (negative, zero or positive, as from cmp.Compare).
func orderHolds(op store.PropOp, c int) bool {
	switch op {
	case store.PropLess:
		return c < 0
	case store.PropLessEqual:
		return c <= 0
	case store.PropGreater:
		return c > 0
	case store.PropGreaterEqual:
		return c >= 0
	default:
		return false
	}
}

// isDateShape decides [store.PropIsDate]: a YYYY-MM-DD string of ASCII
// digits, or a time.Time at UTC midnight (an unquoted YAML date).
func isDateShape(raw any) bool {
	switch v := raw.(type) {
	case string:
		if len(v) != len("2006-01-02") {
			return false
		}
		for i := range len(v) {
			c := v[i]
			if i == 4 || i == 7 {
				if c != '-' {
					return false
				}
				continue
			}
			if c < '0' || c > '9' {
				return false
			}
		}
		return true
	case time.Time:
		return v.Location() == time.UTC && v.Equal(v.Truncate(utcDay))
	default:
		return false
	}
}

// matchesKey decides [store.PropKeyEqual]: raw must be an object whose key
// entry is the string value. Any other shape does not match.
func matchesKey(raw any, key, value string) bool {
	var entry any
	switch m := raw.(type) {
	case map[string]any:
		entry = m[key]
	case map[string]string:
		entry = m[key]
	default:
		return false
	}
	s, ok := entry.(string)
	return ok && s == value
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
	var s string
	switch v := raw.(type) {
	case []string, []any:
		return false
	case time.Time:
		// yaml.v3 decodes an unquoted timestamp into a time.Time, which the
		// file backends keep as is. The SQL backends hold the same value as
		// its JSON encoding, so compare that form rather than fmt.Sprint's
		// `2026-12-01 00:00:00 +0000 UTC`, which orders differently.
		s = v.Format(time.RFC3339Nano)
		if isDateShape(v) && isDateShape(value) {
			// An unquoted YAML date compared with a date compares as a
			// date, the way [isDateShape] already reads it, so `<=` holds
			// on the day itself (TKT-TKSCSBT).
			s = v.Format(time.DateOnly)
		}
	default:
		s = fmt.Sprint(raw)
	}
	return orderHolds(op, strings.Compare(s, value))
}

// matchesPredicate evaluates one relation predicate for e. sel is the
// selection an EndpointMatch inherits when it sets none of its own: the
// query's [store.GraphQuery.Faces] at the top level.
func matchesPredicate(
	ctx context.Context, r Reader, e *entity.Entity,
	p store.RelationPredicate, dir store.Direction, sel store.FaceSelection,
) (bool, error) {
	return matchesPredicateAt(ctx, r, e, p, dir, sel, 0)
}

// matchesPredicateAt is [matchesPredicate] carrying the current
// EndpointMatch NESTING level, which is distinct from the transitive-walk
// Depth the two InheritThrough expansions use: nesting counts hops the CALLER
// wrote literally, depth counts steps the store takes through one hop.
func matchesPredicateAt(
	ctx context.Context, r Reader, e *entity.Entity,
	p store.RelationPredicate, dir store.Direction, sel store.FaceSelection, nesting int,
) (bool, error) {
	if p.Gate {
		ctx = store.ReadToday(ctx)
	}
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
		ctx, r, candidates, dir, edgeMatch{
			types: typeSet, endpoints: endpointSet, anyEndpoint: anyEndpoint, props: p.EdgeProps,
		}, p.EndpointMatch, sel, nesting)
	if err != nil {
		return false, err
	}
	if p.Negate {
		return !found, nil
	}
	return found, nil
}

// edgeMatch is what one edge must satisfy on its own: its type, the entity
// on its far side, and its own properties ([store.RelationPredicate.EdgeProps]).
type edgeMatch struct {
	types, endpoints map[string]bool
	anyEndpoint      bool
	props            []store.PropPredicate
}

// admits reports whether rel, whose far endpoint is other, passes the type,
// endpoint and edge-property tests.
func (e edgeMatch) admits(rel *entity.Relation, other string) (bool, error) {
	if len(e.types) > 0 && !e.types[rel.Type] {
		return false, nil
	}
	if !e.anyEndpoint && !e.endpoints[other] {
		return false, nil
	}
	if len(e.props) == 0 {
		return true, nil
	}
	return MatchProps(rel.Properties, e.props)
}

// hasMatchingRelation reports whether any candidate has an edge in dir
// satisfying the type and endpoint constraints.
//
// match, when non-nil, additionally requires the entity on the far side of the
// edge to satisfy it. It is checked LAST, after the cheap type and id tests,
// because it is the only one that loads another entity.
func hasMatchingRelation(
	ctx context.Context, r Reader, candidates []string, dir store.Direction,
	edge edgeMatch, match *store.EndpointPredicate,
	sel store.FaceSelection, nesting int,
) (bool, error) {
	q := store.RelationQuery{Direction: dir}
	if match != nil {
		// An endpoint match follows identity-scoped edges only — see
		// [store.RelationPredicate.EndpointMatch]. matchesEndpoint picks the
		// endpoint's face from the selection.
		identity := entity.Face("")
		q.FromFace = &identity
	}
	for _, c := range candidates {
		q.EntityID = c
		for rel, err := range r.ListRelations(ctx, q) {
			if err != nil {
				return false, err
			}
			other := rel.To
			if dir == store.DirectionIncoming {
				other = rel.From
			}
			ok, err := edge.admits(rel, other)
			if err != nil {
				return false, err
			}
			if !ok {
				continue
			}
			if match == nil {
				return true, nil
			}
			ok, err = matchesEndpoint(ctx, r, other, match, sel, nesting)
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
	ctx context.Context, r Reader, id string, p *store.EndpointPredicate, sel store.FaceSelection, nesting int,
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
	// The endpoint's own selection wins; otherwise it inherits the enclosing
	// one (design A6 and section 12). Under InWorld this reads at most the
	// prime; under AllFaces or AtFaces every selected row, any of which may
	// match.
	if !p.Faces.IsZero() {
		sel = p.Faces
	}
	for found, err := range r.ListEntities(ctx, store.EntityQuery{
		Type: p.EntityType, IDs: []string{id}, Faces: sel,
	}) {
		if err != nil {
			return false, err
		}
		ok, err := matchesEndpointRow(ctx, r, found, p, sel, nesting)
		if err != nil || ok {
			return ok, err
		}
	}
	return false, nil
}

// matchesEndpointRow tests one stored row of an endpoint against p: its own
// properties, then the chained relation predicates, which inherit sel.
func matchesEndpointRow(
	ctx context.Context, r Reader, found *entity.Entity, p *store.EndpointPredicate,
	sel store.FaceSelection, nesting int,
) (bool, error) {
	if ok, err := matchesProps(found, p.Props); err != nil || !ok {
		return false, err
	}
	if p.HasInbound != nil {
		ok, err := matchesPredicateAt(ctx, r, found, *p.HasInbound, store.DirectionIncoming, sel, nesting+1)
		if err != nil || !ok {
			return false, err
		}
	}
	if p.HasOutbound != nil {
		ok, err := matchesPredicateAt(ctx, r, found, *p.HasOutbound, store.DirectionOutgoing, sel, nesting+1)
		if err != nil || !ok {
			return false, err
		}
	}
	return true, nil
}

// expandSet returns seeds plus everything reachable via the given
// relation types up to depth. BFS with visited-set; depth is bounded
// by depthCap.
//
// It follows IDENTITY-scoped edges only (a "" tail). A seed is an id, not a
// row, so the walk is entity-level for faced types too; but a content-scoped
// edge belongs to one face of its source, and letting it confer inheritance
// is deferred (stage-2 ruling D5): it fails toward less access.
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
	identity := entity.Face("")
	frontier := append([]string(nil), order...)
	for d := 0; d < depth && len(frontier) > 0; d++ {
		var next []string
		for _, n := range frontier {
			for rel, err := range r.ListRelations(ctx, store.RelationQuery{
				EntityID:  n,
				Direction: store.DirectionOutgoing,
				FromFace:  &identity,
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

// answeredCandidates is q's candidates and a context carrying the answers
// of q's roll-up leaves for them (see withRollupAnswers).
func answeredCandidates(
	ctx context.Context, r Reader, q store.GraphQuery,
) (context.Context, []*entity.Entity, error) {
	candidates, err := collectByType(ctx, r, q, nil)
	if err != nil {
		return ctx, nil, err
	}
	ctx, err = withRollupAnswers(ctx, r, q, candidates)
	return ctx, candidates, err
}

// rollupAnswersKey carries the roll-up leaf answers of one query run.
type rollupAnswersKey struct{}

// withRollupAnswers answers every roll-up leaf of q.Where for candidates,
// one batch per leaf ([store.RollupLeafAnswers]), so matchesCond judges a
// leaf without a store call per row. r must then be a [store.RollupReader].
func withRollupAnswers(
	ctx context.Context, r Reader, q store.GraphQuery, candidates []*entity.Entity,
) (context.Context, error) {
	if !store.HasRollupLeaf(q.Where) {
		return ctx, nil
	}
	rr, ok := r.(store.RollupReader)
	if !ok {
		return ctx, fmt.Errorf("%w: this reader cannot answer a roll-up leaf", store.ErrInvalidQuery)
	}
	ids := make([]string, 0, len(candidates))
	for _, e := range candidates {
		ids = append(ids, e.ID)
	}
	slices.Sort(ids)
	answers, err := store.RollupLeafAnswers(ctx, rr, *q.Where, slices.Compact(ids))
	if err != nil {
		return ctx, err
	}
	return context.WithValue(ctx, rollupAnswersKey{}, answers), nil
}
