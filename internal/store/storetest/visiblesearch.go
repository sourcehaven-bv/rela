package storetest

import (
	"context"
	"errors"
	"slices"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/search"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// VisibleSearchFactory returns a fresh store, the ungated searcher over
// it (the per-backend parity baseline), and the VisibleSearcher under
// test. The two searchers must share the backend: the suite's
// ordered-subsequence invariant compares the gated stream against the
// ungated stream of the SAME backend — cross-backend rankers
// legitimately differ, so order is never compared across
// implementations, only within one.
type VisibleSearchFactory func(t *testing.T) (store.Store, search.Searcher, search.VisibleSearcher)

// RunVisibleSearchTests is the conformance suite for
// [search.VisibleSearcher]. Any implementation — the generic
// scope-filter wrapper or a backend-native one — must pass it
// (TKT-BA8BSX).
//
// Every case asserts two invariants on top of its specific expectation:
//
//   - no-leak: every yielded hit is in the case's allowed set;
//   - ordered-subsequence: the visible stream equals the ungated
//     stream of the same backend with disallowed hits deleted, order
//     preserved.
//
// Fixture corpora stay far below every backend's candidate bound, so
// truncation never confounds the subsequence check; the post-visibility
// limit contract is pinned separately (LimitPostVisibility).
func RunVisibleSearchTests(t *testing.T, vsf VisibleSearchFactory) {
	t.Helper()

	allow := search.TypeScope{AllowAll: true}

	// ticketsInPRJ1 admits tickets with a direct belongs-to edge to
	// PRJ-1: TKT-1 only.
	ticketsInPRJ1 := func() *store.GraphQuery {
		return &store.GraphQuery{
			EntityType: "ticket",
			HasOutbound: &store.RelationPredicate{
				Endpoints: []string{"PRJ-1"},
				OfTypes:   []string{"belongs-to"},
			},
		}
	}
	// ticketsInPRJ1Transitive additionally walks the ticket's own
	// belongs-to ancestors: TKT-3 → EPIC-1 → PRJ-1 brings TKT-3 in.
	ticketsInPRJ1Transitive := func() *store.GraphQuery {
		return &store.GraphQuery{
			EntityType: "ticket",
			HasOutbound: &store.RelationPredicate{
				Endpoints:            []string{"PRJ-1"},
				OfTypes:              []string{"belongs-to"},
				EntityInheritThrough: []string{"belongs-to"},
				EntityDepth:          3,
			},
		}
	}

	t.Run("AllowAllParity", func(t *testing.T) {
		s, ungated, vs := vsf(t)
		seedVisibleSearchWorld(t, s)
		scope := map[string]search.TypeScope{
			"project": allow, "epic": allow, "ticket": allow,
			"doc": allow, "memo": allow, "ghost": allow,
		}
		q := search.Query{Text: "alpha"}
		base := collectHits(t, ungated.Search(ctx(), q))
		got := collectHits(t, vs.SearchVisible(ctx(), q, scope))
		require.Equal(t, base, got, "fully-open scope must be invisible")
		require.NotEmpty(t, got, "fixture sanity: the query must match")
	})

	t.Run("DenyAllByAbsence", func(t *testing.T) {
		s, ungated, vs := vsf(t)
		seedVisibleSearchWorld(t, s)
		scope := map[string]search.TypeScope{"ticket": allow}
		q := search.Query{Text: "alpha"}
		base := collectHits(t, ungated.Search(ctx(), q))
		got := collectHits(t, vs.SearchVisible(ctx(), q, scope))
		requireVisibleSubset(t, base, got, "TKT-1", "TKT-2", "TKT-3")
		for _, h := range got {
			require.NotEqual(t, "MEMO-1", h.ID, "type absent from scope must never hit")
		}
	})

	t.Run("QueryVerdict", func(t *testing.T) {
		s, ungated, vs := vsf(t)
		seedVisibleSearchWorld(t, s)
		scope := map[string]search.TypeScope{"ticket": {Query: ticketsInPRJ1()}}
		q := search.Query{Text: "alpha"}
		base := collectHits(t, ungated.Search(ctx(), q))
		got := collectHits(t, vs.SearchVisible(ctx(), q, scope))
		requireVisibleSubset(t, base, got, "TKT-1")
		require.Len(t, got, 1)
	})

	t.Run("MixedScope", func(t *testing.T) {
		s, ungated, vs := vsf(t)
		seedVisibleSearchWorld(t, s)
		scope := map[string]search.TypeScope{
			"doc":    allow,
			"ticket": {Query: ticketsInPRJ1()},
			// memo, ghost: denied by absence
		}
		q := search.Query{Text: "alpha"}
		base := collectHits(t, ungated.Search(ctx(), q))
		got := collectHits(t, vs.SearchVisible(ctx(), q, scope))
		requireVisibleSubset(t, base, got, "DOC-1", "TKT-1")
		require.Len(t, got, 2)
	})

	t.Run("LimitPostVisibility", func(t *testing.T) {
		s, ungated, vs := vsf(t)
		seedVisibleSearchWorld(t, s)
		scope := map[string]search.TypeScope{"ticket": {Query: ticketsInPRJ1()}}
		q := search.Query{Text: "alpha"}

		// Precondition: the backend's top-ranked hit for this query is
		// hidden under the scope. If a ranking change ever makes TKT-1
		// the global best match, this case would stop proving anything
		// — fail loudly instead of passing vacuously.
		base := collectHits(t, ungated.Search(ctx(), q))
		require.NotEmpty(t, base)
		require.NotEqual(t, "TKT-1", base[0].ID,
			"fixture precondition: top ungated hit must be hidden under the scope")

		q.Limit = 1
		got := collectHits(t, vs.SearchVisible(ctx(), q, scope))
		require.Len(t, got, 1, "limit must count VISIBLE hits, not candidates")
		require.Equal(t, "TKT-1", got[0].ID,
			"a pre-visibility limit would have spent the budget on hidden hits")
	})

	t.Run("TransitivePredicate", func(t *testing.T) {
		s, ungated, vs := vsf(t)
		seedVisibleSearchWorld(t, s)
		scope := map[string]search.TypeScope{"ticket": {Query: ticketsInPRJ1Transitive()}}
		q := search.Query{Text: "alpha"}
		base := collectHits(t, ungated.Search(ctx(), q))
		got := collectHits(t, vs.SearchVisible(ctx(), q, scope))
		requireVisibleSubset(t, base, got, "TKT-1", "TKT-3")
		require.Len(t, got, 2)
	})

	t.Run("TypesIntersectScope", func(t *testing.T) {
		s, ungated, vs := vsf(t)
		seedVisibleSearchWorld(t, s)
		scope := map[string]search.TypeScope{"doc": allow, "ticket": allow}
		q := search.Query{Text: "alpha", Types: []string{"memo", "doc"}}
		base := collectHits(t, ungated.Search(ctx(), q))
		got := collectHits(t, vs.SearchVisible(ctx(), q, scope))
		// memo is in Types but not in scope; ticket is in scope but
		// not in Types. Only doc survives the intersection.
		requireVisibleSubset(t, base, got, "DOC-1")
		require.Len(t, got, 1)
	})

	t.Run("WildcardAdmitsUnknownTypes", func(t *testing.T) {
		s, ungated, vs := vsf(t)
		seedVisibleSearchWorld(t, s)
		scope := map[string]search.TypeScope{search.WildcardType: allow}
		q := search.Query{Text: "alpha"}
		base := collectHits(t, ungated.Search(ctx(), q))
		got := collectHits(t, vs.SearchVisible(ctx(), q, scope))
		require.Equal(t, base, got, "wildcard-allow must behave as fully open")
		require.True(t, slices.ContainsFunc(got, func(h search.Hit) bool { return h.ID == "GHOST-1" }),
			"fixture sanity: the off-scope-builder type must be in the corpus")
	})

	t.Run("FailClosedWithoutWildcard", func(t *testing.T) {
		s, ungated, vs := vsf(t)
		seedVisibleSearchWorld(t, s)
		scope := map[string]search.TypeScope{"ticket": allow}
		q := search.Query{Text: "alpha"}
		base := collectHits(t, ungated.Search(ctx(), q))
		got := collectHits(t, vs.SearchVisible(ctx(), q, scope))
		requireVisibleSubset(t, base, got, "TKT-1", "TKT-2", "TKT-3")
		for _, h := range got {
			require.NotEqual(t, "GHOST-1", h.ID,
				"a type without a scope entry must fail closed")
		}
	})

	t.Run("EmptyScopeDeniesEverything", func(t *testing.T) {
		s, _, vs := vsf(t)
		seedVisibleSearchWorld(t, s)
		got := collectHits(t, vs.SearchVisible(ctx(), search.Query{Text: "alpha"}, nil))
		require.Empty(t, got, "nil scope must deny everything (fail-closed)")
		got = collectHits(t, vs.SearchVisible(ctx(), search.Query{Text: "alpha"}, map[string]search.TypeScope{}))
		require.Empty(t, got, "empty scope must deny everything (fail-closed)")
	})

	t.Run("WildcardQueryIsInvalid", func(t *testing.T) {
		s, _, vs := vsf(t)
		seedVisibleSearchWorld(t, s)
		scope := map[string]search.TypeScope{
			search.WildcardType: {Query: ticketsInPRJ1()},
		}
		var streamErr error
		for _, err := range vs.SearchVisible(ctx(), search.Query{Text: "alpha"}, scope) {
			if err != nil {
				streamErr = err
				break
			}
		}
		require.Error(t, streamErr, "wildcard entry carrying a GraphQuery must be rejected")
		require.True(t, errors.Is(streamErr, search.ErrScope), "must wrap search.ErrScope, got: %v", streamErr)
	})

	t.Run("FiltersThenLimit", func(t *testing.T) {
		// RR (code review): pins the q.Filters dimension of the
		// contract — residual property filters apply BEFORE the limit,
		// on every backend. A backend that pushes the limit below the
		// filter (e.g. SQL LIMIT before Go-side MatchFilters) would
		// spend the budget on filtered-out hits and under-fill.
		s, ungated, vs := vsf(t)
		seedVisibleSearchWorld(t, s)
		scope := map[string]search.TypeScope{"ticket": allow}
		q := search.Query{
			Text:    "alpha",
			Filters: []search.PropertyFilter{{Property: "status", Value: "open", Op: search.FilterEq}},
		}

		// Expected: the first Limit hits of the OPEN-filtered ungated
		// ticket stream — i.e. the filter consumed no limit budget. A
		// limit-before-filter implementation under-fills whenever the
		// closed ticket outranks an open one (true on the linear and
		// bleve runs; pgstore's SQL-side LIMIT placement is pinned
		// deterministically by its own builder unit test).
		base := collectHits(t, ungated.Search(ctx(), search.Query{Text: "alpha", Types: []string{"ticket"}}))
		require.GreaterOrEqual(t, len(base), 3, "fixture sanity: all three tickets must match")
		var want []string
		for _, h := range base {
			if h.ID != "TKT-1" { // TKT-1 is the closed one
				want = append(want, h.ID)
			}
		}
		q.Limit = 2
		want = want[:2]
		got := collectHits(t, vs.SearchVisible(ctx(), q, scope))
		require.Equal(t, want, hitIDs(got),
			"filters must apply before the limit, in backend order")
	})

	t.Run("InvalidFilterRejectedIdentically", func(t *testing.T) {
		// Both implementations must reject an unsupported (ordered)
		// filter with the same sentinel, before yielding any hit —
		// the Filters dimension of the contract is shared, not
		// implementation-defined.
		s, _, vs := vsf(t)
		seedVisibleSearchWorld(t, s)
		scope := map[string]search.TypeScope{"ticket": allow}
		q := search.Query{
			Text:    "alpha",
			Filters: []search.PropertyFilter{{Property: "status", Value: "1", Op: search.FilterGt}},
		}
		var hits int
		var streamErr error
		for _, err := range vs.SearchVisible(ctx(), q, scope) {
			if err != nil {
				streamErr = err
				break
			}
			hits++
		}
		require.Zero(t, hits, "no hit may precede the validation error")
		require.ErrorIs(t, streamErr, search.ErrOrderedFilterUnsupported)
	})

	t.Run("SortIsIgnored", func(t *testing.T) {
		// The contract says q.Sort is ignored (matching Service.Search);
		// prove no backend accidentally honors it.
		s, _, vs := vsf(t)
		seedVisibleSearchWorld(t, s)
		scope := map[string]search.TypeScope{"ticket": allow}
		plain := collectHits(t, vs.SearchVisible(ctx(), search.Query{Text: "alpha"}, scope))
		sorted := collectHits(t, vs.SearchVisible(ctx(), search.Query{
			Text: "alpha",
			Sort: []search.SortClause{{Field: "title", Direction: search.SortDesc}},
		}, scope))
		require.Equal(t, plain, sorted, "q.Sort must be ignored by every implementation")
	})

	t.Run("EmptyTextListsVisible", func(t *testing.T) {
		s, ungated, vs := vsf(t)
		seedVisibleSearchWorld(t, s)
		scope := map[string]search.TypeScope{"ticket": allow}
		q := search.Query{Text: ""}
		base := collectHits(t, ungated.Search(ctx(), q))
		got := collectHits(t, vs.SearchVisible(ctx(), q, scope))
		// Empty-text listing order is backend-defined; pin membership
		// (sorted) rather than order for this case only.
		requireSameIDSet(t, []string{"TKT-1", "TKT-2", "TKT-3"}, got)
		for _, h := range got {
			require.True(t, hitInSet(base, h.ID), "visible hit missing from ungated baseline")
		}
	})
}

// VisibleFieldSearchFactory returns a fresh store, the ungated searcher, and
// the FieldVisibleSearcher under test. Mirrors [VisibleSearchFactory] but for
// the property-level seam (TKT-GGQ0JT).
type VisibleFieldSearchFactory func(t *testing.T) (store.Store, search.Searcher, search.FieldVisibleSearcher)

// RunVisibleFieldSearchTests is the conformance suite for the property-level
// filter [search.FieldVisibleSearcher.SearchVisibleFields]. It pins the
// match-on-hidden-field oracle closure: a hit whose text matched ONLY fields
// the principal may not see is dropped, while a hit that matched a visible
// field (or id/content, never property-gated) survives.
//
// The fixture adds a `code` property carrying a value ("zeta777") that appears
// in NO title, so a search for it can only match via the property — making the
// hidden-only case unambiguous. Every backend's own matcher must agree with
// the ground-truth (search.MatchTextFields over the visible projection).
func RunVisibleFieldSearchTests(t *testing.T, vsf VisibleFieldSearchFactory) {
	t.Helper()

	allowTickets := map[string]search.TypeScope{"ticket": {AllowAll: true}}

	// hideCode hides the `code` property (search vocabulary "prop:code") for
	// every hit — the common closed-world case.
	hideCode := func(_ context.Context, _ search.Hit, _ *entity.Entity) (map[string]struct{}, error) {
		return map[string]struct{}{search.PropFieldPrefix + "code": {}}, nil
	}

	t.Run("HiddenOnlyMatchDropped", func(t *testing.T) {
		s, _, vs := vsf(t)
		seedVisibleFieldWorld(t, s)
		// "zeta777" lives only in TKT-1's hidden `code` property.
		q := search.Query{Text: "zeta777"}
		got := collectHits(t, vs.SearchVisibleFields(ctx(), q, allowTickets, hideCode))
		require.Empty(t, got, "a hit matching only a hidden field must be dropped (oracle closed)")
	})

	t.Run("HiddenOnlyMatchSurvivesWhenVisible", func(t *testing.T) {
		s, _, vs := vsf(t)
		seedVisibleFieldWorld(t, s)
		q := search.Query{Text: "zeta777"}
		// No hidden fields: the property is visible, so the hit stands.
		got := collectHits(t, vs.SearchVisibleFields(ctx(), q, allowTickets, nil))
		requireSameIDSet(t, []string{"TKT-1"}, got)
	})

	t.Run("VisibleFieldMatchKeptDespiteHiddenField", func(t *testing.T) {
		s, _, vs := vsf(t)
		seedVisibleFieldWorld(t, s)
		// "alpha" matches TKT-1's visible title AND (hypothetically) other
		// fields; with `code` hidden the title match alone keeps the hit.
		q := search.Query{Text: "alpha"}
		got := collectHits(t, vs.SearchVisibleFields(ctx(), q, allowTickets, hideCode))
		require.True(t, hitInSet(got, "TKT-1"),
			"a hit matching a visible field must survive even when another matched field is hidden")
	})

	t.Run("IDMatchSurvivesHiddenProperty", func(t *testing.T) {
		s, _, vs := vsf(t)
		seedVisibleFieldWorld(t, s)
		// The consumer only ever hides declared PROPERTIES ("prop:<name>"), so
		// id/content are never in the hidden set. A hit that matches by id
		// survives even though its `code` property is hidden — id is never
		// property-gated by construction of the hidden set.
		q := search.Query{Text: "TKT-1"}
		got := collectHits(t, vs.SearchVisibleFields(ctx(), q, allowTickets, hideCode))
		require.True(t, hitInSet(got, "TKT-1"), "an id match must survive: id is never property-gated")
	})

	t.Run("FailClosedOnHiddenFuncError", func(t *testing.T) {
		s, _, vs := vsf(t)
		seedVisibleFieldWorld(t, s)
		boom := func(_ context.Context, _ search.Hit, _ *entity.Entity) (map[string]struct{}, error) {
			return nil, errors.New("acl resolution boom")
		}
		q := search.Query{Text: "zeta777"}
		var streamErr error
		hits := 0
		for _, err := range vs.SearchVisibleFields(ctx(), q, allowTickets, boom) {
			if err != nil {
				streamErr = err
				break
			}
			hits++
		}
		require.Zero(t, hits, "no hit may survive a hidden-fields resolution failure")
		require.ErrorIs(t, streamErr, search.ErrScope, "hidden-func errors fail closed via ErrScope")
	})

	t.Run("EntityLevelStillApplies", func(t *testing.T) {
		s, _, vs := vsf(t)
		seedVisibleFieldWorld(t, s)
		// Scope denies tickets entirely: field filter never even runs.
		q := search.Query{Text: "zeta777"}
		got := collectHits(t, vs.SearchVisibleFields(ctx(), q, map[string]search.TypeScope{}, nil))
		require.Empty(t, got, "empty scope denies everything before field filtering")
	})

	t.Run("ContentMatchSurvivesHiddenProperty", func(t *testing.T) {
		s, _, vs := vsf(t)
		e := entity.New("TKT-9", "ticket")
		e.SetString("title", "gamma orbiter")
		e.SetString("code", "zeta777")
		e.Content = "uniquebodytoken operational notes"
		require.NoError(t, s.CreateEntity(ctx(), e), "create TKT-9")
		// The query matches the entity body (content), which is never
		// property-gated, so hiding `code` must not drop it.
		q := search.Query{Text: "uniquebodytoken"}
		got := collectHits(t, vs.SearchVisibleFields(ctx(), q, allowTickets, hideCode))
		require.True(t, hitInSet(got, "TKT-9"),
			"a content match must survive: content is never property-gated")
	})

	t.Run("CallerCannotFalseDropViaIDOrContent", func(t *testing.T) {
		s, _, vs := vsf(t)
		e := entity.New("TKT-9", "ticket")
		e.SetString("title", "delta probe")
		e.Content = "singularcontentword"
		require.NoError(t, s.CreateEntity(ctx(), e), "create TKT-9")
		// A buggy HiddenFieldsFunc that (wrongly) lists id/content must NOT be
		// able to drop a hit that matched id/content — the seam guards the
		// never-gated invariant (MatchHasVisibleField special-cases them).
		hideIDAndContent := func(_ context.Context, _ search.Hit, _ *entity.Entity) (map[string]struct{}, error) {
			return map[string]struct{}{search.FieldID: {}, search.FieldContent: {}}, nil
		}
		q := search.Query{Text: "singularcontentword"}
		got := collectHits(t, vs.SearchVisibleFields(ctx(), q, allowTickets, hideIDAndContent))
		require.True(t, hitInSet(got, "TKT-9"),
			"id/content are never property-gated even if a caller mis-lists them")
	})
}

// seedVisibleFieldWorld builds a small fixture for the property-level suite:
// TKT-1 carries a visible title ("alpha rocket") and a `code` property whose
// value ("zeta777") appears nowhere else, so a search for it matches ONLY via
// that property.
func seedVisibleFieldWorld(t *testing.T, s store.Store) {
	t.Helper()
	e := entity.New("TKT-1", "ticket")
	e.SetString("title", "alpha rocket")
	e.SetString("code", "zeta777")
	require.NoError(t, s.CreateEntity(ctx(), e), "create TKT-1")

	e2 := entity.New("TKT-2", "ticket")
	e2.SetString("title", "beta lander")
	require.NoError(t, s.CreateEntity(ctx(), e2), "create TKT-2")
}

// seedVisibleSearchWorld builds the shared fixture graph:
//
//	PRJ-1 "Apollo program"      PRJ-2 "Artemis program"
//	  ▲ belongs-to                ▲ belongs-to
//	EPIC-1 "Epic One" ◄─ TKT-3   TKT-2 "alpha lander"
//	  ▲ belongs-to (TKT-3 "alpha probe")
//	TKT-1 "alpha rocket"
//	DOC-1 "alpha handbook"  MEMO-1 "alpha secret"  GHOST-1 "alpha ghost"
//
// "alpha" matches every ticket, the doc, the memo, and the ghost. The
// ghost's type never appears in any non-wildcard scope, standing in
// for an entity type the scope builder doesn't know about.
func seedVisibleSearchWorld(t *testing.T, s store.Store) {
	t.Helper()
	seed := func(id, typ, title string) {
		e := entity.New(id, typ)
		e.SetString("title", title)
		require.NoError(t, s.CreateEntity(ctx(), e), "create %s", id)
	}
	seed("PRJ-1", "project", "Apollo program")
	seed("PRJ-2", "project", "Artemis program")
	seed("EPIC-1", "epic", "Epic One")
	seedStatus := func(id, typ, title, status string) {
		e := entity.New(id, typ)
		e.SetString("title", title)
		e.SetString("status", status)
		require.NoError(t, s.CreateEntity(ctx(), e), "create %s", id)
	}
	seedStatus("TKT-1", "ticket", "alpha rocket", "closed")
	seedStatus("TKT-2", "ticket", "alpha lander", "open")
	seedStatus("TKT-3", "ticket", "alpha probe", "open")
	seed("DOC-1", "doc", "alpha handbook")
	seed("MEMO-1", "memo", "alpha secret")
	seed("GHOST-1", "ghost", "alpha ghost")
	mustRel(t, s, "TKT-1", "belongs-to", "PRJ-1")
	mustRel(t, s, "TKT-2", "belongs-to", "PRJ-2")
	mustRel(t, s, "TKT-3", "belongs-to", "EPIC-1")
	mustRel(t, s, "EPIC-1", "belongs-to", "PRJ-1")
}

// requireVisibleSubset asserts the suite's two cross-case invariants:
// no hit outside allowedIDs (no-leak), and the visible stream equal to
// the same-backend ungated stream with disallowed hits deleted
// (ordered-subsequence).
func requireVisibleSubset(t *testing.T, ungated, visible []search.Hit, allowedIDs ...string) {
	t.Helper()
	allowed := make(map[string]bool, len(allowedIDs))
	for _, id := range allowedIDs {
		allowed[id] = true
	}
	for _, h := range visible {
		require.True(t, allowed[h.ID], "no-leak violated: hit %s (%s) outside allowed set", h.ID, h.Type)
	}
	want := make([]string, 0, len(visible))
	for _, h := range ungated {
		if allowed[h.ID] {
			want = append(want, h.ID)
		}
	}
	require.Equal(t, want, hitIDs(visible),
		"ordered-subsequence violated: visible stream must equal the ungated stream minus hidden hits")
}

// requireSameIDSet asserts hit IDs equal want, ignoring order.
func requireSameIDSet(t *testing.T, want []string, hits []search.Hit) {
	t.Helper()
	got := hitIDs(hits)
	slices.Sort(got)
	want = slices.Clone(want)
	slices.Sort(want)
	require.Equal(t, want, got)
}

func hitIDs(hits []search.Hit) []string {
	ids := make([]string, 0, len(hits))
	for _, h := range hits {
		ids = append(ids, h.ID)
	}
	return ids
}

func hitInSet(hits []search.Hit, id string) bool {
	return slices.ContainsFunc(hits, func(h search.Hit) bool { return h.ID == id })
}
