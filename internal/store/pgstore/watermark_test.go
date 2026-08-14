package pgstore_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/store/pgstore"
)

// TestEntityTypeWatermark_MovesOnWrite pins the basic contract: the watermark
// changes on create and on update, so a poller can detect either.
func TestEntityTypeWatermark_MovesOnWrite(t *testing.T) {
	st := newTombstoneStore(t)
	ctx := context.Background()

	empty, err := st.EntityTypeWatermark(ctx, "requirement")
	require.NoError(t, err)
	require.Zero(t, empty, "a type with no rows reports 0, not an error")

	mustCreateEntity(t, st, "REQ-1", "requirement")
	afterCreate, err := st.EntityTypeWatermark(ctx, "requirement")
	require.NoError(t, err)
	require.Greater(t, afterCreate, empty, "create must move the watermark")

	e, err := st.GetEntity(ctx, "REQ-1")
	require.NoError(t, err)
	e.SetString("title", "changed")
	require.NoError(t, st.UpdateEntity(ctx, e))

	afterUpdate, err := st.EntityTypeWatermark(ctx, "requirement")
	require.NoError(t, err)
	require.Greater(t, afterUpdate, afterCreate, "update must move the watermark")
}

// TestEntityTypeWatermark_DeleteDoesNotGoBackwards is the one that matters.
//
// Deletes are HARD, so max(seq) over live rows alone DROPS when the newest row
// is removed. A tag that moves backwards makes a client which already saw the
// higher value stop polling — permanently stale with no way to notice. The
// deletion-tombstone half of the query exists solely to prevent this.
func TestEntityTypeWatermark_DeleteDoesNotGoBackwards(t *testing.T) {
	st := newTombstoneStore(t)
	ctx := context.Background()

	mustCreateEntity(t, st, "REQ-1", "requirement")
	mustCreateEntity(t, st, "REQ-2", "requirement")
	beforeDelete, err := st.EntityTypeWatermark(ctx, "requirement")
	require.NoError(t, err)

	// Delete the NEWEST row: without tombstones this is the case that regresses.
	_, err = st.DeleteEntity(ctx, "REQ-2", false)
	require.NoError(t, err)

	afterDelete, err := st.EntityTypeWatermark(ctx, "requirement")
	require.NoError(t, err)
	require.Greater(t, afterDelete, beforeDelete,
		"deleting the newest row must ADVANCE the watermark, not lower it — "+
			"a backwards tag strands every client that saw the higher value")
}

// TestEntityTypeWatermark_ScopedByType pins that one type's churn does not move
// another's. Over-triggering WITHIN a type is accepted; across types it would
// make the watermark useless.
func TestEntityTypeWatermark_ScopedByType(t *testing.T) {
	st := newTombstoneStore(t)
	ctx := context.Background()

	mustCreateEntity(t, st, "REQ-1", "requirement")
	other, err := st.EntityTypeWatermark(ctx, "decision")
	require.NoError(t, err)
	require.Zero(t, other, "a write to `requirement` must not move `decision`")

	mustCreateEntity(t, st, "DEC-1", "decision")
	reqBefore, err := st.EntityTypeWatermark(ctx, "requirement")
	require.NoError(t, err)

	mustCreateEntity(t, st, "DEC-2", "decision")
	reqAfter, err := st.EntityTypeWatermark(ctx, "requirement")
	require.NoError(t, err)
	require.Equal(t, reqBefore, reqAfter, "`decision` churn must not move `requirement`")
}

// TestEntityTypeWatermark_RelationDeleteDoesNotMoveEntityType pins the `kind`
// predicate: deletions holds entity AND relation tombstones in one table, so an
// entity-type watermark that ignored kind would be moved by unrelated relation
// deletes.
func TestEntityTypeWatermark_RelationDeleteDoesNotMoveEntityType(t *testing.T) {
	st := newTombstoneStore(t)
	ctx := context.Background()

	mustCreateEntity(t, st, "REQ-1", "requirement")
	mustCreateEntity(t, st, "REQ-2", "requirement")
	_, err := st.CreateRelation(ctx, "REQ-1", "relates-to", "REQ-2", nil)
	require.NoError(t, err)

	before, err := st.EntityTypeWatermark(ctx, "requirement")
	require.NoError(t, err)

	require.NoError(t, st.DeleteRelation(ctx, "REQ-1", "relates-to", "REQ-2"))

	after, err := st.EntityTypeWatermark(ctx, "requirement")
	require.NoError(t, err)
	require.Equal(t, before, after,
		"a RELATION delete must not move the ENTITY-type watermark (kind='e' predicate)")
}

// The capability is optional and type-asserted by callers; pin that pgstore
// actually satisfies it so a signature drift fails here rather than silently
// disabling the optimization at every call site.
var _ store.TypeWatermark = (*pgstore.Store)(nil)
