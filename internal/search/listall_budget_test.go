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

// listStore stores n requirements, every other one open, and n/10 docs,
// and returns it behind a read counter with a searcher over it.
func listStore(t *testing.T, n int) (*storetest.Counting, *search.Service) {
	t.Helper()
	ctx := context.Background()
	idx, err := bleveindex.NewMem()
	require.NoError(t, err)
	t.Cleanup(func() { _ = idx.Close() })
	base := memstore.New()
	for i := range n {
		e := entity.New(fmt.Sprintf("REQ-%04d", i), "requirement")
		e.SetString("title", "req")
		e.SetString("status", []string{"open", "closed"}[i%2])
		require.NoError(t, base.CreateEntity(ctx, e))
	}
	for i := range n / 10 {
		e := entity.New(fmt.Sprintf("DOC-%04d", i), "doc")
		e.SetString("title", "doc")
		require.NoError(t, base.CreateEntity(ctx, e))
	}
	st := storetest.NewCounting(base)
	return st, search.New(st, idx)
}

func hitIDs(t *testing.T, seq func(func(search.Hit, error) bool)) []string {
	t.Helper()
	var ids []string
	for h, err := range seq {
		require.NoError(t, err)
		ids = append(ids, h.ID)
	}
	return ids
}

// A search with no text reads the store a page at a time and stops at the
// limit, so its reads are the same over 100 rows as over 1,000
// (TKT-D9UO9Y4). Several types are merged in id order.
func TestSearch_NoTextIsScaleFree(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name  string
		types []string
		want  []string
	}{
		{"one type", []string{"requirement"}, []string{"REQ-0000", "REQ-0001", "REQ-0002"}},
		{"two types", []string{"requirement", "doc"}, []string{"DOC-0000", "DOC-0001", "DOC-0002"}},
		{"every type", nil, []string{"DOC-0000", "DOC-0001", "DOC-0002"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			storetest.AssertScaleFree(t, func(n int) (storetest.Cost, string) {
				st, svc := listStore(t, n)
				st.Reset()
				q := search.Query{Types: tc.types, Limit: 3, World: store.TrivialScope()}
				require.Equal(t, tc.want, hitIDs(t, svc.Search(context.Background(), q)))
				return st.Cost(), st.String()
			})
		})
	}
}

// The gated search with no text admits a batch at a time and stops at the
// limit, for an allow-all scope and for a scope with a row query.
func TestSearchVisible_NoTextIsScaleFree(t *testing.T) {
	t.Parallel()
	open := &store.GraphQuery{EntityType: "requirement", Props: []store.PropPredicate{
		{Property: "status", Op: store.PropEqual, Value: "open", Scalar: true},
	}}
	for _, tc := range []struct {
		name  string
		scope search.TypeScope
		want  []string
	}{
		{"allow all", search.TypeScope{AllowAll: true}, []string{"REQ-0000", "REQ-0001", "REQ-0002", "REQ-0003"}},
		{"row query", search.TypeScope{Query: open}, []string{"REQ-0000", "REQ-0002", "REQ-0004", "REQ-0006"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			storetest.AssertScaleFree(t, func(n int) (storetest.Cost, string) {
				st, svc := listStore(t, n)
				v, err := search.NewVisible(svc, st)
				require.NoError(t, err)
				st.Reset()
				q := search.Query{Types: []string{"requirement"}, Limit: 4, World: store.TrivialScope()}
				got := hitIDs(t, v.SearchVisible(context.Background(), q,
					map[string]search.TypeScope{"requirement": tc.scope}))
				require.Equal(t, tc.want, got)
				return st.Cost(), st.String()
			})
		})
	}
}

// An admitting search with no text admits whole families: a family whose
// faces straddle a store page is resolved once, on all of its faces. The 31
// rows before A fill the first page (32 rows) up to A@draft.
func TestSearch_NoTextAdmitsWholeFamilies(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	idx, err := bleveindex.NewMem()
	require.NoError(t, err)
	t.Cleanup(func() { _ = idx.Close() })
	st := memstore.New()
	refs := make([]entity.Ref, 0, 34)
	for i := range 31 {
		refs = append(refs, entity.Ref{ID: fmt.Sprintf("0-%02d", i), Face: "draft"})
	}
	refs = append(refs, entity.Ref{ID: "A", Face: "draft"}, entity.Ref{ID: "A", Face: "published"},
		entity.Ref{ID: "B", Face: "draft"})
	for _, r := range refs {
		e := entity.New(r.ID, "policy")
		e.Face = r.Face
		e.SetString("title", r.ID)
		require.NoError(t, st.CreateEntity(ctx, e))
	}
	world := store.NewWorldScope(map[string]store.TypeResolution{
		"policy": {Chain: []entity.Face{"published", "draft"}, Fallback: store.FallbackExclude},
	})
	var batches [][]string
	q := search.Query{World: world, Admit: func(c []search.Candidate) ([]search.Candidate, error) {
		var refs []string
		for _, cand := range c {
			refs = append(refs, cand.ID+"@"+cand.Face.String())
		}
		batches = append(batches, refs)
		return c, nil
	}}
	var got []string
	for h, err := range search.New(st, idx).Search(ctx, q) {
		require.NoError(t, err)
		got = append(got, h.ID+"@"+h.Face.String())
	}
	require.Len(t, got, 33)
	require.Equal(t, []string{"A@published", "B@draft"}, got[31:])
	require.Len(t, batches, 2, "the first page should flush before A")
	require.Equal(t, []string{"A@draft", "A@published", "B@draft"}, batches[1],
		"A's faces were admitted apart")
}
