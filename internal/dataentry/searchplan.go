package dataentry

import (
	"context"
	"fmt"
	"slices"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/filter"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/queryplan"
	"github.com/Sourcehaven-BV/rela/internal/search"
	"github.com/Sourcehaven-BV/rela/internal/search/searchparser"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// Structured search pushdown (TKT-D3RAB0E).
//
// executeQuery's branch without free text loads every row of the named
// types, filters them in Go and sorts them in Go, so its cost grows with the
// store even when the caller keeps a handful of rows. For most structured
// queries the store can do all three: an equality on a string or enum
// property, and a sort on a string-shaped property, mean the same thing in
// the store as in the Go pass. A searchPlan is such a query. It reads one
// bounded page per type and counts only when the page was cut.
//
// The plan and the Go path must return the same rows in the same order. That
// is a matter of eligibility: [planSearch] declines anything the two sides
// might evaluate differently, and the caller then takes the Go path. The
// differential test in searchplan_test.go pins the agreement.
//
// The ACL is applied inside every query: each type's query is the one
// [scopedQuery] builds for the principal, with the world and the face
// allowlist stamped on, so the page, the count and the position are all
// computed over rows the principal may read.

// searchPlan is a structured search the store can answer by itself.
type searchPlan struct {
	// types are read in this order, as visibleListByTypes reads them. Empty
	// means the query matches nothing.
	types []string
	// props are the query's property filters, each one a predicate the
	// store evaluates exactly as filter.MatchAll does.
	props []store.PropPredicate
	// order is the query's sort. Set only when types has one entry: the
	// Go path sorts the union of the types, which no per-type read
	// reproduces.
	order []store.OrderSpec
}

// planSearch decides whether sq can be answered by the store and, if so,
// builds the plan. ok=false means "take the Go path"; declining is always
// correct, only slower.
//
// exactType is the one type the caller keeps afterwards, or "" for none
// (the `type` parameter of /_search, the Type of a search scope). Rows of
// any other type are dropped by the caller, so the plan does not read them.
func planSearch(meta *metamodel.Metamodel, sq *searchparser.SearchQuery, exactType string) (searchPlan, bool) {
	if meta == nil || sq.HasFreeText() {
		return searchPlan{}, false
	}
	types := uniqueTypes(sq.EntityTypes)
	if exactType != "" {
		if len(types) > 0 && !slices.Contains(types, exactType) {
			return searchPlan{}, true
		}
		types = []string{exactType}
	}
	if len(types) == 0 {
		return searchPlan{}, false
	}
	for _, typ := range types {
		if _, ok := meta.GetEntityDef(typ); !ok {
			return searchPlan{}, false
		}
	}

	plan := searchPlan{types: types}
	for _, f := range sq.PropertyFilters {
		p, ok := exactSearchPredicate(meta, types, f)
		if !ok {
			return searchPlan{}, false
		}
		plan.props = append(plan.props, p)
	}
	if !sq.HasSort() {
		return plan, true
	}
	if len(types) != 1 {
		return searchPlan{}, false
	}
	def, _ := meta.GetEntityDef(types[0])
	rollups := meta.RollupNames(types[0])
	for _, spec := range sq.SortClauses {
		pd, declared := def.Properties[spec.Property]
		if filter.IsVirtualSortProperty(spec.Property) || !declared || pd.List ||
			!queryplan.StringShaped(meta, pd) || slices.Contains(rollups, spec.Property) {

			return searchPlan{}, false
		}
		plan.order = append(plan.order, store.OrderSpec{
			Property:   spec.Property,
			Descending: spec.IsDescending(),
			Values:     queryplan.DeclaredValues(meta, def, spec.Property),
		})
	}
	return plan, true
}

// uniqueTypes returns types without repeats, in first-seen order, as
// visibleListByTypes reads them.
func uniqueTypes(types []string) []string {
	out := make([]string, 0, len(types))
	for _, typ := range types {
		if !slices.Contains(out, typ) {
			out = append(out, typ)
		}
	}
	return out
}

// exactSearchPredicate lowers f to a store predicate when the store answers
// it exactly as filter.MatchAll does on every one of types, and reports
// false otherwise.
//
// Only `=` and `!=` without a glob qualify, and only on a scalar property
// declared as a string or an enum (inline `values:` or a custom type) on
// every type. On those, filter.MatchAll compares the stored string to the
// value byte for byte, and emptiness is decided by internal/propmatch on
// both sides. Everything else declines, because Go and the store would
// compare it differently.
func exactSearchPredicate(meta *metamodel.Metamodel, types []string, f *filter.Filter) (store.PropPredicate, bool) {
	if f.IsGlob {
		return store.PropPredicate{}, false
	}
	var op store.PropOp
	switch f.Operator {
	case filter.OpEqual:
		op = store.PropEqual
	case filter.OpNotEqual:
		if f.Value == "" {
			return store.PropPredicate{}, false
		}
		op = store.PropNotEqual
	default:
		return store.PropPredicate{}, false
	}
	for _, typ := range types {
		def, ok := meta.GetEntityDef(typ)
		if !ok || slices.Contains(meta.RollupNames(typ), f.Property) {
			return store.PropPredicate{}, false
		}
		pd, ok := def.Properties[f.Property]
		if !ok || pd.List {
			return store.PropPredicate{}, false
		}
		values, isEnum := filter.EnumValues(meta, pd)
		switch {
		case pd.Type == metamodel.PropertyTypeString:
		case isEnum && (f.Value == "" || slices.Contains(values, f.Value)):
		default:
			return store.PropPredicate{}, false
		}
	}
	p := store.PropPredicate{Property: f.Property, Op: op, Value: f.Value}
	if op == store.PropEqual {
		p.Scalar = f.Value != ""
	}
	return p, true
}

// searchQueries builds the store query of each readable type in the plan,
// in read order. A type the principal may not read is left out, as
// visibleListByTypes leaves it out.
func (p searchPlan) searchQueries(
	ctx context.Context, scope map[string]search.TypeScope,
) ([]searchTypeQuery, error) {
	out := make([]searchTypeQuery, 0, len(p.types))
	for _, typ := range p.types {
		ts, ok := search.ResolveTypeScope(scope, typ)
		if !ok {
			continue
		}
		rqr := acl.ReadQueryResult{AllowAll: ts.AllowAll, Query: ts.Query, Faces: ts.Faces}
		q, errClass, withheld, err := scopedQuery(ctx, rqr, scopeRequest{Type: typ, Props: p.props, Faces: ts.Faces})
		if err != nil {
			return nil, err
		}
		if withheld {
			continue
		}
		q.OrderBy = p.order
		out = append(out, searchTypeQuery{query: q, errClass: errClass})
	}
	return out, nil
}

// searchTypeQuery is one type's store query and the error class its
// failure is reported as.
type searchTypeQuery struct {
	query    store.GraphQuery
	errClass error
}

// page reads up to limit rows of the plan's result, starting offset rows
// into it, and the number of rows it matches in all. limit must be
// positive: searchPage sets it, so no read is unbounded.
//
// The result is the types' results in order. With an offset, each type is
// counted first, so a type wholly before the offset is skipped without a
// read and the first type read starts at what is left of the offset. Each
// type then reads what is left of limit, so at most limit rows are read in
// all. Without an offset the total is counted only when the page is full,
// since a short page holds every match.
func (p searchPlan) page(
	ctx context.Context, svc Services, scope map[string]search.TypeScope, limit, offset int,
) (rows []*entity.Entity, total int, err error) {
	queries, err := p.searchQueries(ctx, scope)
	if err != nil {
		return nil, 0, err
	}
	var counts []int
	if offset > 0 {
		if counts, err = countQueries(ctx, svc, queries); err != nil {
			return nil, 0, err
		}
	}
	rows = make([]*entity.Entity, 0, min(limit, maxSearchResults))
	skip := offset
	for i, tq := range queries {
		if len(rows) == limit {
			break
		}
		q := tq.query
		if counts != nil {
			if skip >= counts[i] {
				skip -= counts[i]
				continue
			}
			q.Offset, skip = skip, 0
		}
		q.Limit = limit - len(rows)
		for h, err := range store.GraphQueryHeaders(ctx, svc.Store, q) {
			if err != nil {
				return nil, 0, fmt.Errorf("%w: %w", tq.errClass, err)
			}
			rows = append(rows, headerEntity(h))
		}
	}
	if counts == nil && len(rows) < limit {
		return rows, len(rows), nil
	}
	if counts == nil {
		if counts, err = countQueries(ctx, svc, queries); err != nil {
			return nil, 0, err
		}
	}
	for _, n := range counts {
		total += n
	}
	if len(rows) > 0 {
		total = max(total, offset+len(rows))
	}
	return rows, total, nil
}

// countQueries returns the number of rows each of queries matches, in order.
func countQueries(ctx context.Context, svc Services, queries []searchTypeQuery) ([]int, error) {
	out := make([]int, len(queries))
	for i, tq := range queries {
		n, err := store.CountMatched(ctx, svc.Store, tq.query)
		if err != nil {
			return nil, fmt.Errorf("%w: %w", tq.errClass, err)
		}
		out[i] = n
	}
	return out, nil
}

// position answers where id sits in the plan's result, for a plan over one
// type. ok=false means the plan reads several types, whose concatenation is
// not one store query; the caller takes the Go path.
func (p searchPlan) position(
	ctx context.Context, svc Services, scope map[string]search.TypeScope, id string,
) (pos store.Position, found, ok bool, err error) {
	if len(p.types) > 1 {
		return store.Position{}, false, false, nil
	}
	queries, err := p.searchQueries(ctx, scope)
	if err != nil || len(queries) == 0 {
		return store.Position{}, false, true, err
	}
	tq := queries[0]
	pos, found, err = store.GraphPosition(ctx, svc.Store, tq.query, id)
	if err != nil {
		return store.Position{}, false, true, fmt.Errorf("%w: %w", tq.errClass, err)
	}
	return pos, found, true, nil
}
