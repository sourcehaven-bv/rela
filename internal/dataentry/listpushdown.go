package dataentry

import (
	"context"
	"fmt"
	"strings"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	entityPkg "github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/queryplan"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// List paging pushdown (TKT-1U8XYN).
//
// The list pipeline's general shape — load the whole type, filter, sort and
// slice in Go — exists because some of its inputs cannot be expressed to
// the store: free-text search, relation filters, typed comparisons. Most
// requests carry none of those: the SPA's default list is "this type,
// maybe an equality filter, sorted by one property, page N". For that shape
// the store can do the filtering, ordering and paging itself, so a page
// costs one bounded read plus one count instead of a scan of the type.
//
// The pushed and the Go path must return the same rows in the same order,
// and that is a matter of eligibility, not of translation cleverness: the
// plan only pushes what both sides evaluate identically. Equality and
// inequality on scalar STRING-shaped properties (string, enum, date,
// datetime, custom types) compare byte-for-byte on both sides; ordering by
// such a property is byte-wise on both sides with absent values last (see
// applyV1Sorting). Anything else — integers (byte order is not numeric
// order), lists, `contains`/`in`/range operators, free text, relation
// filters — stays on the Go path. Pinned by the differential test in
// listpushdown_test.go.
//
// The ACL is honored by construction: the pushed query IS the principal's
// compiled read query (or an unconstrained one for an allow-all principal)
// with the request's world and face set stamped on a copy, and the total is
// the scoped count — never the type's row count (RR-SSPCCI, RR-J1VAW0).

// listPlan is a list request the store can page by itself. errClass is
// the failure the Go path would have reported for the same principal —
// errListLoad for an allow-all read, errACLListQuery for a compiled one —
// so a backend fault surfaces identically on either path.
type listPlan struct {
	query    store.GraphQuery
	errClass error
}

// planListPushdown decides whether the request is eligible and, if so,
// builds the paged query. ok=false means "take the Go path"; it is never
// an error, because ineligibility is the ordinary case for a request the
// planner does not understand.
func planListPushdown(
	meta *metamodel.Metamodel, typeName string, query map[string][]string,
	rqr acl.ReadQueryResult, world store.WorldScope, page, perPage int,
	isRelationKey func(string) bool,
) (listPlan, bool) {
	if queryGet(query, "q") != "" || rqr.DenyAll {
		return listPlan{}, false
	}
	def, ok := meta.GetEntityDef(typeName)
	if !ok {
		return listPlan{}, false
	}
	stringShaped := func(property string) bool {
		pd, ok := def.Properties[property]
		return ok && queryplan.StringShaped(meta, pd)
	}

	var props []store.PropPredicate
	for key, values := range query {
		if !strings.HasPrefix(key, "filter[") || len(values) == 0 {
			continue
		}
		property, operator, ok := parseRelationFilterKey(key)
		if !ok || strings.HasSuffix(key, "[]") || (isRelationKey != nil && isRelationKey(property)) {
			return listPlan{}, false
		}
		if !stringShaped(property) || len(values) != 1 {
			return listPlan{}, false
		}
		value := resolveFilterVariable(values[0])
		switch operator {
		case "eq":
			props = append(props, store.PropPredicate{
				Property: property, Op: store.PropEqual, Value: value, Scalar: value != "",
			})
		case "ne":
			if strings.Contains(value, ",") {
				return listPlan{}, false // a NOT IN list; the Go path handles it
			}
			props = append(props, store.PropPredicate{Property: property, Op: store.PropNotEqual, Value: value})
		default:
			return listPlan{}, false
		}
	}

	var order []store.OrderSpec
	for _, spec := range parseSortParam(query) {
		if !stringShaped(spec.Property) {
			return listPlan{}, false
		}
		order = append(order, store.OrderSpec{Property: spec.Property, Descending: spec.IsDescending()})
	}

	var gq store.GraphQuery
	errClass := errACLListQuery
	switch {
	case rqr.AllowAll:
		gq = store.GraphQuery{EntityType: typeName}
		errClass = errListLoad
	case rqr.Query != nil:
		gq = *rqr.Query // COPY: the ACL layer reuses its result per principal
	default:
		return listPlan{}, false
	}
	gq.World = world
	gq.FaceIn = rqr.Faces
	gq.Props = append(append([]store.PropPredicate(nil), gq.Props...), props...)
	gq.OrderBy = order
	gq.Limit = perPage
	gq.Offset = (page - 1) * perPage
	return listPlan{query: gq, errClass: errClass}, true
}

// run executes the plan: the scoped total in one count and the page in one
// content-free read. Rows come back as content-free entities
// (rowcontent.go), exactly what the Go path hands the handler.
func (p listPlan) run(ctx context.Context, st store.Store) (rows []*entityPkg.Entity, total int, err error) {
	total, err = scopedTotal(ctx, st, p.query)
	if err != nil {
		return nil, 0, fmt.Errorf("%w: %w", p.errClass, err)
	}
	rows = make([]*entityPkg.Entity, 0, p.query.Limit)
	for h, herr := range store.GraphQueryHeaders(ctx, st, p.query) {
		if herr != nil {
			return nil, 0, fmt.Errorf("%w: %w", p.errClass, herr)
		}
		rows = append(rows, headerEntity(h))
	}
	return rows, total, nil
}

// scopedTotal returns the number of rows the (ACL-scoped) query matches. It
// deliberately exposes only the matched count: GraphCount also reports the
// type's total, which for a gated principal is a count over rows they may
// not see (RR-SSPCCI), a handler holding both integers can ship the wrong
// one, and computing it is the costlier of the two statements.
func scopedTotal(ctx context.Context, st store.Store, q store.GraphQuery) (int, error) {
	return store.CountMatched(ctx, st, q)
}
