package pgstore

import (
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/search"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// TestBuildVisibleSearchSQL_LimitPlacement pins the load-bearing LIMIT
// rule deterministically (no database): with Go-side q.Filters pending,
// the SQL LIMIT must be omitted — a LIMIT below the filter would spend
// the row budget on rows MatchFilters is about to drop, re-opening the
// starvation gap the post-visibility contract closes. Without filters,
// the LIMIT is pushed down.
func TestBuildVisibleSearchSQL_LimitPlacement(t *testing.T) {
	scope := map[string]search.TypeScope{"ticket": {AllowAll: true}}

	t.Run("no filters: LIMIT pushed into SQL", func(t *testing.T) {
		sqlText, args, ok := buildVisibleSearchSQL(search.Query{Text: "alpha", Limit: 7}, scope)
		if !ok {
			t.Fatal("expected a query")
		}
		if !strings.Contains(sqlText, " LIMIT ") {
			t.Errorf("LIMIT missing from SQL: %s", sqlText)
		}
		if args[len(args)-1] != 7 {
			t.Errorf("last arg = %v, want the limit 7", args[len(args)-1])
		}
	})

	t.Run("with filters: LIMIT stays above MatchFilters", func(t *testing.T) {
		q := search.Query{
			Text:    "alpha",
			Limit:   7,
			Filters: []search.PropertyFilter{{Property: "status", Value: "open", Op: search.FilterEq}},
		}
		sqlText, _, ok := buildVisibleSearchSQL(q, scope)
		if !ok {
			t.Fatal("expected a query")
		}
		if strings.Contains(sqlText, " LIMIT ") {
			t.Errorf("SQL LIMIT must be omitted when Go-side filters remain: %s", sqlText)
		}
	})
}

// TestBuildVisibleSearchSQL_Shape pins structural properties of the
// composed statement that the DB-gated conformance run can't assert
// textually: deny-everything yields no query at all, the wildcard-allow
// scope drops the visibility disjunction, and two Query verdicts get
// distinct CTE prefixes.
func TestBuildVisibleSearchSQL_Shape(t *testing.T) {
	pred := func() *store.GraphQuery {
		return &store.GraphQuery{
			EntityType: "ticket",
			HasOutbound: &store.RelationPredicate{
				Endpoints: []string{"PRJ-1"}, OfTypes: []string{"belongs-to"},
				InheritThrough: []string{"member-of"}, Depth: 2,
			},
		}
	}

	t.Run("empty scope: no query", func(t *testing.T) {
		if _, _, ok := buildVisibleSearchSQL(search.Query{Text: "x"}, nil); ok {
			t.Error("nil scope must not produce a query")
		}
		deny := map[string]search.TypeScope{"ticket": {}}
		if _, _, ok := buildVisibleSearchSQL(search.Query{Text: "x"}, deny); ok {
			t.Error("zero-value-only scope must not produce a query")
		}
	})

	t.Run("wildcard allow: no visibility clause", func(t *testing.T) {
		scope := map[string]search.TypeScope{search.WildcardType: {AllowAll: true}}
		sqlText, _, ok := buildVisibleSearchSQL(search.Query{Text: "x"}, scope)
		if !ok {
			t.Fatal("expected a query")
		}
		if strings.Contains(sqlText, "e.type =") {
			t.Errorf("wildcard-allow must not emit type restrictions: %s", sqlText)
		}
	})

	t.Run("two Query verdicts: distinct CTE prefixes", func(t *testing.T) {
		docPred := pred()
		docPred.EntityType = "doc"
		scope := map[string]search.TypeScope{
			"doc":    {Query: docPred},
			"ticket": {Query: pred()},
		}
		sqlText, _, ok := buildVisibleSearchSQL(search.Query{Text: "x"}, scope)
		if !ok {
			t.Fatal("expected a query")
		}
		// Sorted scope keys: doc → v0, ticket → v1.
		for _, cte := range []string{"v0_out_endpoint_closure", "v1_out_endpoint_closure"} {
			if !strings.Contains(sqlText, cte) {
				t.Errorf("expected CTE %s in SQL: %s", cte, sqlText)
			}
		}
	})
}
