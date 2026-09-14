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
}

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

	case rqr.AllowAll && len(req.Props) == 0:
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
		return out, false, nil

	case rqr.AllowAll:
		// Extra conjuncts need a GraphQuery even though the ACL permits
		// everything.
		q := store.GraphQuery{EntityType: req.Type}
		out, err := collectHeaders(ctx, svc, stampScope(ctx, q, req), errListLoad)
		return out, false, err

	case rqr.Query == nil:
		// Defensive: a zero ReadQueryResult would otherwise alias AllowAll.
		// Fail loud instead of silently widening the read.
		return nil, false, fmt.Errorf("%w: zero ReadQueryResult for type %q", errACLListQuery, req.Type)

	default:
		// COPY before stamping — see the doc comment.
		q := *rqr.Query
		q.Props = append(append([]store.PropPredicate(nil), q.Props...), req.Props...)
		out, err := collectHeaders(ctx, svc, stampScope(ctx, q, req), errACLListQuery)
		return out, false, err
	}
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
