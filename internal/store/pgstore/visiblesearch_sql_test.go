package pgstore

import (
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/search"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// TestBuildVisibleSearchSQL_Keyset pins the page shape deterministically (no
// database): every statement is one LIMITed page, and a resumed page carries
// the keyset of the result's order (rank, then id; id alone without text).
func TestBuildVisibleSearchSQL_Keyset(t *testing.T) {
	scope := map[string]search.TypeScope{"ticket": {AllowAll: true}}

	t.Run("first page: LIMIT, no keyset", func(t *testing.T) {
		sqlText, args, ok := buildVisibleSearchSQL(search.Query{Text: "alpha", World: store.TrivialScope()}, scope, nil, nil, 7)
		if !ok {
			t.Fatal("expected a query")
		}
		if !strings.HasSuffix(sqlText, " LIMIT $"+strconv.Itoa(len(args))) || args[len(args)-1] != 7 {
			t.Errorf("want a trailing LIMIT 7: %s %v", sqlText, args)
		}
		if strings.Contains(sqlText, "e.id >") {
			t.Errorf("first page must carry no keyset: %s", sqlText)
		}
	})

	t.Run("text: resumes after rank and id", func(t *testing.T) {
		after := &visibleKey{rank: 0.5, id: "T-9"}
		sqlText, args, _ := buildVisibleSearchSQL(search.Query{Text: "alpha", World: store.TrivialScope()}, scope, nil, after, 7)
		if !strings.Contains(sqlText, " OR (") || !strings.Contains(sqlText, "e.id >") {
			t.Errorf("want a (rank, id) keyset: %s", sqlText)
		}
		if !slices.Contains(args, any(float32(0.5))) || !slices.Contains(args, any("T-9")) {
			t.Errorf("keyset args missing: %v", args)
		}
	})

	t.Run("no text: resumes after id", func(t *testing.T) {
		after := &visibleKey{id: "T-9"}
		sqlText, _, _ := buildVisibleSearchSQL(search.Query{World: store.TrivialScope()}, scope, nil, after, 7)
		if !strings.Contains(sqlText, "AND e.id > $") || strings.Contains(sqlText, " OR (") {
			t.Errorf("want an id-only keyset: %s", sqlText)
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
			Faces: store.InWorld(store.TrivialScope()),
		}
	}

	t.Run("empty scope: no query", func(t *testing.T) {
		if _, _, ok := buildVisibleSearchSQL(search.Query{Text: "x", World: store.TrivialScope()}, nil, nil, nil, 1); ok {
			t.Error("nil scope must not produce a query")
		}
		deny := map[string]search.TypeScope{"ticket": {}}
		if _, _, ok := buildVisibleSearchSQL(search.Query{Text: "x", World: store.TrivialScope()}, deny, nil, nil, 1); ok {
			t.Error("zero-value-only scope must not produce a query")
		}
	})

	t.Run("wildcard allow: no visibility clause", func(t *testing.T) {
		scope := map[string]search.TypeScope{search.WildcardType: {AllowAll: true}}
		sqlText, _, ok := buildVisibleSearchSQL(search.Query{Text: "x", World: store.TrivialScope()}, scope, nil, nil, 1)
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
		sqlText, _, ok := buildVisibleSearchSQL(search.Query{Text: "x", World: store.TrivialScope()}, scope, nil, nil, 1)
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
