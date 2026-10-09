package pgstore_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/store/pgstore"
)

// TestContentHashTriggers pins migration 0020's triggers: a stored content_hash
// survives a write that changes nothing hashed, and is cleared by one that
// does, on entities and relations (BUG-1DWMYO). The version sweep selects on
// that column, so a hash left behind by a content change would hide the edit.
func TestContentHashTriggers(t *testing.T) {
	pool := newScopedPool(t)
	s, err := pgstore.New(pool)
	require.NoError(t, err)
	t.Cleanup(func() { _ = s.Close() })
	ctx := context.Background()

	e := entity.New("FEAT-1", "feature")
	e.SetString("title", "v1")
	require.NoError(t, s.CreateEntity(ctx, e))
	require.NoError(t, s.CreateEntity(ctx, entity.New("FEAT-2", "feature")))
	key := entity.RelationKey{From: "FEAT-1", Type: "depends-on", To: "FEAT-2"}
	_, err = s.CreateRelation(ctx, key, &store.RelationData{Content: "v1"})
	require.NoError(t, err)

	readHash := func(q string) *string {
		t.Helper()
		var h *string
		require.NoError(t, pool.QueryRow(ctx, q).Scan(&h))
		return h
	}
	const entityHash = `SELECT content_hash FROM entities WHERE id = 'FEAT-1'`
	const relationHash = `SELECT content_hash FROM relations WHERE from_id = 'FEAT-1'`
	require.Nil(t, readHash(entityHash), "a new row's hash is not known")

	_, err = pool.Exec(ctx, `UPDATE entities SET content_hash = 'h' WHERE id = 'FEAT-1'`)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `UPDATE relations SET content_hash = 'h' WHERE from_id = 'FEAT-1'`)
	require.NoError(t, err)

	require.NoError(t, s.UpdateEntity(ctx, e))
	_, err = s.UpdateRelation(ctx, key, store.RelationData{Content: "v1"})
	require.NoError(t, err)
	require.NotNil(t, readHash(entityHash), "an unchanged save must keep the entity hash")
	require.NotNil(t, readHash(relationHash), "an unchanged save must keep the relation hash")

	e.SetString("title", "v2")
	require.NoError(t, s.UpdateEntity(ctx, e))
	_, err = s.UpdateRelation(ctx, key, store.RelationData{Content: "v2"})
	require.NoError(t, err)
	require.Nil(t, readHash(entityHash), "a property change must clear the entity hash")
	require.Nil(t, readHash(relationHash), "a content change must clear the relation hash")
}

// TestSweepWriteBackSkipsARowWrittenAfterTheRead pins the xmin guard on the
// write-back (BUG-1DWMYO). The row's stored hash is NULL and its content
// matches its latest version, so the tick computes that version's hash and
// writes it back. An edit lands between the candidate query and the
// write-back. Stored on the edited row, the old hash would mark it clean and
// the edit would never be captured.
func TestSweepWriteBackSkipsARowWrittenAfterTheRead(t *testing.T) {
	pool := newScopedPool(t)
	s, err := pgstore.New(pool)
	require.NoError(t, err)
	t.Cleanup(func() { _ = s.Close() })
	ctx := context.Background()
	provider := stubProvider{hash: "schema-1", json: []byte(`{"v":1}`)}
	cfg := store.SweepConfig{Interval: time.Hour, Idle: time.Nanosecond, MaxStaleness: time.Nanosecond, Batch: 10}

	e := entity.New("FEAT-1", "feature")
	e.SetString("title", "v1")
	require.NoError(t, s.CreateEntity(ctx, e))
	require.NoError(t, s.SweepNow(ctx, provider, cfg))
	_, err = pool.Exec(ctx, `UPDATE entities SET content_hash = NULL`)
	require.NoError(t, err)

	require.NoError(t, s.SweepNowWithWrite(ctx, provider, cfg, func() {
		e.SetString("title", "v2")
		require.NoError(t, s.UpdateEntity(ctx, e))
	}))
	require.NoError(t, s.SweepNow(ctx, provider, cfg))

	metas, err := s.VersionStore().ListVersions(ctx, entity.Ref{ID: "FEAT-1"})
	require.NoError(t, err)
	require.Len(t, metas, 2, "the edit made during the tick was never captured")
}
