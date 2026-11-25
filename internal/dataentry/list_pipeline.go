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
	// FIRST, before anything reads a row: the field gate's answer depends on
	// the policy and the caller's roles only, so the refusal must not vary
	// with what the type holds.
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
	// AFTER the ACL scope and every filter, BEFORE paging and the count: the
	// condition narrows the population the page and total describe, so
	// applying it later would page one set and count another.
	// Prime only when a condition will judge every row: otherwise the page
	// is primed below, and priming the whole population here is wasted.
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

// scopedSortedEntities runs the shared list pipeline — ACL read scope,
// load, free-text intersection (?q=), structured filters (filter[...]),
// then the configured sort — and returns the fully ordered result set
// *before* pagination. Both handleV1ListEntities and
// handleV1EntityPosition call this, so a list and its scope navigator
// are guaranteed to observe identical ordering (under ACL both operate
// on the same visible subset — hidden entities don't occupy ordinals).
//
// ACL ordering contract (TKT-VMD8, RR-X56H + RR-3IO2): the read-scope
// verdict resolves FIRST. DenyAll returns before the search backend,
// filters, or sort run — a denied principal must not be able to probe
// backend latency through ?q=. A composed Query loads the visible
// subset via store.GraphQuery; search/filter/sort then operate on that
// filtered slice only (search-after-ACL). AllowAll keeps the pre-ACL
// load path byte-identical.
//
// Errors: free-text search failures surface verbatim (HTTP 500
// search_failed at the call site); ACL query failures are wrapped in
// errACLListQuery so call sites map them via writeGateError. Everything
// else degrades to an empty/whole set as the list endpoint always did.
func (p *listPipeline) scopedSortedEntities(
	ctx context.Context,
	typeName string,
	query map[string][]string,
) ([]*entityPkg.Entity, error) {
	// Resolves the scope from the request's ?query_scope= (absent = the
	// type's default), so every caller of this function is scoped too:
	// export mirrors the list it names, and _position navigates the set the
	// list actually showed.
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
	// Stamp the scope's request-scoped state ONCE, before any row is
	// evaluated — a scope reading current_user resolves it here, not per row.
	ctx, err := scope.bind(ctx)
	if err != nil {
		return nil, nil, err
	}
	// The page scope resolves BEFORE the load, so an anchor the principal
	// cannot see answers the uniform 404 whatever the type's verdict is.
	pageIDs, ordering, pageScoped, err := p.pageScope(ctx, params, typeName)
	if err != nil {
		return nil, nil, err
	}
	if query.reach != nil {
		pageIDs, pageScoped = narrowToReach(pageIDs, pageScoped, query.reach), true
	}
	// The verdict switch, the world scope, the face allowlist and the query
	// scope all live in scopedread.go (scopedQuery, scopedHeaders and
	// scanHeaders); see the scopedHeaders doc for why they must not be
	// re-implemented per handler.
	rqr := readGateFromContext(ctx).ReadQuery(ctx, typeName)
	if readWithheld(ctx, rqr) {
		// Return BEFORE the free-text search: a principal who may read
		// nothing must not be able to probe the search backend's latency or
		// induce load through ?q= (RR-X56H).
		return []*entityPkg.Entity{}, nil, nil
	}

	// Free-text search: the hits narrow the read when ?q=... is present.
	// Full-text scores are discarded — the list's configured sort wins over
	// relevance ranking, same approach SearchView uses for filtering.
	// Backend errors surface as HTTP 500 rather than rendering an empty list
	// and pretending the search succeeded.
	searchResult, err := p.queries().freeTextIDsForType(ctx, queryGet(params, "q"), typeName)
	if err != nil {
		return nil, nil, err
	}
	// The page scope and the hits each name the rows the list may hold, so
	// the read asks for those ids only rather than scanning the type
	// (TKT-F2EJQY8). Each id still passes the verdict, the world, the faces
	// and the query scope in scopedHeaders, so neither can surface a row the
	// full read would not admit.
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

	// Each filter[<key>] param was classified as property vs relation ONCE,
	// by the gate, so both passes agree on routing with the gate itself.
	entities, err = withQueriedRollups(ctx, p.rollups(), typeName, query, entities,
		func(rows []*entityPkg.Entity) ([]*entityPkg.Entity, error) {
			rows, ferr := applyV1Filters(rows, query)
			if ferr != nil {
				return nil, ferr
			}
			rows, ferr = p.applyRelationFilters(ctx, rows, params, typeName, query.isRelationKey)
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
		// Unreachable after the readWithheld check in the caller (same ctx
		// and verdict); kept so the empty shape does not depend on that check.
		return []*entityPkg.Entity{}, nil
	}
	headers, err := scanHeaders(ctx, p.services(), q, errClass, req)
	if err != nil {
		return nil, err
	}
	return headerEntities(headers), nil
}

// applyRelationFilters keeps only the entities that match every relation
// filter present in the query. isRelationKey (the config-backed classifier
// built in scopedSortedEntities) decides which `filter[<key>]` params are
// relation filters; property filters are handled by applyV1Filters. Matching is
// direction-aware: the FilterControl config on a list or kanban of this
// entity type supplies the direction (default outgoing; precedence in
// dataentryconfig.Config.RelationFilterDirection), and an entity matches iff
// it has an edge of that (relation, direction) to a READABLE neighbor whose
// display title equals the requested value.
//
// Operators: only `eq` (the bare `filter[<rel>]` form) and `ne`
// (`filter[<rel>][ne]`) are supported. Any other operator segment is rejected
// with errBadFilter → HTTP 400, matching applyV1Filters. This supersedes the
// RR-6RF60V drop-to-zero-rows behavior: both were fail-closed, but zero rows
// reads as "no data" while a 400 names the broken operator to the caller.
// Never fail open.
//
// ACL (RR-HK1XNO): neighbor titles are resolved through the read gate, so a
// filter value matching a neighbor the caller cannot read does NOT include the
// edged rows (no title-match inference channel). Neighbors are gated in one
// batch per relation param via matchRelationFilter → visibleNeighborTitles.
// NOTE: when the sibling helper App.visibleRelationIDs (TKT-ODHV2D) merges,
// this should converge on it; today it uses the same readGate.ReadableFacesMany
// batching pattern inline.
//
// Cost (RR-38K7K9): this runs over the entire type's visible set BEFORE
// pagination, doing ListRelations + a batched GetEntity per row. It is
// therefore O(N·edges) per request, uncached, where N is the unpaginated
// visible row count — acceptable for the current dataset sizes but NOT a
// store-side pushdown. A future optimization is to push the relation-title
// predicate into the store query. Each row short-circuits: matchRelationFilter
// stops scanning a row's edges as soon as one matches.
//
// This runs in scopedSortedEntities so BOTH the list handler and scope nav
// (_position, via resolveScope) observe identical relation filtering — they
// share this pipeline. Property filters are untouched (TKT-5U7QBR).
func (p *listPipeline) applyRelationFilters(
	ctx context.Context, entities []*entityPkg.Entity, query map[string][]string, typeName string,
	isRelationKey func(string) bool,
) ([]*entityPkg.Entity, error) {
	cfg := p.schema().Cfg
	for key, values := range query {
		if !strings.HasPrefix(key, "filter[") || len(values) == 0 {
			continue
		}
		relation, operator, ok := parseRelationFilterKey(key)
		if !ok || !isRelationKey(relation) {
			continue // malformed, or a property filter handled elsewhere
		}
		want := values[len(values)-1]
		if want == "" {
			continue // empty control value = no constraint
		}

		// Reject any operator we don't support for relations. Never fall
		// through and leave the set unfiltered (that would be the fail-open
		// bug RR-6RF60V); see the doc comment on the 400-over-zero-rows choice.
		switch operator {
		case "eq", "ne":
			// supported below
		default:
			return nil, fmt.Errorf("%w: unknown operator %q in relation filter key %q (supported: eq, ne)",
				errBadFilter, operator, key)
		}

		direction, _ := cfg.RelationFilterDirection(typeName, relation)
		matched, merr := matchRelationFilterMany(
			ctx, p.services(), p.visible(), affRedactor{aff: p.affordances}, entities, relation, direction, want)
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
