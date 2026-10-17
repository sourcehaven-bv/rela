package storetest

import (
	"errors"
	"fmt"
	"math/rand/v2"
	"slices"
	"strings"
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
			Faces: store.InWorld(store.TrivialScope()),
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
			Faces: store.InWorld(store.TrivialScope()),
		}
	}

	t.Run("AllowAllParity", func(t *testing.T) {
		s, ungated, vs := vsf(t)
		seedVisibleSearchWorld(t, s)
		scope := map[string]search.TypeScope{
			"project": allow, "epic": allow, "ticket": allow,
			"doc": allow, "memo": allow, "ghost": allow,
		}
		q := search.Query{Text: "alpha", World: store.TrivialScope()}
		base := collectHits(t, ungated.Search(ctx(), q))
		got := collectHits(t, vs.SearchVisible(ctx(), q, scope))
		require.Equal(t, base, got, "fully-open scope must be invisible")
		require.NotEmpty(t, got, "fixture sanity: the query must match")
	})

	t.Run("DenyAllByAbsence", func(t *testing.T) {
		s, ungated, vs := vsf(t)
		seedVisibleSearchWorld(t, s)
		scope := map[string]search.TypeScope{"ticket": allow}
		q := search.Query{Text: "alpha", World: store.TrivialScope()}
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
		q := search.Query{Text: "alpha", World: store.TrivialScope()}
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
		q := search.Query{Text: "alpha", World: store.TrivialScope()}
		base := collectHits(t, ungated.Search(ctx(), q))
		got := collectHits(t, vs.SearchVisible(ctx(), q, scope))
		requireVisibleSubset(t, base, got, "DOC-1", "TKT-1")
		require.Len(t, got, 2)
	})

	t.Run("LimitPostVisibility", func(t *testing.T) {
		s, ungated, vs := vsf(t)
		seedVisibleSearchWorld(t, s)
		scope := map[string]search.TypeScope{"ticket": {Query: ticketsInPRJ1()}}
		q := search.Query{Text: "alpha", World: store.TrivialScope()}

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
		q := search.Query{Text: "alpha", World: store.TrivialScope()}
		base := collectHits(t, ungated.Search(ctx(), q))
		got := collectHits(t, vs.SearchVisible(ctx(), q, scope))
		requireVisibleSubset(t, base, got, "TKT-1", "TKT-3")
		require.Len(t, got, 2)
	})

	t.Run("TypesIntersectScope", func(t *testing.T) {
		s, ungated, vs := vsf(t)
		seedVisibleSearchWorld(t, s)
		scope := map[string]search.TypeScope{"doc": allow, "ticket": allow}
		q := search.Query{Text: "alpha", Types: []string{"memo", "doc"}, World: store.TrivialScope()}
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
		q := search.Query{Text: "alpha", World: store.TrivialScope()}
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
		q := search.Query{Text: "alpha", World: store.TrivialScope()}
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
		got := collectHits(t, vs.SearchVisible(ctx(), search.Query{Text: "alpha", World: store.TrivialScope()}, nil))
		require.Empty(t, got, "nil scope must deny everything (fail-closed)")
		got = collectHits(t, vs.SearchVisible(ctx(), search.Query{Text: "alpha", World: store.TrivialScope()}, map[string]search.TypeScope{}))
		require.Empty(t, got, "empty scope must deny everything (fail-closed)")
	})

	// An unset world is refused rather than read as the trivial world
	// (TKT-7IZHP0 design A4).
	t.Run("UnsetWorldIsInvalid", func(t *testing.T) {
		s, _, vs := vsf(t)
		seedVisibleSearchWorld(t, s)
		scope := map[string]search.TypeScope{"ticket": allow}
		var streamErr error
		for _, err := range vs.SearchVisible(ctx(), search.Query{Text: "alpha"}, scope) {
			if err != nil {
				streamErr = err
				break
			}
		}
		require.ErrorIs(t, streamErr, store.ErrInvalidQuery)
	})

	t.Run("WildcardQueryIsInvalid", func(t *testing.T) {
		s, _, vs := vsf(t)
		seedVisibleSearchWorld(t, s)
		scope := map[string]search.TypeScope{
			search.WildcardType: {Query: ticketsInPRJ1()},
		}
		var streamErr error
		for _, err := range vs.SearchVisible(ctx(), search.Query{Text: "alpha", World: store.TrivialScope()}, scope) {
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
			World:   store.TrivialScope(),
		}

		// Expected: the first Limit hits of the OPEN-filtered ungated
		// ticket stream — i.e. the filter consumed no limit budget. A
		// limit-before-filter implementation under-fills whenever the
		// closed ticket outranks an open one (true on the linear and
		// bleve runs; pgstore's SQL-side LIMIT placement is pinned
		// deterministically by its own builder unit test).
		base := collectHits(t, ungated.Search(ctx(), search.Query{Text: "alpha", Types: []string{"ticket"}, World: store.TrivialScope()}))
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
			World:   store.TrivialScope(),
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
		plain := collectHits(t, vs.SearchVisible(ctx(), search.Query{Text: "alpha", World: store.TrivialScope()}, scope))
		sorted := collectHits(t, vs.SearchVisible(ctx(), search.Query{
			Text:  "alpha",
			Sort:  []search.SortClause{{Field: "title", Direction: search.SortDesc}},
			World: store.TrivialScope(),
		}, scope))
		require.Equal(t, plain, sorted, "q.Sort must be ignored by every implementation")
	})

	t.Run("EmptyTextListsVisible", func(t *testing.T) {
		s, ungated, vs := vsf(t)
		seedVisibleSearchWorld(t, s)
		scope := map[string]search.TypeScope{"ticket": allow}
		q := search.Query{Text: "", World: store.TrivialScope()}
		base := collectHits(t, ungated.Search(ctx(), q))
		got := collectHits(t, vs.SearchVisible(ctx(), q, scope))
		// Empty-text listing order is backend-defined; pin membership
		// (sorted) rather than order for this case only.
		requireSameIDSet(t, []string{"TKT-1", "TKT-2", "TKT-3"}, got)
		for _, h := range got {
			require.True(t, hitInSet(base, h.ID), "visible hit missing from ungated baseline")
		}
	})

	RunFacedVisibleSearchTests(t, vsf)
	t.Run("FieldScope", func(t *testing.T) { RunFieldScopeSearchTests(t, vsf) })
}

// RunFacedVisibleSearchTests pins the per-face half of the scope
// (TKT-7IZHP0): the verdict and the face allowlist trim each entity's
// candidate faces BEFORE the world ranks them. A denied prime falls
// through to a readable face, as on lists and the single-entity read, and
// a face the scope denies is never matched or served.
//
// The world prefers draft and falls back to published. POL-1 holds both
// faces, POL-2 only a draft, and the term "sketch" appears only in POL-1's
// draft.
func RunFacedVisibleSearchTests(t *testing.T, vsf VisibleSearchFactory) {
	t.Helper()
	draft, published := entity.Face("draft"), entity.Face("published")
	world := store.NewWorldScope(map[string]store.TypeResolution{
		"policy": {Chain: []entity.Face{draft, published}, Fallback: store.FallbackExclude},
	})
	seed := func(t *testing.T, s store.Store) {
		t.Helper()
		for _, e := range []struct {
			id    string
			face  entity.Face
			title string
		}{
			{"POL-1", draft, "alpha sketch"},
			{"POL-1", published, "alpha final"},
			{"POL-2", draft, "alpha other"},
			{"U-rev", "", "reviewer"},
		} {
			ent := entity.New(e.id, "policy")
			if e.id == "U-rev" {
				ent = entity.New(e.id, "user")
			}
			ent.Face = e.face
			ent.SetString("title", e.title)
			require.NoError(t, s.CreateEntity(ctx(), ent), "create %s@%s", e.id, e.face)
		}
		mustRel(t, s, "U-rev", "reviews", "POL-1")
		mustRel(t, s, "U-rev", "reviews", "POL-2")
	}
	requireFaces := func(t *testing.T, want map[string]entity.Face, hits []search.Hit) {
		t.Helper()
		got := make(map[string]entity.Face, len(hits))
		for _, h := range hits {
			got[h.ID] = h.Face
		}
		require.Equal(t, want, got)
	}
	reviewerOnPublished := func() *store.GraphQuery {
		return &store.GraphQuery{
			EntityType: "policy",
			Any: []store.GraphBranch{{
				HasInbound: &store.RelationPredicate{Endpoints: []string{"U-rev"}, OfTypes: []string{"reviews"}},
				FaceIn:     []entity.Face{published},
			}},
		}
	}

	t.Run("FaceAllowlistFallsThroughToReadableFace", func(t *testing.T) {
		s, _, vs := vsf(t)
		seed(t, s)
		scope := map[string]search.TypeScope{"policy": {AllowAll: true, Faces: []entity.Face{published}}}
		got := collectHits(t, vs.SearchVisible(ctx(), search.Query{Text: "alpha", World: world}, scope))
		requireFaces(t, map[string]entity.Face{"POL-1": published}, got)
	})

	t.Run("QueryVerdictIsPerFaceRow", func(t *testing.T) {
		s, _, vs := vsf(t)
		seed(t, s)
		scope := map[string]search.TypeScope{"policy": {Query: reviewerOnPublished()}}
		got := collectHits(t, vs.SearchVisible(ctx(), search.Query{Text: "alpha", World: world}, scope))
		requireFaces(t, map[string]entity.Face{"POL-1": published}, got)
	})

	t.Run("DeniedFaceIsNeverMatched", func(t *testing.T) {
		s, _, vs := vsf(t)
		seed(t, s)
		for name, scope := range map[string]map[string]search.TypeScope{
			"allowlist": {"policy": {AllowAll: true, Faces: []entity.Face{published}}},
			"verdict":   {"policy": {Query: reviewerOnPublished()}},
		} {
			t.Run(name, func(t *testing.T) {
				got := collectHits(t, vs.SearchVisible(ctx(), search.Query{Text: "sketch", World: world}, scope))
				require.Empty(t, got, "the term is only in the draft face, which the scope denies")
			})
		}
	})

	t.Run("EmptyTextServesTheAdmittedPrime", func(t *testing.T) {
		s, _, vs := vsf(t)
		seed(t, s)
		scope := map[string]search.TypeScope{"policy": {AllowAll: true, Faces: []entity.Face{published}}}
		got := collectHits(t, vs.SearchVisible(ctx(), search.Query{World: world, Types: []string{"policy"}}, scope))
		requireFaces(t, map[string]entity.Face{"POL-1": published}, got)
	})

	t.Run("AllFacesAdmittedServesTheWorldPrime", func(t *testing.T) {
		s, _, vs := vsf(t)
		seed(t, s)
		scope := map[string]search.TypeScope{"policy": {AllowAll: true}}
		got := collectHits(t, vs.SearchVisible(ctx(), search.Query{Text: "alpha", World: world}, scope))
		requireFaces(t, map[string]entity.Face{"POL-1": draft, "POL-2": draft}, got)
	})

	for name, scope := range map[string]map[string]search.TypeScope{
		// A GraphQuery targets one type, and a face allowlist on the
		// default verdict would apply to types the grant never named.
		"WildcardFaceAllowlistIsInvalid": {search.WildcardType: {AllowAll: true, Faces: []entity.Face{published}}},
		// store.GraphQuery.FaceIn reads an empty list as every face, so a
		// backend forwarding it would fail open.
		"EmptyFaceAllowlistIsInvalid/allowAll": {"policy": {AllowAll: true, Faces: []entity.Face{}}},
		"EmptyFaceAllowlistIsInvalid/query":    {"policy": {Query: reviewerOnPublished(), Faces: []entity.Face{}}},
	} {
		t.Run(name, func(t *testing.T) {
			s, _, vs := vsf(t)
			seed(t, s)
			var streamErr error
			for _, err := range vs.SearchVisible(ctx(), search.Query{Text: "alpha", World: world}, scope) {
				if err != nil {
					streamErr = err
					break
				}
			}
			require.ErrorIs(t, streamErr, search.ErrScope)
		})
	}
}

// RunFieldScopeSearchTests pins [search.TypeScope.Fields], the closure of the
// match-on-hidden-field oracle: a hit whose text matched ONLY properties the
// reader may not see is dropped, while a hit that matched a visible property,
// an ungated property, the id or the content survives. The limit counts the
// surviving hits.
//
// The fixture's `code` value ("zeta777") appears in no title, so a search for
// it can only match through that property.
func RunFieldScopeSearchTests(t *testing.T, vsf VisibleSearchFactory) {
	t.Helper()

	q := func(text string) search.Query { return search.Query{Text: text, World: store.TrivialScope()} }
	// codeHidden gates title, code and status and shows title only.
	codeHidden := func() *search.FieldScope {
		return &search.FieldScope{
			Gated:  []string{"code", "status", "title"},
			Grants: []search.FieldGrant{{Fields: []string{"title"}}},
		}
	}
	tickets := func(fs *search.FieldScope) map[string]search.TypeScope {
		return map[string]search.TypeScope{"ticket": {AllowAll: true, Fields: fs}}
	}
	run := func(t *testing.T, vs search.VisibleSearcher, query search.Query, scope map[string]search.TypeScope) []search.Hit {
		t.Helper()
		return collectHits(t, vs.SearchVisible(ctx(), query, scope))
	}

	t.Run("HiddenOnlyMatchDropped", func(t *testing.T) {
		s, _, vs := vsf(t)
		seedVisibleFieldWorld(t, s)
		require.Empty(t, run(t, vs, q("zeta777"), tickets(codeHidden())),
			"a hit matching only a hidden field must be dropped")
	})

	t.Run("NoFieldScopeShowsEverything", func(t *testing.T) {
		s, _, vs := vsf(t)
		seedVisibleFieldWorld(t, s)
		requireSameIDSet(t, []string{"TKT-1", "TKT-3"}, run(t, vs, q("zeta777"), tickets(nil)))
	})

	t.Run("VisibleFieldMatchKept", func(t *testing.T) {
		s, _, vs := vsf(t)
		seedVisibleFieldWorld(t, s)
		requireSameIDSet(t, []string{"TKT-1"}, run(t, vs, q("alpha"), tickets(codeHidden())))
	})

	t.Run("UndeclaredPropertyMatchKept", func(t *testing.T) {
		s, _, vs := vsf(t)
		seedVisibleFieldWorld(t, s)
		// `extra` is in no Gated list, so no grant decides it: it is visible.
		requireSameIDSet(t, []string{"TKT-2"}, run(t, vs, q("undeclaredvalue"), tickets(codeHidden())))
	})

	t.Run("IDMatchSurvives", func(t *testing.T) {
		s, _, vs := vsf(t)
		seedVisibleFieldWorld(t, s)
		require.True(t, hitInSet(run(t, vs, q("TKT-1"), tickets(codeHidden())), "TKT-1"),
			"an id match must survive: the id is never property-gated")
	})

	t.Run("ContentMatchSurvives", func(t *testing.T) {
		s, _, vs := vsf(t)
		seedVisibleFieldWorld(t, s)
		require.True(t, hitInSet(run(t, vs, q("uniquebodytoken"), tickets(codeHidden())), "TKT-3"),
			"a content match must survive: content is never property-gated")
	})

	t.Run("EntityLevelStillApplies", func(t *testing.T) {
		s, _, vs := vsf(t)
		seedVisibleFieldWorld(t, s)
		require.Empty(t, run(t, vs, q("zeta777"), map[string]search.TypeScope{}),
			"an empty scope denies everything before the field check")
	})

	t.Run("OtherTypesUngated", func(t *testing.T) {
		s, _, vs := vsf(t)
		seedVisibleFieldWorld(t, s)
		scope := tickets(codeHidden())
		scope["memo"] = search.TypeScope{AllowAll: true}
		requireSameIDSet(t, []string{"MEMO-1"}, run(t, vs, q("zeta777"), scope))
	})

	t.Run("GrantUnion", func(t *testing.T) {
		s, _, vs := vsf(t)
		seedVisibleFieldWorld(t, s)
		// Two grants, as two roles compile: code is visible through the second.
		fs := codeHidden()
		fs.Grants = append(fs.Grants, search.FieldGrant{Fields: []string{"code"}})
		requireSameIDSet(t, []string{"TKT-1", "TKT-3"}, run(t, vs, q("zeta777"), tickets(fs)))
	})

	t.Run("ConditionalGrantOnProperty", func(t *testing.T) {
		s, _, vs := vsf(t)
		seedVisibleFieldWorld(t, s)
		fs := codeHidden()
		fs.Grants = append(fs.Grants, search.FieldGrant{Fields: []string{"code"}, When: &store.GraphQuery{
			Props: []store.PropPredicate{{Property: "status", Op: store.PropEqual, Value: "open", Scalar: true}},
		}})
		requireSameIDSet(t, []string{"TKT-1"}, run(t, vs, q("zeta777"), tickets(fs)))
	})

	t.Run("EdgeGrantedRole", func(t *testing.T) {
		s, _, vs := vsf(t)
		seedVisibleFieldWorld(t, s)
		// carol holds a role on TKT-1 through an owner-of edge.
		held := store.RelationPredicate{Endpoints: []string{"carol"}, OfTypes: []string{"owner-of"}}
		fs := codeHidden()
		fs.Grants = append(fs.Grants, search.FieldGrant{Fields: []string{"code"}, When: &store.GraphQuery{
			Related: []store.DirectedRelation{{Incoming: true, Pred: held}},
		}})
		requireSameIDSet(t, []string{"TKT-1"}, run(t, vs, q("zeta777"), tickets(fs)))

		// The open rows where the role is NOT held see every gated property.
		notHeld := held
		notHeld.Negate = true
		open := &search.FieldScope{Gated: []string{"code", "status", "title"}, Grants: []search.FieldGrant{
			{Fields: []string{"title"}, When: &store.GraphQuery{Related: []store.DirectedRelation{{Incoming: true, Pred: held}}}},
			{Fields: []string{"code", "status", "title"}, When: &store.GraphQuery{
				Related: []store.DirectedRelation{{Incoming: true, Pred: notHeld}},
			}},
		}}
		requireSameIDSet(t, []string{"TKT-3"}, run(t, vs, q("zeta777"), tickets(open)))
	})

	t.Run("LimitCountsVisibleHitsOnly", func(t *testing.T) {
		s, _, vs := vsf(t)
		for i := range 6 {
			e := entity.New(fmt.Sprintf("HID-%d", i), "ticket")
			e.SetString("title", "plain")
			e.SetString("code", "omega42")
			require.NoError(t, s.CreateEntity(ctx(), e))
		}
		for i := range 3 {
			e := entity.New(fmt.Sprintf("VIS-%d", i), "ticket")
			e.SetString("title", "omega42 shown")
			require.NoError(t, s.CreateEntity(ctx(), e))
		}
		query := q("omega42")
		query.Limit = 2
		got := run(t, vs, query, tickets(codeHidden()))
		require.Len(t, got, 2, "the limit counts surviving hits, not candidates")
		for _, h := range got {
			require.True(t, strings.HasPrefix(h.ID, "VIS-"), "hidden-only hit %s returned", h.ID)
		}
		query.Limit = 5
		require.Len(t, run(t, vs, query, tickets(codeHidden())), 3)
	})

	t.Run("NonASCIICaseFolding", func(t *testing.T) {
		s, _, vs := vsf(t)
		e := entity.New("TKT-U", "ticket")
		e.SetString("title", "ÉCLAIR Ärger")
		e.SetString("code", "ØRESUND")
		require.NoError(t, s.CreateEntity(ctx(), e))
		requireSameIDSet(t, []string{"TKT-U"}, run(t, vs, q("éclair"), tickets(codeHidden())))
		requireSameIDSet(t, []string{"TKT-U"}, run(t, vs, q("ärger"), tickets(codeHidden())))
		require.Empty(t, run(t, vs, q("øresund"), tickets(codeHidden())),
			"a case-folded match in a hidden field must be dropped")
		requireSameIDSet(t, []string{"TKT-U"}, run(t, vs, q("øresund"), tickets(nil)))
	})

	t.Run("BackendFoldsLikeGo", func(t *testing.T) {
		// BUG-YG126C. Each pair holds the needle in the visible title only
		// under Go's fold, and once more literally in the hidden code of the
		// first entity. Whether an entity is a hit must not depend on its
		// hidden field, so both must be found. A backend that folds case
		// its own way finds only the first: through the hidden field, while
		// the Go judgement credits the title. "é" is a short needle (LIKE
		// folds ASCII only); "ԩab" a long one (FTS5's fold skips U+0528).
		for _, tc := range []struct{ needle, title, code string }{
			{"é", "CAFÉ", "café"},
			{"ԩab", "ԨAB", "xԩab"},
		} {
			t.Run(tc.needle, func(t *testing.T) {
				s, _, vs := vsf(t)
				both := entity.New("TKT-B", "ticket")
				both.SetString("title", tc.title)
				both.SetString("code", tc.code)
				require.NoError(t, s.CreateEntity(ctx(), both))
				titled := entity.New("TKT-T", "ticket")
				titled.SetString("title", tc.title)
				require.NoError(t, s.CreateEntity(ctx(), titled))
				requireSameIDSet(t, []string{"TKT-B", "TKT-T"}, run(t, vs, q(tc.needle), tickets(codeHidden())))
				requireSameIDSet(t, []string{"TKT-B", "TKT-T"}, run(t, vs, q(tc.needle), tickets(nil)))
			})
		}
	})

	t.Run("FoldDiffersBetweenGoAndSQL", func(t *testing.T) {
		s, ungated, vs := vsf(t)
		// Go folds 'İ' to "i"; PostgreSQL folds it to "i" plus U+0307. The
		// needle "i\u0307" is therefore in the visible title only under
		// PostgreSQL's fold, and in the hidden code under both.
		e := entity.New("TKT-I", "ticket")
		e.SetString("title", "İstanbul")
		e.SetString("code", "xi\u0307x")
		require.NoError(t, s.CreateEntity(ctx(), e))
		needle := "i\u0307"
		require.Empty(t, run(t, vs, q(needle), tickets(codeHidden())),
			"a needle only the hidden field holds under the search fold must not return the entity")
		if hitInSet(collectHits(t, ungated.Search(ctx(), q(needle))), "TKT-I") {
			requireSameIDSet(t, []string{"TKT-I"}, run(t, vs, q(needle), tickets(nil)))
		}
	})

	t.Run("TwoGatedTypesInOneStatement", func(t *testing.T) {
		s, _, vs := vsf(t)
		seedVisibleFieldWorld(t, s)
		held := store.RelationPredicate{Endpoints: []string{"carol"}, OfTypes: []string{"owner-of"}}
		ownedCode := func() *search.FieldScope {
			fs := codeHidden()
			fs.Grants = append(fs.Grants, search.FieldGrant{Fields: []string{"code"}, When: &store.GraphQuery{
				Related: []store.DirectedRelation{{Incoming: true, Pred: held}},
			}})
			return fs
		}
		// Both types render a relation condition, each with its own CTE.
		scope := tickets(ownedCode())
		scope["memo"] = search.TypeScope{AllowAll: true, Fields: ownedCode()}
		requireSameIDSet(t, []string{"TKT-1"}, run(t, vs, q("zeta777"), scope))

		mustRel(t, s, "carol", "owner-of", "MEMO-1")
		requireSameIDSet(t, []string{"TKT-1", "MEMO-1"}, run(t, vs, q("zeta777"), scope))
	})

	t.Run("RoleUnion", func(t *testing.T) {
		// The shapes the role union compiles (BUG-Y5UF3K). A global role
		// with a block shows title on every row. A role without a block,
		// conferred on TKT-1 through owner-of, shows every gated property
		// there; a role with a block conferred through the same edge adds
		// title only. Holding the second role must not take away what the
		// first shows.
		s, _, vs := vsf(t)
		seedVisibleFieldWorld(t, s)
		held := store.RelationPredicate{Endpoints: []string{"carol"}, OfTypes: []string{"owner-of"}}
		onHeld := func() *store.GraphQuery {
			return &store.GraphQuery{Related: []store.DirectedRelation{{Incoming: true, Pred: held}}}
		}
		fs := codeHidden()
		fs.Grants = append(fs.Grants,
			search.FieldGrant{Fields: []string{"code", "status", "title"}, When: onHeld()},
			search.FieldGrant{Fields: []string{"title"}, When: onHeld()},
		)
		requireSameIDSet(t, []string{"TKT-1"}, run(t, vs, q("zeta777"), tickets(fs)))

		// A role without a block held globally shows every gated property
		// on every row: a grant with no condition.
		fs.Grants = append(fs.Grants, search.FieldGrant{Fields: []string{"code", "status", "title"}})
		requireSameIDSet(t, []string{"TKT-1", "TKT-3"}, run(t, vs, q("zeta777"), tickets(fs)))
	})

	t.Run("FacedRowsAndInheritedHolder", func(t *testing.T) {
		runFacedFieldScope(t, vsf)
	})

	t.Run("DifferentialAgainstMatchTextFields", func(t *testing.T) {
		s, ungated, vs := vsf(t)
		runFieldScopeDifferential(t, s, ungated, vs)
	})
}

// runFacedFieldScope pins field grants on faced rows and on a role held
// through an inherited relation, the two shapes the ACL compiles that a flat
// fixture does not exercise. POL-1 belongs to PRJ-9, which carol owns, so a
// grant held through owner-of (inherited through belongs-to) holds on POL-1;
// POL-2 belongs to nothing. Both faces of POL-1 carry the same code, so only
// the grant's FaceIn decides between them.
func runFacedFieldScope(t *testing.T, vsf VisibleSearchFactory) {
	t.Helper()
	draft, published := entity.Face("draft"), entity.Face("published")
	world := store.NewWorldScope(map[string]store.TypeResolution{
		"policy": {Chain: []entity.Face{draft, published}, Fallback: store.FallbackExclude},
	})
	seed := func(t *testing.T, s store.Store) {
		t.Helper()
		for _, e := range []struct {
			id, typ string
			face    entity.Face
		}{
			{"carol", "person", ""},
			{"PRJ-9", "project", ""},
			{"POL-1", "policy", draft},
			{"POL-1", "policy", published},
			{"POL-2", "policy", draft},
		} {
			ent := entity.New(e.id, e.typ)
			ent.Face = e.face
			ent.SetString("title", "policy "+e.id)
			if e.typ == "policy" {
				ent.SetString("code", "zeta777")
			}
			require.NoError(t, s.CreateEntity(ctx(), ent), "create %s@%s", e.id, e.face)
		}
		mustRel(t, s, "carol", "owner-of", "PRJ-9")
		mustRel(t, s, "POL-1", "belongs-to", "PRJ-9")
	}
	held := store.RelationPredicate{
		Endpoints: []string{"carol"}, OfTypes: []string{"owner-of"},
		EntityInheritThrough: []string{"belongs-to"}, EntityDepth: 10,
	}
	scopeOn := func(faceIn []entity.Face, faces []entity.Face) map[string]search.TypeScope {
		fs := &search.FieldScope{Gated: []string{"code", "title"}, Grants: []search.FieldGrant{
			{Fields: []string{"title"}},
			{Fields: []string{"code"}, When: &store.GraphQuery{
				Related: []store.DirectedRelation{{Incoming: true, Pred: held}},
				FaceIn:  faceIn,
			}},
		}}
		return map[string]search.TypeScope{"policy": {AllowAll: true, Faces: faces, Fields: fs}}
	}
	query := search.Query{Text: "zeta777", World: world}
	// A global role without a block that reads only published rows
	// compiles to a grant whose condition is the face alone (BUG-Y5UF3K).
	t.Run("face-only grant of a block-less role", func(t *testing.T) {
		s, _, vs := vsf(t)
		seed(t, s)
		fs := &search.FieldScope{Gated: []string{"code", "title"}, Grants: []search.FieldGrant{
			{Fields: []string{"title"}},
			{Fields: []string{"code", "title"}, When: &store.GraphQuery{FaceIn: []entity.Face{published}}},
		}}
		got := map[string]entity.Face{}
		for _, h := range collectHits(t, vs.SearchVisible(ctx(), query,
			map[string]search.TypeScope{"policy": {AllowAll: true, Faces: []entity.Face{published}, Fields: fs}})) {
			got[h.ID] = h.Face
		}
		require.Equal(t, map[string]entity.Face{"POL-1": published}, got)
		got = map[string]entity.Face{}
		for _, h := range collectHits(t, vs.SearchVisible(ctx(), query,
			map[string]search.TypeScope{"policy": {AllowAll: true, Faces: []entity.Face{draft}, Fields: fs}})) {
			got[h.ID] = h.Face
		}
		require.Empty(t, got, "the grant does not hold on draft rows")
	})
	for _, tc := range []struct {
		name          string
		faceIn, faces []entity.Face
		want          map[string]entity.Face
	}{
		{"inherited holder on the served face", []entity.Face{draft}, nil, map[string]entity.Face{"POL-1": draft}},
		{"grant on another face than the served one", []entity.Face{draft}, []entity.Face{published},
			map[string]entity.Face{}},
		{"grant on the served fallback face", []entity.Face{published}, []entity.Face{published},
			map[string]entity.Face{"POL-1": published}},
		// The form a related() condition lowers to: never on a named face.
		{"zero-face grant on faced rows", []entity.Face{""}, nil, map[string]entity.Face{}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s, _, vs := vsf(t)
			seed(t, s)
			got := map[string]entity.Face{}
			for _, h := range collectHits(t, vs.SearchVisible(ctx(), query, scopeOn(tc.faceIn, tc.faces))) {
				got[h.ID] = h.Face
			}
			require.Equal(t, tc.want, got)
		})
	}
}

// runFieldScopeDifferential compares the backend's field check against the
// ground truth, [search.MatchTextFields] plus [search.FieldScope.AdmitsMatch],
// on generated entities and scopes. The candidates are the backend's own
// ungated hits. A backend that reports its own match provenance (bleve
// tokenizes and folds accents) is judged on that provenance; every other
// backend, pgstore and sqlitestore included, on MatchTextFields itself.
func runFieldScopeDifferential(t *testing.T, s store.Store, ungated search.Searcher, vs search.VisibleSearcher) {
	t.Helper()
	matchFields := search.MatchTextFields
	if p, ok := ungated.(interface {
		MatchedFields(*entity.Entity, string) map[string]struct{}
	}); ok {
		matchFields = p.MatchedFields
	}
	rng := rand.New(rand.NewPCG(7, 11))
	words := []string{"kiwi", "Kiwi", "KIWIS", "émile", "ÉMILE", "straße", "lime", "plum"}
	props := []string{"a", "b", "c", "d"}
	statuses := []string{"open", "done"}
	pick := func(xs []string) string { return xs[rng.IntN(len(xs))] }
	all := make([]*entity.Entity, 0, 40)
	for i := range 40 {
		e := entity.New(fmt.Sprintf("DIF-%02d", i), "ticket")
		for _, p := range props {
			if rng.IntN(2) == 0 {
				e.SetString(p, pick(words)+" "+pick(words))
			}
		}
		e.SetString("st", pick(statuses))
		if rng.IntN(4) == 0 {
			e.Content = pick(words)
		}
		require.NoError(t, s.CreateEntity(ctx(), e))
		all = append(all, e)
	}
	for round := range 12 {
		var gated []string
		for _, p := range props {
			if rng.IntN(3) > 0 {
				gated = append(gated, p)
			}
		}
		fs := &search.FieldScope{Gated: gated}
		fs.Grants = append(fs.Grants, search.FieldGrant{Fields: []string{pick(props)}})
		status := pick(statuses)
		fs.Grants = append(fs.Grants, search.FieldGrant{Fields: []string{pick(props), pick(props)}, When: &store.GraphQuery{
			Props: []store.PropPredicate{{Property: "st", Op: store.PropEqual, Value: status, Scalar: true}},
		}})
		text := strings.Fields(pick(words))[0]
		if round%3 == 0 {
			text = strings.ToUpper(text[:2])
		}
		byID := make(map[string]*entity.Entity, len(all))
		for _, e := range all {
			byID[e.ID] = e
		}
		want := []string{}
		for _, h := range collectHits(t, ungated.Search(ctx(), search.Query{Text: text, World: store.TrivialScope()})) {
			e := byID[h.ID]
			matched := matchFields(e, text)
			if matched == nil {
				continue
			}
			if fs.AdmitsMatch(matched, func(int) bool { return e.Properties["st"] == status }) {
				want = append(want, e.ID)
			}
		}
		scope := map[string]search.TypeScope{"ticket": {AllowAll: true, Fields: fs}}
		got := collectHits(t, vs.SearchVisible(ctx(), search.Query{Text: text, World: store.TrivialScope()}, scope))
		requireSameIDSet(t, want, got)
	}
}

// seedVisibleFieldWorld builds the fixture of [RunFieldScopeSearchTests]:
//
//	TKT-1 "alpha rocket"  code=zeta777  status=open   (carol owner-of)
//	TKT-2 "beta lander"   extra=undeclaredvalue
//	TKT-3 "gamma orbiter" code=zeta777  status=done   body "uniquebodytoken"
//	MEMO-1 "memo"         code=zeta777
func seedVisibleFieldWorld(t *testing.T, s store.Store) {
	t.Helper()
	seed := func(id, typ string, props map[string]string, content string) {
		e := entity.New(id, typ)
		for k, v := range props {
			e.SetString(k, v)
		}
		e.Content = content
		require.NoError(t, s.CreateEntity(ctx(), e), "create %s", id)
	}
	seed("carol", "person", map[string]string{"title": "Carol"}, "")
	seed("TKT-1", "ticket", map[string]string{"title": "alpha rocket", "code": "zeta777", "status": "open"}, "")
	seed("TKT-2", "ticket", map[string]string{"title": "beta lander", "extra": "undeclaredvalue"}, "")
	seed("TKT-3", "ticket", map[string]string{"title": "gamma orbiter", "code": "zeta777", "status": "done"},
		"uniquebodytoken operational notes")
	seed("MEMO-1", "memo", map[string]string{"title": "memo", "code": "zeta777"}, "")
	mustRel(t, s, "carol", "owner-of", "TKT-1")
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
