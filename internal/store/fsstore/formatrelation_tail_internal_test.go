package fsstore

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// TestFormatRelation_FacedTail pins that FormatRelation reads and rewrites
// the file of the addressed tail (TKT-KQXVF7), not the identity edge's.
func TestFormatRelation_FacedTail(t *testing.T) {
	ctx := context.Background()
	s, _ := newTestStore(t)
	for _, e := range []*entity.Entity{
		{ID: "REQ-1", Type: "requirement", Face: "draft"},
		{ID: "SOL-1", Type: "solution"},
	} {
		require.NoError(t, s.CreateEntity(ctx, e))
	}
	k := entity.RelationKey{From: "REQ-1", FromFace: "draft", Type: "solves", To: "SOL-1"}
	_, err := s.CreateRelation(ctx, k, &store.RelationData{Content: "body"})
	require.NoError(t, err)

	changed, err := s.FormatRelation(ctx, k, true)
	require.NoError(t, err)
	assert.False(t, changed, "a freshly written file is already formatted")

	_, err = s.FormatRelation(ctx, entity.RelationKey{From: "REQ-1", Type: "solves", To: "SOL-1"}, true)
	require.ErrorIs(t, err, store.ErrNotFound, "the identity edge does not exist")
}
