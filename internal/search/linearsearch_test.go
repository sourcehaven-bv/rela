package search_test

import (
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/search"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// TestLinearSearch_EntityRenamed pins the single-event rename
// contract on the in-memory backend: EntityRenamed must drop the
// old key and insert the renamed entity in one critical section so
// concurrent readers never observe both — or neither.
func TestLinearSearch_EntityRenamed(t *testing.T) {
	idx := search.NewLinearSearch()

	old := entity.New("REQ-1", "requirement")
	old.SetString("title", "Single-event rename")
	require.NoError(t, idx.EntityPut(old))

	renamed := entity.New("REQ-99", "requirement")
	renamed.SetString("title", "Single-event rename")
	require.NoError(t, idx.EntityRenamed("REQ-1", renamed))

	// Search by the title text. The old ID must be gone from the
	// index; the new ID must be findable.
	ids := searchIDs(t, idx, "Single-event", 10)
	assert.Contains(t, ids, "REQ-99",
		"renamed entity should be findable under the new ID")
	assert.NotContains(t, ids, "REQ-1",
		"old ID must be removed from the index after rename")
}

// seedLinear builds a LinearSearch whose entities all match the query
// "match" via a shared content substring, with natural-sort-significant
// IDs (REQ-2 must sort before REQ-10).
func seedLinear(t *testing.T) *search.LinearSearch {
	t.Helper()
	l := search.NewLinearSearch()
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
		got := searchIDs(t, l, "match", 0)
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
		got := searchIDs(t, l, "match", 3)
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("run %d: got %v, want %v (first 3 in natural-sort order)", i, got, want)
		}
	}
}

// TestLinearSearch_NoMatchesEmpty pins the empty-result shape.
func TestLinearSearch_NoMatchesEmpty(t *testing.T) {
	l := seedLinear(t)
	got := searchIDs(t, l, "nonexistent-term", 0)
	if len(got) != 0 {
		t.Errorf("got %v, want no results", got)
	}
}

// searchIDs runs a DEFAULT-WORLD search and returns just the ids, which is
// what the pre-worlds assertions in this file are written against.
//
// The world-scoped behavior has its own tests (world_test.go); these keep
// asserting the properties that must hold regardless of worlds — ordering,
// limits, rename/delete eviction — without every one of them restating the
// default world.
func searchIDs(t *testing.T, b search.Backend, text string, limit int) []string {
	t.Helper()
	faces, err := b.Search(text, limit, store.DefaultWorld())
	if err != nil {
		t.Fatalf("Search(%q, %d): %v", text, limit, err)
	}
	ids := make([]string, 0, len(faces))
	for _, f := range faces {
		ids = append(ids, f.ID)
	}
	return ids
}
