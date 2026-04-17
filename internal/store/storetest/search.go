package storetest

import (
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/search"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// RunSearchTests runs search and filter conformance tests.
func RunSearchTests(t *testing.T, sf SearchFactory) {
	t.Run("TextMatchTitle", func(t *testing.T) {
		s, searcher := sf(t)
		seedSearchData(t, s)

		results := collectHits(t, searcher.Search(ctx(), search.Query{Text: "login"}))
		assert.Len(t, results, 2) // FEAT-001 title + REQ-001 content
	})

	t.Run("TextMatchContent", func(t *testing.T) {
		s, searcher := sf(t)
		seedSearchData(t, s)

		results := collectHits(t, searcher.Search(ctx(), search.Query{Text: "accessing the system"}))
		assert.Len(t, results, 1)
		assert.Equal(t, "REQ-001", results[0].ID)
	})

	t.Run("TextMatchID", func(t *testing.T) {
		s, searcher := sf(t)
		seedSearchData(t, s)

		results := collectHits(t, searcher.Search(ctx(), search.Query{Text: "FEAT-002"}))
		assert.Len(t, results, 1)
		assert.Equal(t, "FEAT-002", results[0].ID)
	})

	t.Run("FilterByType", func(t *testing.T) {
		s, searcher := sf(t)
		seedSearchData(t, s)

		results := collectHits(t, searcher.Search(ctx(), search.Query{Types: []string{"feature"}}))
		assert.Len(t, results, 2)
	})

	t.Run("FilterEq", func(t *testing.T) {
		s, searcher := sf(t)
		seedSearchData(t, s)

		results := collectHits(t, searcher.Search(ctx(), search.Query{
			Filters: []search.PropertyFilter{
				{Property: "status", Value: "open", Op: search.FilterEq},
			},
		}))
		assert.Len(t, results, 2)
	})

	t.Run("FilterNe", func(t *testing.T) {
		s, searcher := sf(t)
		seedSearchData(t, s)

		results := collectHits(t, searcher.Search(ctx(), search.Query{
			Filters: []search.PropertyFilter{
				{Property: "status", Value: "open", Op: search.FilterNe},
			},
		}))
		assert.Len(t, results, 1)
		assert.Equal(t, "FEAT-002", results[0].ID)
	})

	t.Run("FilterContains", func(t *testing.T) {
		s, searcher := sf(t)
		seedSearchData(t, s)

		results := collectHits(t, searcher.Search(ctx(), search.Query{
			Filters: []search.PropertyFilter{
				{Property: "title", Value: "user", Op: search.FilterContains},
			},
		}))
		assert.Len(t, results, 2)
	})

	t.Run("FilterIn", func(t *testing.T) {
		s, searcher := sf(t)
		seedSearchData(t, s)

		results := collectHits(t, searcher.Search(ctx(), search.Query{
			Filters: []search.PropertyFilter{
				{Property: "priority", Value: "high,critical", Op: search.FilterIn},
			},
		}))
		assert.Len(t, results, 1)
		assert.Equal(t, "FEAT-001", results[0].ID)
	})

	t.Run("FilterExists", func(t *testing.T) {
		s, searcher := sf(t)
		seedSearchData(t, s)

		results := collectHits(t, searcher.Search(ctx(), search.Query{
			Filters: []search.PropertyFilter{
				{Property: "priority", Op: search.FilterExists},
			},
		}))
		assert.Len(t, results, 2)
	})

	t.Run("FilterNotExists", func(t *testing.T) {
		s, searcher := sf(t)
		seedSearchData(t, s)

		results := collectHits(t, searcher.Search(ctx(), search.Query{
			Filters: []search.PropertyFilter{
				{Property: "priority", Op: search.FilterNotExists},
			},
		}))
		assert.Len(t, results, 1)
		assert.Equal(t, "REQ-001", results[0].ID)
	})

	t.Run("CombinedTextAndFilter", func(t *testing.T) {
		s, searcher := sf(t)
		seedSearchData(t, s)

		results := collectHits(t, searcher.Search(ctx(), search.Query{
			Text: "login",
			Filters: []search.PropertyFilter{
				{Property: "status", Value: "open", Op: search.FilterEq},
			},
		}))
		assert.Len(t, results, 2)
	})

	t.Run("CombinedTextAndFilterNarrows", func(t *testing.T) {
		s, searcher := sf(t)
		seedSearchData(t, s)

		results := collectHits(t, searcher.Search(ctx(), search.Query{
			Text:  "login",
			Types: []string{"feature"},
		}))
		assert.Len(t, results, 1)
		assert.Equal(t, "FEAT-001", results[0].ID)
	})

	t.Run("Limit", func(t *testing.T) {
		s, searcher := sf(t)
		seedSearchData(t, s)

		results := collectHits(t, searcher.Search(ctx(), search.Query{Limit: 1}))
		assert.Len(t, results, 1)
	})

	t.Run("LimitExactMatch", func(t *testing.T) {
		s, searcher := sf(t)
		seedSearchData(t, s)

		results := collectHits(t, searcher.Search(ctx(), search.Query{Limit: 3}))
		assert.Len(t, results, 3)
	})

	t.Run("EmptyQueryReturnsAll", func(t *testing.T) {
		s, searcher := sf(t)
		seedSearchData(t, s)

		results := collectHits(t, searcher.Search(ctx(), search.Query{}))
		assert.Len(t, results, 3)
	})

	t.Run("EarlyBreak", func(t *testing.T) {
		s, searcher := sf(t)
		seedSearchData(t, s)

		var ids []string
		for h, err := range searcher.Search(ctx(), search.Query{}) {
			require.NoError(t, err)
			ids = append(ids, h.ID)
			if len(ids) == 1 {
				break
			}
		}
		assert.Len(t, ids, 1)
	})

	t.Run("NoMatch", func(t *testing.T) {
		s, searcher := sf(t)
		seedSearchData(t, s)

		results := collectHits(t, searcher.Search(ctx(), search.Query{Text: "zzzznotfound"}))
		assert.Empty(t, results)
	})

	t.Run("FilterGt", func(t *testing.T) {
		s, searcher := sf(t)
		seedSearchData(t, s)

		results := collectHits(t, searcher.Search(ctx(), search.Query{
			Filters: []search.PropertyFilter{
				{Property: "priority", Value: "high", Op: search.FilterGt},
			},
		}))
		// "low" > "high" lexicographically
		assert.Len(t, results, 1)
		assert.Equal(t, "FEAT-002", results[0].ID)
	})

	t.Run("FilterGte", func(t *testing.T) {
		s, searcher := sf(t)
		seedSearchData(t, s)

		results := collectHits(t, searcher.Search(ctx(), search.Query{
			Filters: []search.PropertyFilter{
				{Property: "priority", Value: "high", Op: search.FilterGte},
			},
		}))
		assert.Len(t, results, 2)
	})

	t.Run("FilterLtExcludesEqual", func(t *testing.T) {
		s, searcher := sf(t)
		e1 := entity.New("A", "t")
		e1.SetString("score", "5")
		e2 := entity.New("B", "t")
		e2.SetString("score", "3")
		e3 := entity.New("C", "t")
		e3.SetString("score", "5")
		require.NoError(t, s.CreateEntity(ctx(), e1))
		require.NoError(t, s.CreateEntity(ctx(), e2))
		require.NoError(t, s.CreateEntity(ctx(), e3))

		results := collectHits(t, searcher.Search(ctx(), search.Query{
			Filters: []search.PropertyFilter{
				{Property: "score", Value: "5", Op: search.FilterLt},
			},
		}))
		assert.Len(t, results, 1)
		assert.Equal(t, "B", results[0].ID)
	})

	t.Run("FilterLteIncludesEqual", func(t *testing.T) {
		s, searcher := sf(t)
		e1 := entity.New("A", "t")
		e1.SetString("score", "5")
		e2 := entity.New("B", "t")
		e2.SetString("score", "3")
		e3 := entity.New("C", "t")
		e3.SetString("score", "7")
		require.NoError(t, s.CreateEntity(ctx(), e1))
		require.NoError(t, s.CreateEntity(ctx(), e2))
		require.NoError(t, s.CreateEntity(ctx(), e3))

		results := collectHits(t, searcher.Search(ctx(), search.Query{
			Filters: []search.PropertyFilter{
				{Property: "score", Value: "5", Op: search.FilterLte},
			},
		}))
		assert.Len(t, results, 2)
		ids := []string{results[0].ID, results[1].ID}
		assert.Contains(t, ids, "A")
		assert.Contains(t, ids, "B")
	})
}
