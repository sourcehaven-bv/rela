package search_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/search"
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
	ids, err := idx.Search("Single-event", 10)
	require.NoError(t, err)
	assert.Contains(t, ids, "REQ-99",
		"renamed entity should be findable under the new ID")
	assert.NotContains(t, ids, "REQ-1",
		"old ID must be removed from the index after rename")
}
