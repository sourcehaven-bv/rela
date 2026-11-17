package search_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/search"
	"github.com/Sourcehaven-BV/rela/internal/search/bleveindex"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/store/memstore"
	"github.com/Sourcehaven-BV/rela/internal/store/storetest"
)

// bodyStore stores n requirements with bodies, every other one open, and
// n/10 docs, plus a faced policy family per tenth requirement. It returns
// the raw store; the caller wraps it.
func bodyStore(t *testing.T, n int) store.Store {
	t.Helper()
	ctx := context.Background()
	st := memstore.New()
	for i := range n {
		e := entity.New(fmt.Sprintf("REQ-%04d", i), "requirement")
		e.SetString("title", fmt.Sprintf("req %d", i))
		e.SetString("status", []string{"open", "closed"}[i%2])
		e.Content = "body of a requirement"
		require.NoError(t, st.CreateEntity(ctx, e))
	}
	for i := range n / 10 {
		e := entity.New(fmt.Sprintf("DOC-%04d", i), "doc")
		e.SetString("title", "doc")
		e.Content = "body of a doc"
		require.NoError(t, st.CreateEntity(ctx, e))
		for j, face := range []entity.Face{"draft", "published"} {
			if i%3 == 1 && j == 1 {
				continue // some families have no published face
			}
			p := entity.New(fmt.Sprintf("POL-%04d", i), "policy")
			p.Face = face
			p.SetString("title", fmt.Sprintf("policy %d at %s", i, face))
			p.SetString("status", []string{"open", "closed"}[(i+j)%2])
			p.Content = "body of a policy"
			require.NoError(t, st.CreateEntity(ctx, p))
		}
	}
	return st
}

// publishedWorld ranks a policy's published face over its draft and excludes
// a family with neither.
func publishedWorld() store.WorldScope {
	return store.NewWorldScope(map[string]store.TypeResolution{
		"policy": {Chain: []entity.Face{"published", "draft"}, Fallback: store.FallbackExclude},
	})
}

// noTextQueries are searches with no text over bodyStore, covering one type,
// several, every type, filters, a world and the admitting path.
func noTextQueries() map[string]search.Query {
	admitAll := func(c []search.Candidate) ([]search.Candidate, error) { return c, nil }
	admitOpen := func(c []search.Candidate) ([]search.Candidate, error) {
		// Admits by face, so the prime may change: drafts of even
		// families are not admitted.
		out := c[:0:0]
		for _, cand := range c {
			if cand.Face != "draft" || len(cand.ID)%2 == 0 {
				out = append(out, cand)
			}
		}
		return out, nil
	}
	open := []search.PropertyFilter{{Property: "status", Op: search.FilterEq, Value: "open"}}
	return map[string]search.Query{
		"one type":            {Types: []string{"requirement"}, Limit: 7, World: store.TrivialScope()},
		"one type, no limit":  {Types: []string{"doc"}, World: store.TrivialScope()},
		"two types":           {Types: []string{"requirement", "doc"}, Limit: 40, World: store.TrivialScope()},
		"every type":          {Limit: 300, World: store.TrivialScope()},
		"every type, filter":  {Filters: open, Limit: 50, World: store.TrivialScope()},
		"world":               {Types: []string{"policy"}, World: publishedWorld()},
		"world, every type":   {Limit: 90, World: publishedWorld()},
		"admit all":           {Types: []string{"policy", "doc"}, Limit: 60, World: publishedWorld(), Admit: admitAll},
		"admit some, filter":  {Filters: open, World: publishedWorld(), Admit: admitOpen},
		"admit, trivial":      {Types: []string{"requirement"}, Limit: 5, World: store.TrivialScope(), Admit: admitAll},
		"admit, long scan":    {Limit: 1000, World: publishedWorld(), Admit: admitAll},
		"limit of one, world": {Types: []string{"policy"}, Limit: 1, World: publishedWorld()},
	}
}

// A search with no text reads no body (collection-reads rule, TKT-Q918DFR):
// a hit needs the id, type, face and title, which a header page carries.
func TestSearch_NoTextReadsNoBodies(t *testing.T) {
	t.Parallel()
	for name, q := range noTextQueries() {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			watch := storetest.NewBodyWatch(bodyStore(t, 200))
			idx, err := bleveindex.NewMem()
			require.NoError(t, err)
			t.Cleanup(func() { _ = idx.Close() })
			watch.Reset()
			ids := hitIDs(t, search.New(watch, idx).Search(context.Background(), q))
			require.NotEmpty(t, ids, "the query should find something")
			require.Zero(t, watch.Bodies(), "a search with no text read bodies: %s", watch)
		})
	}
}

// The gated search with no text reads no body either, for an allow-all
// scope and for a scope with a row query.
func TestSearchVisible_NoTextReadsNoBodies(t *testing.T) {
	t.Parallel()
	open := &store.GraphQuery{EntityType: "requirement", Props: []store.PropPredicate{
		{Property: "status", Op: store.PropEqual, Value: "open", Scalar: true},
	}}
	for name, scope := range map[string]search.TypeScope{
		"allow all": {AllowAll: true},
		"row query": {Query: open},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			watch := storetest.NewBodyWatch(bodyStore(t, 200))
			idx, err := bleveindex.NewMem()
			require.NoError(t, err)
			t.Cleanup(func() { _ = idx.Close() })
			v, err := search.NewVisible(search.New(watch, idx), watch)
			require.NoError(t, err)
			watch.Reset()
			q := search.Query{Types: []string{"requirement"}, Limit: 40, World: store.TrivialScope()}
			ids := hitIDs(t, v.SearchVisible(context.Background(), q,
				map[string]search.TypeScope{"requirement": scope}))
			require.Len(t, ids, 40)
			require.Zero(t, watch.Bodies(), "a gated search with no text read bodies: %s", watch)
		})
	}
}

// entityReaderOnly hides every optional capability of a store, so a search
// over it pages with ListEntitiesPage, as it did before TKT-Q918DFR.
type entityReaderOnly struct{ store.EntityReader }

// Reading header pages changes no result: every search with no text returns
// the same hits, in the same order, as one over the entity pages.
func TestSearch_NoTextHeaderPagesMatchEntityPages(t *testing.T) {
	t.Parallel()
	st := bodyStore(t, 200)
	idx, err := bleveindex.NewMem()
	require.NoError(t, err)
	t.Cleanup(func() { _ = idx.Close() })
	headers := search.New(st, idx)
	entities := search.New(entityReaderOnly{st}, idx)
	for name, q := range noTextQueries() {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			want := hits(t, entities.Search(context.Background(), q))
			require.NotEmpty(t, want)
			require.Equal(t, want, hits(t, headers.Search(context.Background(), q)))
		})
	}
}

func hits(t *testing.T, seq func(func(search.Hit, error) bool)) []search.Hit {
	t.Helper()
	var out []search.Hit
	for h, err := range seq {
		require.NoError(t, err)
		out = append(out, h)
	}
	return out
}
