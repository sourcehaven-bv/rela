package dataentry

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/filter"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/search"
	"github.com/Sourcehaven-BV/rela/internal/search/searchparser"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/visibility"
)

// queryService owns the search-query pipeline: the `/_search` and
// `_position`-scope entry point (executeQuery), its free-text branch, the
// list endpoint's `?q=` id-set helper, and the metamodel-aware sort and
// property-filter passes those share. Extracted from App (TKT-SJ0LRS, part
// of the TKT-R68TV8 decomposition arc).
//
// Consolidating them here is the point of the extraction: the read handlers,
// the next-action engine (nextaction.go) and the `_position` scope resolver
// (scope.go) now share ONE collaborator instead of each reaching back into
// App for the same closures.
//
// Collaborator rationale differs from the sibling handlers in one respect
// that is load-bearing. The ACL-scoped searcher and the field resolver
// are held as CLOSURES rather than values, because tests reassign
// `app.visibleSearcher` (see rebindVisibleSearcher) and `app.fieldResolver`
// after construction; a captured value would silently keep the searcher from
// construction time and a test injecting a recording/denying searcher would
// exercise the wrong one.
//
// The searcher seam is deliberately [search.VisibleSearcher], never a plain
// search.Searcher: executeQuery's free-text branch must only ever see hits
// the request principal may read (TKT-BA8BSX). This type also holds NO
// store.Store FIELD — it never carries an independent store handle, so a
// read cannot bypass the request-scoped bundle. The store reads it does
// perform arrive through the Services bundle passed in per call, whose
// gating is applied by the free functions in helpers.go
// (visibleListByTypes / visibleEntitiesOfType) against the resolved scope.
type queryService struct {
	// schema is the reloadable snapshot; the sort and property-filter passes
	// resolve entity definitions against it per call so a metamodel reload
	// propagates.
	schema func() *Schema
	// services returns the read bundle (store + searcher + metamodel).
	services func() Services
	// visibleSearcher is the ACL-scoped search seam. Late-bound: tests
	// reassign App.visibleSearcher after construction.
	visibleSearcher func() search.VisibleSearcher
	// fields supplies each type's search field scope, so a hit that matched
	// only a `visible:`-hidden property is dropped. It is the App's field
	// resolver ([FieldVerdictResolver.SearchFieldScope]), late-bound for the
	// same reason as visibleSearcher.
	fields func() visibility.FieldScoper
}

// newQueryService wires the leaf over the App's current collaborators.
// Called from NewApp and from the test rebind path.
func newQueryService(app *App) *queryService {
	return &queryService{
		schema:          app.State,
		services:        app.Services,
		visibleSearcher: func() search.VisibleSearcher { return app.visibleSearcher },
		fields:          func() visibility.FieldScoper { return app.fieldResolver },
	}
}

// executeQuery parses a search query and returns all matching entities, with
// the search page's syntax. It runs under the ctx principal's read scope, so
// every consumer inherits the gate. The scope resolves first, and a fully
// denied scope returns before any backend work, so latency reveals nothing.
// maxSearchResults counts rows that survived visibility and property
// filters. Scope failures wrap errACLListQuery, store-load failures wrap
// errListLoad, and search-backend failures pass through.
func (q *queryService) executeQuery(ctx context.Context, query string) ([]*entity.Entity, error) {
	return q.executeQueryWith(ctx, query, queryOptions{})
}

// queryOptions are executeQueryWith's optional inputs. The zero value is
// executeQuery.
type queryOptions struct {
	// prefilters are extra store predicates ANDed into the no-free-text
	// branch's pushdown; see executeQueryWith.
	prefilters []store.PropPredicate
	// limit is how many results the caller will keep, or 0 for all. It
	// only lowers the free-text branch's relevance cap, and only when no
	// later pass could drop or reorder a hit inside it; see
	// freeTextLimit. The caller still truncates.
	limit int
	// exactType is the one type the caller keeps afterwards (the
	// `type` parameter of /_search), or "" for none.
	exactType string
	// firstN is how many leading results the caller keeps, or 0 for all.
	// Unlike limit it bounds the branch WITHOUT free text: that branch then
	// reads the store in windows until it has firstN results (see
	// firstNByTypes). Results past firstN may or may not be returned.
	firstN int
	// offset is how many leading results searchPage skips: the rows of the
	// pages before the one /_search serves. Only searchPage reads it.
	offset int
}

// freeTextLimit is the relevance cap for a free-text search. It is
// maxSearchResults unless the caller keeps only opts.limit rows and every
// returned row is kept, in order: no property filter, no `sort:` clause, and
// no type the caller drops. A caller keeping more than maxSearchResults rows
// gets one hit past the cap, so it can tell a cut result from one that ends
// there.
func freeTextLimit(sq *searchparser.SearchQuery, opts queryOptions) int {
	if opts.limit > maxSearchResults {
		return maxSearchResults + 1
	}
	if opts.limit <= 0 || opts.limit == maxSearchResults ||
		len(sq.PropertyFilters) > 0 || sq.HasSort() {

		return maxSearchResults
	}
	if opts.exactType != "" {
		if len(sq.EntityTypes) == 0 {
			return maxSearchResults
		}
		for _, t := range sq.EntityTypes {
			if t != opts.exactType {
				return maxSearchResults
			}
		}
	}
	return opts.limit
}

// executeQueryWith is executeQuery with options: extra store predicates ANDed
// into the no-free-text branch's pushdown, and the caller's limit.
// opts.prefilters may only remove rows the caller's own authoritative filter
// would also remove. They are ignored on the free-text branch, which is
// relevance-capped, so no caller with a condition may pass free text.
func (q *queryService) executeQueryWith(
	ctx context.Context, query string, opts queryOptions,
) ([]*entity.Entity, error) {
	results, _, err := q.executeQueryCut(ctx, query, opts)
	return results, err
}

// executeQueryCut is executeQueryWith that also reports whether the read was
// cut short, so rows past the results were not read: by opts.firstN, or,
// for free text, by the relevance cap (the search returned more than
// maxSearchResults hits, which freeTextLimit allows only for a caller that
// asks past the cap).
func (q *queryService) executeQueryCut(
	ctx context.Context, query string, opts queryOptions,
) (results []*entity.Entity, cut bool, err error) {
	sq := parseSearchQuery(ctx, query)
	if sq.IsEmpty() {
		return nil, false, nil
	}
	if worldFromContext(ctx).blocksAllReads() {
		return []*entity.Entity{}, false, nil
	}

	svc := q.services()
	typeNames := make([]string, 0, len(svc.Meta.Entities))
	for name := range svc.Meta.Entities {
		typeNames = append(typeNames, name)
	}
	slices.Sort(typeNames)
	scope := readGateFromContext(ctx).SearchScope(ctx, typeNames)
	if len(scope) == 0 {
		return []*entity.Entity{}, false, nil
	}

	var candidates []*entity.Entity
	if sq.HasFreeText() {
		candidates, err = q.runVisibleFreeTextSearch(ctx, svc, sq, scope, freeTextLimit(sq, opts))
		if cut = len(candidates) > maxSearchResults; cut {
			candidates = candidates[:maxSearchResults]
		}
	} else {
		pushed := pushdownPrefilters(sq.PropertyFilters, svc.Meta, sq.EntityTypes)
		pushed = append(pushed, opts.prefilters...)
		if opts.firstN > 0 && !sq.HasSort() && len(sq.EntityTypes) > 0 {
			return q.firstNByTypes(ctx, svc, sq, scope, pushed, opts.firstN)
		}
		candidates, err = visibleListByTypes(ctx, svc, sq.EntityTypes, scope, pushed)
	}
	if err != nil {
		return nil, false, err
	}

	results = make([]*entity.Entity, 0, len(candidates))
	for _, e := range candidates {
		if !q.matchesPropertyFilters(e, sq.PropertyFilters) {
			continue
		}
		results = append(results, e)
		if sq.HasFreeText() && len(results) >= maxSearchResults {
			break
		}
	}

	if sq.HasSort() {
		q.sortEntitiesMulti(results, sq.SortClauses)
	}

	return results, cut, nil
}

// firstNByTypes is the branch of executeQueryWith without free text for a
// caller that keeps only the first n results. It returns the same leading
// rows, in the same order, as visibleListByTypes plus the property filters,
// but reads the store in growing windows and stops once it has n. cut reports
// that it stopped early. The row gate and world apply in every window's store
// query. Not for a query with a `sort:` clause.
func (q *queryService) firstNByTypes(
	ctx context.Context, svc Services, sq *searchparser.SearchQuery, scope map[string]search.TypeScope,
	pushed []store.PropPredicate, n int,
) (results []*entity.Entity, cut bool, err error) {
	results = make([]*entity.Entity, 0, n)
	seen := make(map[string]struct{}, n)
	for _, typ := range sq.EntityTypes {
		ts, ok := search.ResolveTypeScope(scope, typ)
		if !ok {
			continue
		}
		offset, window := 0, n-len(results)
		for reads := 1; ; reads++ {
			var rows []*entity.Entity
			if reads > maxQueryWindows {
				window = 0
				rows, err = visibleEntitiesOfType(ctx, svc, typ, ts, pushed, offset)
			} else {
				rows, err = visibleEntitiesOfTypeWindow(ctx, svc, typ, ts, pushed, window, offset)
			}
			if err != nil {
				return nil, false, err
			}
			for _, e := range rows {
				if _, dup := seen[e.ID]; dup || !q.matchesPropertyFilters(e, sq.PropertyFilters) {
					continue
				}
				seen[e.ID] = struct{}{}
				results = append(results, e)
				if len(results) == n {
					return results, true, nil
				}
			}
			if window == 0 || len(rows) < window {
				break
			}
			offset, window = offset+window, 4*window
		}
	}
	return results, false, nil
}

// maxQueryWindows is how many growing windows firstNByTypes reads of one
// type before it reads the rest of the type at once.
const maxQueryWindows = 3

// runVisibleFreeTextSearch is executeQuery's free-text branch: the
// same phrase re-quoting as runFreeTextSearchE, routed through the
// ACL-scoped searcher. The backend-side limit is only set when no
// property filters remain — with Go-side filters pending, truncation
// happens in executeQuery after them, or the filter gap would re-open
// the starvation the post-visibility limit closes. capN is the relevance
// cap (see freeTextLimit).
func (q *queryService) runVisibleFreeTextSearch(
	ctx context.Context, svc Services, sq *searchparser.SearchQuery, scope map[string]search.TypeScope, capN int,
) ([]*entity.Entity, error) {
	parts := make([]string, 0, len(sq.FreeTextWords)+len(sq.FreeTextPhrases))
	parts = append(parts, sq.FreeTextWords...)
	for _, p := range sq.FreeTextPhrases {
		parts = append(parts, `"`+p+`"`)
	}
	limit := 0
	if len(sq.PropertyFilters) == 0 {
		limit = capN
	}
	sQuery := search.Query{
		Text:  strings.Join(parts, " "),
		Types: sq.EntityTypes,
		Limit: limit,
		World: worldScopeFrom(ctx),
	}
	var hits []search.Hit
	var types []string
	if svc.Meta != nil {
		types = svc.Meta.EntityTypes()
	}
	stream := visibility.SearchScoped(ctx, q.visibleSearcher(), ctxRowGate{}, q.fields(), sQuery, scope, types)
	for hit, err := range stream {
		if err != nil {
			if errors.Is(err, search.ErrScope) {
				return nil, fmt.Errorf("%w: %w", errACLListQuery, err)
			}
			return nil, fmt.Errorf("free-text search: %w", err)
		}
		hits = append(hits, hit)
	}
	loaded, err := loadHitHeaders(ctx, svc.Store, hits)
	if err != nil {
		return nil, fmt.Errorf("free-text search: %w", err)
	}
	out := make([]*entity.Entity, 0, len(hits))
	for _, hit := range hits {
		if e, ok := loaded[entity.Ref{ID: hit.ID, Face: hit.Face}]; ok {
			out = append(out, e)
		}
	}
	return out, nil
}

// freeTextIDsForType runs a free-text search constrained to the given type
// and returns the matching ids. A query without free-text words returns
// HasFilter=false; any `type:` token is ignored. Searcher errors are
// returned, never an empty match. The search is world-scoped off ctx, which
// makes `?q=` safe to combine with `?world=`: text matches only the face the
// world resolves, and an excluded entity cannot appear.
func (q *queryService) freeTextIDsForType(
	ctx context.Context, query, typeName string,
) (freeTextIDsForTypeResult, error) {
	query = strings.TrimSpace(query)
	if query == "" {
		return freeTextIDsForTypeResult{}, nil
	}
	sq := parseSearchQuery(ctx, query)
	if sq.IsEmpty() || !sq.HasFreeText() {
		return freeTextIDsForTypeResult{}, nil
	}
	sq.EntityTypes = []string{typeName}

	if worldFromContext(ctx).blocksAllReads() {
		return freeTextIDsForTypeResult{IDs: nil, HasFilter: true}, nil
	}

	scope := readGateFromContext(ctx).SearchScope(ctx, sq.EntityTypes)
	if len(scope) == 0 {
		return freeTextIDsForTypeResult{IDs: map[string]struct{}{}, HasFilter: true}, nil
	}
	ids, err := freeTextIDs(ctx, q.visibleSearcher(), q.fields(), sq, scope,
		worldScopeFrom(ctx), maxSearchResults)
	if err != nil {
		return freeTextIDsForTypeResult{}, err
	}
	return freeTextIDsForTypeResult{IDs: ids, HasFilter: true}, nil
}

// sortEntitiesMulti orders search results by the shared query-sort rule, so a
// `sort:` clause in the search bar and the same key on a list page agree
// (TKT-9OFGH4).
//
// [filter.QuerySort] rather than [filter.SortMulti]: the two differed on
// strings, dates, ids and the meaning of descending, and a search result and a
// list page showing the same rows in different orders is the defect. The rule
// is whatever the store can express, since only that can be pushed down.
func (q *queryService) sortEntitiesMulti(entities []*entity.Entity, specs []filter.SortSpec) {
	if len(specs) == 0 {
		return
	}
	s := q.schema()
	entityDefs := make(map[string]*metamodel.EntityDef)
	for _, e := range entities {
		if _, ok := entityDefs[e.Type]; !ok {
			if def, ok := s.Meta.GetEntityDef(e.Type); ok {
				entityDefs[e.Type] = def
			}
		}
	}
	filter.QuerySortApply(filter.NewQuerySort(specs, entityDefs, s.Meta), entities, entityRecord, specs)
}

// matchesPropertyFilters checks whether an entity matches the given property
// filters. Returns true if no filters are specified or all filters match.
func (q *queryService) matchesPropertyFilters(e *entity.Entity, filters []*filter.Filter) bool {
	if len(filters) == 0 {
		return true
	}
	s := q.schema()
	entDef, ok := s.Meta.GetEntityDef(e.Type)
	if !ok {
		return false
	}
	matched, err := filter.MatchAll(entityRecord(e), filters, entDef, s.Meta)
	return err == nil && matched
}

// loadHitHeaders reads the content-free rows the hits name, grouped by
// face: default-face hits in one IDs query, each other face in one
// AtFaces query at that face on the way out. Returns only rows
// that exist, as content-free entities (see rowcontent.go).
func loadHitHeaders(ctx context.Context, st store.Store, hits []search.Hit) (map[entity.Ref]*entity.Entity, error) {
	byFace := make(map[entity.Face][]string)
	seen := make(map[entity.Ref]struct{}, len(hits))
	for _, h := range hits {
		k := entity.Ref{ID: h.ID, Face: h.Face}
		if _, dup := seen[k]; dup {
			continue
		}
		seen[k] = struct{}{}
		byFace[h.Face] = append(byFace[h.Face], h.ID)
	}
	out := make(map[entity.Ref]*entity.Entity, len(seen))
	for face, ids := range byFace {
		q := store.EntityQuery{IDs: ids, Faces: store.AtFaces(face)}
		for h, err := range store.ListEntityHeaders(ctx, st, q) {
			if err != nil {
				return nil, err
			}
			out[entity.Ref{ID: h.ID, Face: h.Face}] = headerEntity(h)
		}
	}
	return out, nil
}

// searchPage is the read behind /_search: one page of the query's result,
// limited to opts.exactType when set, and the total number of matches. The
// page holds at most opts.limit rows (maxSearchResults by default) starting
// opts.offset readable rows in, so hidden rows never shift a page. A query
// [planSearch] can push down counts in the store; others are cut here, and
// for free text the total is a lower bound under the relevance cap. capped
// reports that free text matched more than maxSearchResults hits.
func (q *queryService) searchPage(
	ctx context.Context, query string, opts queryOptions,
) (rows []*entity.Entity, total int, capped bool, err error) {
	size := opts.limit
	if size <= 0 || size > maxSearchResults {
		size = maxSearchResults
	}
	offset := max(opts.offset, 0)
	svc := q.services()
	if plan, scope, ok := structuredPlan(ctx, svc, query, opts.exactType); ok {
		if len(scope) == 0 {
			return []*entity.Entity{}, 0, false, nil
		}
		rows, total, err = plan.page(ctx, svc, scope, size, offset)
		return rows, total, false, err
	}

	read := opts
	read.limit = offset + size + 1
	rows, capped, err = q.executeQueryCut(ctx, query, read)
	if err != nil {
		return nil, 0, false, err
	}
	if opts.exactType != "" {
		rows = slices.DeleteFunc(rows, func(e *entity.Entity) bool { return e.Type != opts.exactType })
	}
	total = len(rows)
	rows = rows[min(offset, len(rows)):min(offset+size, len(rows))]
	return rows, total, capped, nil
}

// searchPosition answers where id sits in a search scope's result (the
// query, then exactType when set) from the store, when the query is a
// structured one over a single type that [planSearch] accepts. handled=false
// means "take the Go path" (resolveScope).
func (q *queryService) searchPosition(
	ctx context.Context, query, exactType, id string,
) (pos store.Position, found, handled bool, err error) {
	svc := q.services()
	plan, scope, ok := structuredPlan(ctx, svc, query, exactType)
	if !ok || len(plan.types) > 1 {
		return store.Position{}, false, false, nil
	}
	if len(scope) == 0 {
		return store.Position{}, false, true, nil
	}
	return plan.position(ctx, svc, scope, id)
}

// structuredPlan parses query and plans it for the store, with the read
// scope the plan runs under. ok=false means the query is free text, empty,
// or one the store cannot answer exactly. A denied world, or a plan that
// matches nothing, is ok with an empty scope: nothing is read, which is what
// executeQueryWith answers too.
func structuredPlan(
	ctx context.Context, svc Services, query, exactType string,
) (plan searchPlan, scope map[string]search.TypeScope, ok bool) {
	sq := parseSearchQuery(ctx, query)
	if sq.IsEmpty() {
		return searchPlan{}, nil, false
	}
	if plan, ok = planSearch(svc.Meta, sq, exactType); !ok {
		return searchPlan{}, nil, false
	}
	if worldFromContext(ctx).blocksAllReads() || len(plan.types) == 0 {
		return plan, nil, true
	}
	return plan, readGateFromContext(ctx).SearchScope(ctx, plan.types), true
}

// parseSearchQuery parses a search query and resolves the date variables of
// its property filters (`prop:due<$today`, `$now`, offsets) at the
// request's read time. Every search path parses through here, so `/_search`,
// a dashboard `query:` card, a nav search scope and a next-action query all
// read one `$today` per request.
func parseSearchQuery(ctx context.Context, query string) *searchparser.SearchQuery {
	sq := searchparser.ParseQuery(query)
	sq.PropertyFilters = filter.BindDateVariables(sq.PropertyFilters, requestReadTime(ctx))
	return sq
}
