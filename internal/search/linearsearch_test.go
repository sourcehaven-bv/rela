package search

import (
	"reflect"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/entity"
)

// seedLinear builds a LinearSearch whose entities all match the query
// "match" via a shared content substring, with natural-sort-significant
// IDs (REQ-2 must sort before REQ-10).
func seedLinear(t *testing.T) *LinearSearch {
	t.Helper()
	l := NewLinearSearch()
	for _, id := range []string{"REQ-10", "REQ-2", "REQ-1", "REQ-21", "REQ-3"} {
		e := entity.New(id, "requirement")
		e.Content = "this is a match"
		if err := l.EntityPut(e); err != nil {
			t.Fatalf("EntityPut(%s): %v", id, err)
		}
	}
	return l
}

// TestLinearSearch_DeterministicOrder pins that repeated identical
// queries return the same, natural-sort-ordered result — not an
// arbitrary permutation of the backing map's randomized iteration.
func TestLinearSearch_DeterministicOrder(t *testing.T) {
	l := seedLinear(t)
	want := []string{"REQ-1", "REQ-2", "REQ-3", "REQ-10", "REQ-21"}

	// Run many times: map iteration order is randomized per range, so a
	// non-deterministic implementation would diverge within a few runs.
	for i := range 50 {
		got, err := l.Search("match", 0)
		if err != nil {
			t.Fatalf("Search: %v", err)
		}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("run %d: got %v, want %v (natural-sort order, stable)", i, got, want)
		}
	}
}

// TestLinearSearch_LimitReturnsFirstN pins that a limit returns the
// first-N of the defined order, deterministically — not an arbitrary
// subset truncated mid-iteration over the map.
func TestLinearSearch_LimitReturnsFirstN(t *testing.T) {
	l := seedLinear(t)
	want := []string{"REQ-1", "REQ-2", "REQ-3"}

	for i := range 50 {
		got, err := l.Search("match", 3)
		if err != nil {
			t.Fatalf("Search: %v", err)
		}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("run %d: got %v, want %v (first 3 in natural-sort order)", i, got, want)
		}
	}
}

// TestLinearSearch_NoMatchesEmpty pins the empty-result shape.
func TestLinearSearch_NoMatchesEmpty(t *testing.T) {
	l := seedLinear(t)
	got, err := l.Search("nonexistent-term", 0)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("got %v, want no results", got)
	}
}
