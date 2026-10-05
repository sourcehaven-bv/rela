package search_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/search"
	"github.com/Sourcehaven-BV/rela/internal/search/bleveindex"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/store/memstore"
)

// TestVisible_CallerAdmitNarrowsTheScope pins that a caller's Query.Admit
// runs after the scope's admission rather than replacing it: the caller
// sees only candidates the scope admitted, and can trim them further.
func TestVisible_CallerAdmitNarrowsTheScope(t *testing.T) {
	idx, err := bleveindex.NewMem()
	require.NoError(t, err)
	t.Cleanup(func() { _ = idx.Close() })
	s := memstore.New(memstore.WithObserver(idx))
	for _, id := range []string{"REQ-1", "REQ-2", "DOC-1"} {
		typ := "requirement"
		if id == "DOC-1" {
			typ = "doc"
		}
		e := entity.New(id, typ)
		e.SetString("title", "alpha")
		require.NoError(t, s.CreateEntity(context.Background(), e))
	}
	v, err := search.NewVisible(search.New(s, idx), s)
	require.NoError(t, err)

	var seen []string
	q := search.Query{Text: "alpha", World: store.TrivialScope(),
		Admit: func(c []search.Candidate) ([]search.Candidate, error) {
			var out []search.Candidate
			for _, cand := range c {
				seen = append(seen, cand.ID)
				if cand.ID != "REQ-2" {
					out = append(out, cand)
				}
			}
			return out, nil
		}}
	scope := map[string]search.TypeScope{"requirement": {AllowAll: true}}
	var got []string
	for h, err := range v.SearchVisible(context.Background(), q, scope) {
		require.NoError(t, err)
		got = append(got, h.ID)
	}
	require.ElementsMatch(t, []string{"REQ-1", "REQ-2"}, seen, "the caller sees only what the scope admitted")
	require.Equal(t, []string{"REQ-1"}, got)
}
