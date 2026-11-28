package dataentry

import (
	"context"
	"fmt"
	"net/url"
	"strings"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	entityPkg "github.com/Sourcehaven-BV/rela/internal/entity"
)

// listPipeline is the shared list read: field gate, ACL read scope, load,
// free-text intersection, filters, sort and the list `condition:`. The list
// endpoint, `/_position`, the nav items and status, the dashboard cards,
// export, the gantt and the next-action candidates all read through it, so
// they agree on which rows a list holds and in what order. Extracted from App
// so those consumers depend on the pipeline rather than on App, and so the
// entity read handler does not become the hub they reach through.
//
// The collaborators are closures over App rather than captured values: tests
// and the wiring setters reassign App.store, App.fieldResolver,
// App.affordances, App.queries and App.queryScopes after construction, and a
// captured copy would go stale (the schemaHandler idiom).
type listPipeline struct {
	// schema is the reloadable snapshot.
	schema func() *Schema
	// services returns the read bundle over the app's current store.
	services func() Services
	// fields answers the field verdicts that prime a page and gate a query.
	fields func() FieldVerdictResolver
	// affordances backs the redactor a condition and a relation filter judge
	// rows through.
	affordances func() affordanceService
	// visible is the row-gated reader a relation filter resolves its
	// neighbors through.
	visible func() visibleReader
	// queries answers the free-text `?q=` id set.
	queries func() *queryService
	// queryScopes compiles `query_scopes:`; see [App.SetQueryScopeResolver].
	queryScopes func() QueryScopeResolverFunc
	// rollups computes the queried roll-ups a filter or sort reads.
	rollups func() rollupSource
	// gate is gateAppListQuery bound to the app: the field gate plus the
	// relation-filter classification.
	gate func(ctx context.Context, state *Schema, typeName string, query map[string][]string) (gatedListQuery, error)
	// narrowing is resolveListNarrowing bound to the app: the list
	// `condition:` and the query scope.
	narrowing func(
		ctx context.Context, state *Schema, typeName string, query map[string][]string,
	) (context.Context, listNarrowing, error)
	// listOrder returns the manual-order store, or nil when not wired.
	listOrder func() listOrderStore
	// pageScope is resolvePageScope bound to the app.
	pageScope func(
		ctx context.Context, query map[string][]string, typeName string,
	) (ids map[string]bool, ordering *relationOrdering, active bool, err error)
}

// newListPipeline wires the pipeline to app's live collaborators.
func newListPipeline(app *App) *listPipeline {
	return &listPipeline{
		schema:      app.State,
		services:    app.Services,
		fields:      func() FieldVerdictResolver { return app.fieldResolver },
		affordances: func() affordanceService { return app.affordances },
		visible:     func() visibleReader { return app.visibleReader },
		queries:     func() *queryService { return app.queries },
		queryScopes: func() QueryScopeResolverFunc { return app.queryScopes },
		rollups:     func() rollupSource { return newRollupSource(app) },
		listOrder:   func() listOrderStore { return app.listOrder },
		gate: func(
			ctx context.Context, state *Schema, typeName string, query map[string][]string,
		) (gatedListQuery, error) {
			return gateAppListQuery(ctx, app, state, typeName, query)
		},
		narrowing: func(
			ctx context.Context, state *Schema, typeName string, query map[string][]string,
		) (context.Context, listNarrowing, error) {
			return resolveListNarrowing(ctx, app, state, typeName, query)
		},
		pageScope: func(
			ctx context.Context, query map[string][]string, typeName string,
		) (map[string]bool, *relationOrdering, bool, error) {
			return resolvePageScope(ctx, app, query, typeName)
		},
	}
}

// listPage returns one page of the list plus the scoped total. A request the
// store can page by itself (listpushdown.go) costs one count and one bounded
// read; everything else takes the whole-type Go pipeline and slices it.
func (p *listPipeline) listPage(
	ctx context.Context, typeName string, query map[string][]string, page, perPage int,
) (rows []*entityPkg.Entity, total int, ordering *relationOrdering, err error) {
	state := p.schema()
	gated, err := p.gate(ctx, state, typeName, query)
	if err != nil {
		return nil, 0, nil, err
	}
	ctx, n, err := p.narrowing(ctx, state, typeName, query)
	if err != nil {
		return nil, 0, nil, err
	}
	manualID, manual := manualListFor(state, query)
	orders := p.listOrder()
	if orders == nil {
		manual = nil
	}
	if plan, empty, ok := n.pushdownPlan(ctx, typeName, gated, page, perPage); ok && manual == nil {
		if empty {
			return nil, 0, nil, nil
		}
		rows, total, err = plan.run(ctx, p.services().Store)
		return rows, total, nil, err
	}
	cond, scope := n.cond, n.scope
	all, ordering, err := p.scopedSortedEntitiesScoped(ctx, typeName, gated, scope)
	if err != nil {
		return nil, 0, nil, err
	}
	if cond != nil {
		ctx = primeVerdicts(ctx, p.fields(), all)
	}
	all, err = applyViewCondition(ctx, all, cond, suggestionRedactor(p.affordances()), p.services().Store)
	if err != nil {
		return nil, 0, nil, err
	}
	if manual != nil && ordering == nil {
		all = applyManualOrder(ctx, orders, manualID, manual, all)
	}
	total = len(all)
	start := min((page-1)*perPage, total)
	end := min(start+perPage, total)
	return all[start:end], total, ordering, nil
}

// scopedSortedEntities runs the shared list pipeline (ACL read scope, load,
// ?q= intersection, filter[...] and the configured sort) and returns the full
// ordered set before pagination, so a list and its position navigator agree.
//
// The read-scope verdict resolves FIRST: DenyAll returns before the search
// backend runs, so a denied principal cannot probe it through ?q=. Search,
// filters and sort only see the visible subset.
//
// Search failures surface verbatim; ACL query failures wrap errACLListQuery.
func (p *listPipeline) scopedSortedEntities(
	ctx context.Context,
	typeName string,
	query map[string][]string,
) ([]*entityPkg.Entity, error) {
	scopeName, err := queryScopeParam(query)
	if err != nil {
		return nil, err
	}
	state := p.schema()
	scope, err := viewQueryScope(p.queryScopes(), state.Cfg, state.Meta, typeName, scopeName)
	if err != nil {
		return nil, err
	}
	gated, err := p.gate(ctx, state, typeName, query)
	if err != nil {
		return nil, err
	}
	rows, _, err := p.scopedSortedEntitiesScoped(ctx, typeName, gated, scope)
	return rows, err
}

// scopedSortedEntitiesScoped is [listPipeline.scopedSortedEntities] with the query
// scope already resolved, for callers that resolved it to decide something
// else first (listPage uses it to gate the pushdown). It also returns the
// relation ordering a page-scoped request was sorted by, if any; see
// [resolvePageScope]. The query is a [gatedListQuery], so every property it
// filters or sorts on is one the principal sees on every row.
func (p *listPipeline) scopedSortedEntitiesScoped(
	ctx context.Context,
	typeName string,
	query gatedListQuery,
	scope resolvedQueryScope,
) ([]*entityPkg.Entity, *relationOrdering, error) {
	params := query.params
	ctx, err := scope.bind(ctx)
	if err != nil {
		return nil, nil, err
	}
	pageIDs, ordering, pageScoped, err := p.pageScope(ctx, params, typeName)
	if err != nil {
		return nil, nil, err
	}
	if query.reach != nil {
		pageIDs, pageScoped = narrowToReach(pageIDs, pageScoped, query.reach), true
	}
	rqr := readGateFromContext(ctx).ReadQuery(ctx, typeName)
	if readWithheld(ctx, rqr) {
		return []*entityPkg.Entity{}, nil, nil
	}

	searchResult, err := p.queries().freeTextIDsForType(ctx, queryGet(params, "q"), typeName)
	if err != nil {
		return nil, nil, err
	}
	ids, byIDs := narrowingIDs(pageScoped, pageIDs, searchResult)
	req := scopeRequest{
		Type:        typeName,
		Faces:       rqr.Faces,
		ByIDs:       byIDs,
		IDs:         ids,
		Scope:       scope.Scope,
		ScopeProps:  scope.Props,
		ScopeFilter: scope.Filter,
	}
	var entities []*entityPkg.Entity
	if byIDs {
		entities, err = scopedEntities(ctx, p.services(), rqr, req)
	} else {
		entities, err = p.scanType(ctx, rqr, req)
	}
	if err != nil {
		return nil, nil, err
	}

	entities, err = withQueriedRollups(ctx, p.rollups(), typeName, query, entities,
		func(rows []*entityPkg.Entity) ([]*entityPkg.Entity, error) {
			rows, ferr := applyV1Filters(rows, query)
			if ferr != nil {
				return nil, ferr
			}
			rows, ferr = p.applyRelationFilters(ctx, rows, query, typeName)
			if ferr != nil {
				return nil, ferr
			}
			if ordering != nil {
				ordering.sort(rows)
				return rows, nil
			}
			return applyV1Sorting(rows, query, p.schema().Meta), nil
		})
	if err != nil {
		return nil, nil, err
	}
	return entities, ordering, nil
}

// scanType reads every row of req.Type the principal may read, for a list
// request that names no ids: the Go pipeline then filters, sorts and counts
// the whole set. Its cost grows with the type; TKT-5SXPBL1 pushes the shapes
// that still come here into the store.
func (p *listPipeline) scanType(
	ctx context.Context, rqr acl.ReadQueryResult, req scopeRequest,
) ([]*entityPkg.Entity, error) {
	q, errClass, withheld, err := scopedQuery(ctx, rqr, req)
	if err != nil {
		return nil, err
	}
	if withheld {
		return []*entityPkg.Entity{}, nil
	}
	headers, err := scanHeaders(ctx, p.services(), q, errClass, req)
	if err != nil {
		return nil, err
	}
	return headerEntities(headers), nil
}

// applyRelationFilters keeps the entities that match every relation filter of
// gated; its isRelationKey decides which params are relation filters. An entity
// matches when it has an edge of that relation, in the configured direction,
// to a READABLE neighbor whose id (or, deprecated, display title) equals the
// value; see matchRelationFilterMany. An unreadable neighbor never matches, so
// the filter is no existence or title-inference channel.
//
// Only `eq` and `ne` are supported; any other operator is errBadFilter (400),
// never a silent fail-open. It runs over the whole visible set before
// pagination, so the list and _position agree. The pushdown answers an id
// filter in the store instead when it can (listpushdown.go).
func (p *listPipeline) applyRelationFilters(
	ctx context.Context, entities []*entityPkg.Entity, gated gatedListQuery, typeName string,
) ([]*entityPkg.Entity, error) {
	cfg := p.schema().Cfg
	isRelationKey := gated.isRelationKey
	for key, values := range gated.params {
		if !strings.HasPrefix(key, "filter[") || len(values) == 0 {
			continue
		}
		relation, operator, ok := parseRelationFilterKey(key)
		if !ok || isRelationKey == nil || !isRelationKey(relation) {
			continue
		}
		want, err := relationFilterValueOf(key, values)
		if err != nil {
			return nil, err
		}
		if want == "" {
			continue
		}

		switch operator {
		case "eq", "ne":
		default:
			return nil, fmt.Errorf("%w: unknown operator %q in relation filter key %q (supported: eq, ne)",
				errBadFilter, operator, key)
		}

		target, resolved := gated.relationTargets[key]
		if !resolved {
			target = relationTarget{title: want}
		}
		direction, _ := cfg.RelationFilterDirection(typeName, relation)
		matched, merr := matchRelationFilterMany(
			ctx, p.services(), p.visible(), affRedactor{aff: p.affordances}, entities, relation, direction, target)
		if merr != nil {
			return nil, fmt.Errorf("%w: relation filter %q: %w", errListLoad, relation, merr)
		}
		filtered := entities[:0]
		for _, e := range entities {
			if (operator == "eq" && matched[e.ID]) || (operator == "ne" && !matched[e.ID]) {
				filtered = append(filtered, e)
			}
		}
		entities = filtered
	}
	return entities, nil
}

// scopeListCondition narrows a list scope's rows by the `condition:` of the
// list its `list_id` names, as listPage does for the list's own page: after
// the row gate and every filter, so the position and total describe the rows
// the list shows. Rows pass unchanged when no list is named or the list has
// no condition.
func (p *listPipeline) scopeListCondition(
	ctx context.Context, typeName string, query url.Values, rows []*entityPkg.Entity,
) ([]*entityPkg.Entity, error) {
	ctx, n, err := p.narrowing(ctx, p.schema(), typeName, query)
	if err != nil || n.cond == nil {
		return rows, err
	}
	ctx = primeVerdicts(ctx, p.fields(), rows)
	return applyViewCondition(ctx, rows, n.cond, suggestionRedactor(p.affordances()), p.services().Store)
}
