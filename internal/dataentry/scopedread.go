package dataentry

import (
	"context"
	"fmt"
	"maps"
	"slices"
	"strings"

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

	// ByIDs limits the read to the ids in IDs: a page scope's rows, or the
	// hits of a `?q=` search. The read then costs one MatchingFaces call
	// and one header read per matched face, whatever the size of the type.
	// ByIDs with no IDs reads nothing. A flag rather than "IDs non-nil",
	// because a nil slice that means "every row" next to an empty one that
	// means "no row" is a fail-open waiting to happen.
	//
	// The ids only narrow: each one still has to pass the verdict, the
	// world, the faces and the scope, exactly as on a full read.
	ByIDs bool
	IDs   []string

	// Scope is the resolved query scope, or nil for an unscoped read.
	// Applied as a Go-side filter AFTER the store read, because the
	// predicate language is richer than store.PropOp: a scope may use `~=`,
	// ordered comparison and disjunction, none of which lower.
	//
	// Store-safe conjuncts ARE pushed, separately, via ScopeProps — this
	// field is the authoritative re-check that makes the pushdown a
	// superset optimisation rather than the filter itself.
	//
	// A list page does not come here at all when its scope lowers exactly
	// (TKT-XKCNCL): listPage narrows its store query with
	// resolvedQueryScope.lower instead, and this Go pass is the fallback
	// for scopes that do not lower.
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

	// ScopeFilter applies Scope to the rows the store returned. Supplied by
	// the caller because this package may not import predicatefns (arch-lint
	// keeps the condition engine above the data-entry app).
	//
	// Required when Scope is non-nil; a nil filter with a non-nil scope is
	// refused rather than silently skipping the filter, which would serve
	// the unscoped set.
	ScopeFilter QueryScopeFilter
}

// QueryScopeHandle is a compiled query scope, opaque to this package.
//
// The concrete type is *predicate.Program, which this package may not name
// (arch-lint: the condition/policy engine sits above the data-entry app). The
// handle travels from the composition root, which compiled it, to
// [QueryScopeFilter], which the same root supplied — so the two always
// agree about the dynamic type and nothing here has to.
type QueryScopeHandle any

// QueryScopeFilter applies a scope to a batch of rows and returns the ones
// that satisfy it.
//
// A batch rather than a per-row call because a scope may contain
// `related(...)`, which is answered by one store query over all candidates —
// per row it would be the N+1 the collection-read rules forbid. gate
// authorizes each traversal for the request's principal; match answers it
// against the store. Both are passed per call and never retained, so no
// answer outlives the request that computed it.
//
// Errors are propagated, never folded into "no match": an identity scope
// evaluated with no principal must fail the request rather than render an
// empty page that reads as "you have nothing".
type QueryScopeFilter func(
	ctx context.Context, scope QueryScopeHandle, entityType string, headers []store.EntityHeader,
	gate func(context.Context, string, acl.TraversalHop) (*store.RelationPredicate, error),
	match func(context.Context, store.GraphQuery, []string) (map[string]bool, error),
) ([]store.EntityHeader, error)

// QueryScopeLower rewrites a scope as store predicates, so a list the store
// can page by itself stays paged when a scope applies (TKT-XKCNCL).
//
// ok=false means the scope does not lower exactly, or a traversal cannot be
// gated for this principal: take the [QueryScopeFilter] path, which gives the
// same rows or the same error. empty=true means a traversal is denied and
// none is refused, so no row matches. Otherwise frag's Props and Related are
// ANDed onto the read query. gate is the request's traversal gate, the one
// [QueryScopeFilter] receives.
type QueryScopeLower func(
	ctx context.Context, scope QueryScopeHandle, entityType string,
	gate func(context.Context, string, acl.TraversalHop) (*store.RelationPredicate, error),
) (frag store.GraphQuery, empty, ok bool)

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
// # Reading by ids
//
// With ByIDs the stamped query is answered for those ids alone
// ([headersByIDs]), so a page scope or a `?q=` search costs reads that follow
// its own size, not the size of the type (TKT-F2EJQY8). Without it the read
// scans the type, which is what a caller that needs the whole filtered set
// (totals, a Go-side sort, a condition or scope that does not lower) pays.
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
// looks like a paging bug and is not one. Change both. A query scope reaches
// the pushed path only through [QueryScopeLower], whose contract is that it
// selects exactly the rows [applyScope] keeps.
//
// Returns content-free headers (rowcontent.go); a caller that needs bodies
// loads them for the served page only.
func scopedHeaders(
	ctx context.Context, svc Services, rqr acl.ReadQueryResult, req scopeRequest,
) (headers []store.EntityHeader, withheld bool, err error) {
	// A denied world yields nothing, via the SAME empty-result path a
	// genuinely-empty world takes — so the two are identical on the wire.
	if readWithheld(ctx, rqr) {
		return nil, true, nil
	}

	// The verdict decides the query and its error class; the narrowings are
	// then stamped once, on whichever query the verdict produced.
	var q store.GraphQuery
	errClass := errACLListQuery
	switch {
	case rqr.AllowAll:
		// Extra conjuncts must be carried HERE as well as on the ACL-gated
		// branch below. Props on only one branch is the same fail-open
		// shape as RR-GQWRLD: the AllowAll population would get an
		// unnarrowed read while everyone else got a narrowed one.
		q = store.GraphQuery{EntityType: req.Type, Props: allProps(req)}
		errClass = errListLoad

	case rqr.Query == nil:
		// Defensive: a zero ReadQueryResult would otherwise alias AllowAll.
		// Fail loud instead of silently widening the read.
		return nil, false, fmt.Errorf("%w: zero ReadQueryResult for type %q", errACLListQuery, req.Type)

	default:
		// COPY before stamping — see the doc comment.
		q = *rqr.Query
		q.Props = append(append([]store.PropPredicate(nil), q.Props...), allProps(req)...)
	}
	// Stamping the world on every branch is the RR-GQWRLD invariant; note
	// store.TrivialScope() is the zero value, so a surface with no world in
	// ctx pays nothing for it.
	q = stampScope(ctx, q, req)

	var out []store.EntityHeader
	switch {
	case req.ByIDs:
		out, err = headersByIDs(ctx, svc, q, req.IDs, errClass)
	case rqr.AllowAll && len(q.Props) == 0:
		// No extra conjuncts, so the cheaper type-scan serves it.
		out, err = typeHeaders(ctx, svc, store.EntityQuery{Type: q.EntityType, Faces: q.Faces, FaceIn: q.FaceIn})
	default:
		out, err = collectHeaders(ctx, svc, q, errClass)
	}
	if err != nil {
		return nil, false, err
	}
	out, err = applyScope(ctx, svc, out, req)
	return out, false, err
}

// typeHeaders drains a type scan, failing loud like [collectHeaders].
func typeHeaders(ctx context.Context, svc Services, q store.EntityQuery) ([]store.EntityHeader, error) {
	var out []store.EntityHeader
	for h, err := range store.ListEntityHeaders(ctx, svc.Store, q) {
		if err != nil {
			return nil, fmt.Errorf("%w: %w", errListLoad, err)
		}
		out = append(out, h)
	}
	return out, nil
}

// headersByIDs answers q for the candidate ids only: one MatchingFaces call
// decides which rows of those ids q admits, and one header read per matched
// face loads them. Its cost follows len(ids), never the size of the type.
//
// q is the fully stamped query, so the verdict, the world, the face
// allowlist and the props apply exactly as on the scanning branches. The
// rows come back sorted by id, then face, so the order does not depend on
// the backend.
//
// The verdict and the header read are two statements, the same two-step
// [loadRowContent] takes for bodies. A write that lands between them can
// serve a row's newer header; a read never waits for writes, so the scan
// branches give no stronger guarantee across a request either.
func headersByIDs(
	ctx context.Context, svc Services, q store.GraphQuery, ids []string, errClass error,
) ([]store.EntityHeader, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	faces, err := svc.Store.MatchingFaces(ctx, q, ids)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", errClass, err)
	}
	byFace := make(map[entity.Face][]string)
	for id, fs := range faces {
		for _, f := range fs {
			byFace[f] = append(byFace[f], id)
		}
	}
	var out []store.EntityHeader
	for _, face := range slices.Sorted(maps.Keys(byFace)) {
		eq := store.EntityQuery{Type: q.EntityType, IDs: byFace[face], Faces: store.AtFaces(face)}
		for h, err := range store.ListEntityHeaders(ctx, svc.Store, eq) {
			if err != nil {
				// A plain load, not the gate query: errListLoad on both branches.
				return nil, fmt.Errorf("%w: %w", errListLoad, err)
			}
			out = append(out, h)
		}
	}
	slices.SortFunc(out, func(a, b store.EntityHeader) int {
		if c := strings.Compare(a.ID, b.ID); c != 0 {
			return c
		}
		return strings.Compare(string(a.Face), string(b.Face))
	})
	return out, nil
}

// readWithheld reports whether [scopedHeaders] refuses the read outright: a
// denied world or a DenyAll verdict. A caller that must do work BEFORE the
// read, such as the `?q=` search whose hits narrow it, asks this first, so a
// principal who may read nothing never reaches that work (RR-X56H).
func readWithheld(ctx context.Context, rqr acl.ReadQueryResult) bool {
	return worldFromContext(ctx).blocksAllReads() || rqr.DenyAll
}

// narrowingIDs is the id set a list read is narrowed to: the page scope's
// rows, the `?q=` hits, or the rows in both when both apply. narrowed is
// false when neither applies, and the read is then not narrowed at all.
// Sorted, so the store sees the same query for the same request.
func narrowingIDs(
	pageScoped bool, pageIDs map[string]bool, hits freeTextIDsForTypeResult,
) (ids []string, narrowed bool) {
	switch {
	case pageScoped && hits.HasFilter:
		for id := range hits.IDs {
			if pageIDs[id] {
				ids = append(ids, id)
			}
		}
	case pageScoped:
		for id, ok := range pageIDs {
			if ok {
				ids = append(ids, id)
			}
		}
	case hits.HasFilter:
		for id := range hits.IDs {
			ids = append(ids, id)
		}
	default:
		return nil, false
	}
	slices.Sort(ids)
	return ids, true
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
// A nil filter alongside a non-nil scope is an ERROR, not a skip: skipping
// serves the unscoped set, which is the failure direction a scope exists to
// prevent.
//
// A `related(...)` in the scope is authorized by the request's own read gate
// and answered with the same world and faces as the read that produced the
// candidates, so a traversal cannot see further than the list it filters.
func applyScope(
	ctx context.Context, svc Services, headers []store.EntityHeader, req scopeRequest,
) ([]store.EntityHeader, error) {
	if req.Scope == nil || len(headers) == 0 {
		return headers, nil
	}
	if req.ScopeFilter == nil {
		return nil, fmt.Errorf("%w: query scope on %q has no evaluator", errListLoad, req.Type)
	}
	match := func(ctx context.Context, q store.GraphQuery, ids []string) (map[string]bool, error) {
		return store.MatchingIDs(ctx, svc.Store, stampScope(ctx, q, req), ids)
	}
	out, err := req.ScopeFilter(ctx, req.Scope, req.Type, headers,
		traversalGateFromContext(ctx).GateTraversal, match)
	if err != nil {
		// Surfaced, never swallowed into a non-match: an identity scope
		// with no principal must fail loudly rather than render an empty
		// page that reads as "you have nothing".
		return nil, fmt.Errorf("%w: query scope on %q: %w", errListLoad, req.Type, err)
	}
	return out, nil
}

// stampScope applies every ctx- and request-derived narrowing to q.
//
// It stamps the ctx-derived dimensions, the world and the face allowlist.
// The other narrowings are applied in [scopedHeaders] itself: the props
// through allProps on both verdict branches, the ids through the ByIDs
// read, and the query scope through applyScope.
func stampScope(ctx context.Context, q store.GraphQuery, req scopeRequest) store.GraphQuery {
	q.Faces = store.InWorld(worldScopeFrom(ctx))
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
