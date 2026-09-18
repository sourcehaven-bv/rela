package dataentry

import (
	"context"
	"fmt"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// scopeRequest is what one collection read narrows by, beyond the ACL
// verdict itself.
//
// A struct rather than a parameter list because its purpose is to make the
// set of narrowings ENUMERABLE. Adding a dimension is then a field here plus
// one site in [scopedHeaders], instead of an edit to every handler that
// happens to read a collection — which is how the world and face narrowings
// each came to be threaded through some sites and not others.
type scopeRequest struct {
	// Type is the entity type to read. Required.
	Type string

	// Props are extra conjuncts ANDed onto the query: search prefilters,
	// pushed-down condition equalities. Applied to BOTH verdict branches.
	Props []store.PropPredicate

	// Faces limits the read to an allowlist of content states. Empty means
	// no face narrowing.
	Faces []entity.Face

	// Scope is the resolved query scope, or nil for an unscoped read.
	// Applied as a Go-side filter AFTER the store read, because the
	// predicate language is richer than store.PropOp: a scope may use `~=`,
	// ordered comparison and disjunction, none of which lower.
	//
	// Store-safe conjuncts ARE pushed, separately, via ScopeProps — this
	// field is the authoritative re-check that makes the pushdown a
	// superset optimisation rather than the filter itself.
	//
	// Typed as an opaque handle rather than a *predicate.Program because
	// arch-lint keeps the condition engine above this package: the
	// composition root compiles the scope and supplies a matching
	// ScopeEval, exactly as it does for next-action matchers.
	Scope QueryScopeHandle

	// ScopeProps are the conjuncts of Scope the store can evaluate,
	// pre-lowered by the caller. A SUPERSET of the scope: passing them
	// narrows the rows the store returns, and Scope then removes the rest.
	// Passing none is always correct, only slower.
	ScopeProps []store.PropPredicate

	// ScopeEval evaluates Scope against a row. Supplied by the caller
	// because this package may not import predicatefns (arch-lint keeps the
	// condition engine above the data-entry app).
	//
	// Required when Scope is non-nil; a nil evaluator with a non-nil scope
	// is refused rather than silently skipping the filter, which would
	// serve the unscoped set.
	ScopeEval QueryScopeEvaluator
}

// QueryScopeHandle is a compiled query scope, opaque to this package.
//
// The concrete type is *predicate.Program, which this package may not name
// (arch-lint: the condition/policy engine sits above the data-entry app). The
// handle travels from the composition root, which compiled it, to
// [QueryScopeEvaluator], which the same root supplied — so the two always
// agree about the dynamic type and nothing here has to.
type QueryScopeHandle any

// QueryScopeEvaluator reports whether one row satisfies a scope.
//
// Errors are propagated, never folded into "no match": an identity scope
// evaluated with no principal must fail the request rather than render an
// empty page that reads as "you have nothing".
type QueryScopeEvaluator func(
	ctx context.Context, scope QueryScopeHandle, entityType, id string, props map[string]any,
) (bool, error)

// scopedHeaders is the ONE place a data-entry collection read resolves the
// ACL verdict into a store query.
//
// # Why this exists
//
// Four handlers used to make this decision independently, and every
// narrowing dimension had to be threaded through all four by hand. Twice it
// was not, and both times the result was a FAIL-OPEN: RR-GQWRLD (a world
// carried on only the ACL-gated branch, so the AllowAll population silently
// fell back to the default world) and TKT-O7R2A1 (the same for faces,
// leaving the most privileged population with no face narrowing).
//
// The shape of that bug is what this function's structure prevents: the
// verdict switch and the narrowings are in one scope, so a new dimension
// added to [scopeRequest] is applied to both branches or neither.
//
// # The branches
//
// DenyAll yields nothing. AllowAll reads the type directly. A zero verdict
// is REFUSED rather than treated as AllowAll — a zero [acl.ReadQueryResult]
// would otherwise alias the most permissive branch, so the defensive error
// is the difference between a misconfiguration and a silent full disclosure.
// Otherwise the ACL's own query is copied and stamped.
//
// The copy is load-bearing: the ACL layer may cache a ReadQueryResult per
// principal, so stamping *rqr.Query in place would leak one request's
// narrowing into the next caller's.
//
// # Withheld vs empty
//
// withheld=true means the read was refused outright — a denied world or a
// DenyAll verdict — as opposed to a permitted read that matched nothing.
// The two are identical ON THE WIRE and must stay so, but a caller needs to
// tell them apart INTERNALLY to skip downstream work: running a free-text
// search for a principal who may read nothing lets them probe backend
// latency and induce load through `?q=` (RR-X56H, pinned by
// TestACLList_DenyAllSearchShortCircuit).
//
// # The one sanctioned duplicate
//
// [planListPushdown] (listpushdown.go) builds the SAME narrowed query in the
// paged shape the store can serve directly — ordering, limit, offset — so it
// cannot route through here and return a slice. The two serve one endpoint by
// different routes: a narrowing added to this function and not to that one
// makes a list's pushed-down page disagree with its Go-filtered page, which
// looks like a paging bug and is not one. Change both.
//
// Returns content-free headers (rowcontent.go); a caller that needs bodies
// loads them for the served page only.
func scopedHeaders(
	ctx context.Context, svc Services, rqr acl.ReadQueryResult, req scopeRequest,
) (headers []store.EntityHeader, withheld bool, err error) {
	// A denied world yields nothing, via the SAME empty-result path a
	// genuinely-empty world takes — so the two are identical on the wire.
	if worldFromContext(ctx).blocksAllReads() {
		return nil, true, nil
	}

	var out []store.EntityHeader
	switch {
	case rqr.DenyAll:
		return nil, true, nil

	case rqr.AllowAll && len(req.Props) == 0 && len(req.ScopeProps) == 0:
		// No extra conjuncts, so the cheaper type-scan serves it. Stamping
		// the world here as well as on the branches below is the RR-GQWRLD
		// invariant; note store.DefaultWorld() is the zero value, so a
		// surface with no world in ctx pays nothing for this.
		for h, err := range store.ListEntityHeaders(ctx, svc.Store, store.EntityQuery{
			Type:   req.Type,
			World:  worldScopeFrom(ctx),
			FaceIn: req.Faces,
		}) {
			if err != nil {
				return nil, false, fmt.Errorf("%w: %w", errListLoad, err)
			}
			out = append(out, h)
		}
		out, err = applyScope(ctx, out, req)
		return out, false, err

	case rqr.AllowAll:
		// Extra conjuncts need a GraphQuery even though the ACL permits
		// everything — and they must be carried HERE as well as on the
		// ACL-gated branch below. Props on only one branch is the same
		// fail-open shape as RR-GQWRLD: the AllowAll population would get
		// an unnarrowed read while everyone else got a narrowed one.
		q := store.GraphQuery{EntityType: req.Type, Props: allProps(req)}
		out, err := collectHeaders(ctx, svc, stampScope(ctx, q, req), errListLoad)
		if err != nil {
			return nil, false, err
		}
		out, err = applyScope(ctx, out, req)
		return out, false, err

	case rqr.Query == nil:
		// Defensive: a zero ReadQueryResult would otherwise alias AllowAll.
		// Fail loud instead of silently widening the read.
		return nil, false, fmt.Errorf("%w: zero ReadQueryResult for type %q", errACLListQuery, req.Type)

	default:
		// COPY before stamping — see the doc comment.
		q := *rqr.Query
		q.Props = append(append([]store.PropPredicate(nil), q.Props...), allProps(req)...)
		out, err := collectHeaders(ctx, svc, stampScope(ctx, q, req), errACLListQuery)
		if err != nil {
			return nil, false, err
		}
		out, err = applyScope(ctx, out, req)
		return out, false, err
	}
}

// allProps is the full conjunct set pushed to the store: the caller's own
// predicates plus the scope's pushable ones. Both narrow, both are ANDed, and
// both must ride on EVERY verdict branch — see the RR-GQWRLD note above.
func allProps(req scopeRequest) []store.PropPredicate {
	if len(req.ScopeProps) == 0 {
		return req.Props
	}
	out := make([]store.PropPredicate, 0, len(req.Props)+len(req.ScopeProps))
	out = append(out, req.Props...)
	return append(out, req.ScopeProps...)
}

// applyScope runs the query scope's predicate over rows the store returned.
//
// This is the AUTHORITATIVE filter. ScopeProps may have narrowed the read
// already, but only to conjuncts the store can evaluate — a scope using `~=`,
// an ordered comparison or a disjunction reaches here unnarrowed, and dropping
// this pass would serve those rows.
//
// A nil evaluator alongside a non-nil scope is an ERROR, not a skip: skipping
// serves the unscoped set, which is the failure direction a scope exists to
// prevent.
//
// Filters IN PLACE and the caller must not retain headers. Safe at every
// current call site because [scopedHeaders] builds the slice it passes and
// hands it to nobody else, but the reuse is invisible from the signature.
func applyScope(
	ctx context.Context, headers []store.EntityHeader, req scopeRequest,
) ([]store.EntityHeader, error) {
	if req.Scope == nil || len(headers) == 0 {
		return headers, nil
	}
	if req.ScopeEval == nil {
		return nil, fmt.Errorf("%w: query scope on %q has no evaluator", errListLoad, req.Type)
	}
	out := headers[:0]
	for _, h := range headers {
		ok, err := req.ScopeEval(ctx, req.Scope, h.Type, h.ID, h.Properties)
		if err != nil {
			// Surfaced, never swallowed into a non-match: an identity scope
			// with no principal must fail loudly rather than render an empty
			// page that reads as "you have nothing".
			return nil, fmt.Errorf("%w: query scope on %q: %w", errListLoad, req.Type, err)
		}
		if ok {
			out = append(out, h)
		}
	}
	return out, nil
}

// stampScope applies every ctx- and request-derived narrowing to q.
//
// Separated from [scopedHeaders] so the set of narrowings is readable in one
// place: if this function does not mention a dimension, no data-entry
// collection read applies it.
func stampScope(ctx context.Context, q store.GraphQuery, req scopeRequest) store.GraphQuery {
	q.World = worldScopeFrom(ctx)
	if len(req.Faces) > 0 {
		q.FaceIn = req.Faces
	}
	return q
}

// collectHeaders drains a GraphQueryHeaders iterator, failing loud on the
// first error rather than returning a partial slice — a truncated collection
// read is indistinguishable from a genuinely small one.
func collectHeaders(
	ctx context.Context, svc Services, q store.GraphQuery, errClass error,
) ([]store.EntityHeader, error) {
	var out []store.EntityHeader
	for h, err := range store.GraphQueryHeaders(ctx, svc.Store, q) {
		if err != nil {
			return nil, fmt.Errorf("%w: %w", errClass, err)
		}
		out = append(out, h)
	}
	return out, nil
}

// scopedEntities is [scopedHeaders] adapted to the header-entity shape most
// callers want. Separate rather than a flag because the header form is the
// one new code should reach for.
func scopedEntities(
	ctx context.Context, svc Services, rqr acl.ReadQueryResult, req scopeRequest,
) (entities []*entity.Entity, withheld bool, err error) {
	headers, withheld, err := scopedHeaders(ctx, svc, rqr, req)
	if err != nil {
		return nil, false, err
	}
	out := make([]*entity.Entity, 0, len(headers))
	for _, h := range headers {
		out = append(out, headerEntity(h))
	}
	return out, withheld, nil
}
