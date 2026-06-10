package storetest

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/search"
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

	// Ordered property filters are unsupported on the search backend: it
	// matches raw stringified values with no property-type context, so an
	// ordered comparison could only be lexicographic ("10" < "9"), which
	// is silently wrong for integer/date properties. The contract is now
	// "reject up front" rather than "compare lexicographically". Typed
	// ordering belongs on the metamodel-aware filter path.
	for _, op := range []struct {
		name string
		op   search.FilterOp
	}{
		{"FilterGt", search.FilterGt},
		{"FilterGte", search.FilterGte},
		{"FilterLt", search.FilterLt},
		{"FilterLte", search.FilterLte},
	} {
		t.Run("OrderedFilterUnsupported/"+op.name, func(t *testing.T) {
			s, searcher := sf(t)
			seedSearchData(t, s)

			err := searchError(searcher.Search(ctx(), search.Query{
				Filters: []search.PropertyFilter{
					{Property: "priority", Value: "high", Op: op.op},
				},
			}))
			require.ErrorIs(t, err, search.ErrOrderedFilterUnsupported)
		})
	}
}
