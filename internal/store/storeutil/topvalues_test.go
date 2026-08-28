package storeutil_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/store/storeutil"
)

func TestTopValues(t *testing.T) {
	counts := map[string]int{
		"open":   5,
		"done":   5, // ties with "open" — alphabetical wins
		"closed": 9,
		"draft":  1,
	}

	tests := []struct {
		name   string
		counts map[string]int
		limit  int
		want   []string
	}{
		{
			name:   "ranks by count then alphabetically",
			counts: counts,
			limit:  0,
			want:   []string{"closed", "done", "open", "draft"},
		},
		{
			name:   "limit truncates after ranking",
			counts: counts,
			limit:  2,
			want:   []string{"closed", "done"},
		},
		{
			// The convention the three backends disagreed on: limit <= 0 means
			// "all values", NOT "none". A backend reading it as a cap would
			// return an empty slice here.
			name:   "negative limit means all",
			counts: counts,
			limit:  -1,
			want:   []string{"closed", "done", "open", "draft"},
		},
		{
			name:   "limit larger than the set returns everything",
			counts: counts,
			limit:  99,
			want:   []string{"closed", "done", "open", "draft"},
		},
		{
			name:   "empty counts yields an empty, non-nil slice",
			counts: map[string]int{},
			limit:  10,
			want:   []string{},
		},
		{
			name:   "nil counts yields an empty, non-nil slice",
			counts: nil,
			limit:  10,
			want:   []string{},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := storeutil.TopValues(tc.counts, tc.limit)
			require.Equal(t, tc.want, got)
			require.NotNil(t, got, "callers marshal this straight to JSON; nil would encode as null, not []")
		})
	}
}

// TestTopValuesIsDeterministic guards the reason ties are broken
// alphabetically at all: Go randomizes map iteration order, so without the
// secondary sort key two runs over the same data could return different
// orders — and the backends would disagree with each other.
func TestTopValuesIsDeterministic(t *testing.T) {
	counts := map[string]int{"a": 1, "b": 1, "c": 1, "d": 1, "e": 1}

	first := storeutil.TopValues(counts, 0)
	for range 50 {
		require.Equal(t, first, storeutil.TopValues(counts, 0))
	}
	require.Equal(t, []string{"a", "b", "c", "d", "e"}, first)
}

// TestTopValuesAllocatesForTheResult pins the drift this extraction fixed.
// fsstore and memstore sized the result slice to `limit`, so the unlimited
// case (limit == 0) pre-allocated ZERO and grew by repeated reallocation;
// pgstore sized it correctly. Capacity is the observable difference.
func TestTopValuesAllocatesForTheResult(t *testing.T) {
	counts := map[string]int{}
	for i := range 100 {
		counts[string(rune('a'+i%26))+string(rune('a'+i/26))] = i
	}

	got := storeutil.TopValues(counts, 0)
	require.Len(t, got, len(counts))
	require.Equal(t, len(counts), cap(got),
		"unlimited TopValues must pre-allocate for the whole result; sizing to "+
			"limit (0) reallocates on every append")
}
