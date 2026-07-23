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

// seedEntityVersions creates a live entity and writes N captured versions for it,
// returning the store. Content varies per version so content hashes differ.
func seedEntityHistory(t *testing.T, s *pgstore.Store, id string, contents ...string) {
	t.Helper()
	ctx := context.Background()
	require.NoError(t, s.CreateEntity(ctx, mkEntity(id, "ticket", contents[len(contents)-1])))
	for i, c := range contents {
		in := newVersionInput(id, c, map[string]any{"n": i})
		if i == 0 {
			in.Op = store.VersionOpCreate
		} else {
			in.Op = store.VersionOpUpdate
		}
		require.NoError(t, s.VersionStore().WriteVersion(ctx, in))
	}
}

// TestPurgeRefusesWhenLiveRowExists is the RR-SH28E guard: with a live row and no
// ForceLive, purge deletes NOTHING and flags LiveRowExists.
func TestPurgeRefusesWhenLiveRowExists(t *testing.T) {
	s, err := pgstore.New(newScopedPool(t))
	require.NoError(t, err)
	t.Cleanup(func() { _ = s.Close() })
	ctx := context.Background()
	seedEntityHistory(t, s, "TKT-1", "v1", "v2secret")

	res, err := s.VersionStore().PurgeVersions(ctx, store.VersionPurgeRequest{
		EntityID: "TKT-1", Selector: store.PurgeSelector{All: true}, Reason: "test",
	})
	require.NoError(t, err)
	require.True(t, res.LiveRowExists)
	require.Equal(t, 0, res.Purged, "must refuse to purge while live row exists")

	metas, err := s.VersionStore().ListVersions(ctx, "TKT-1")
	require.NoError(t, err)
	require.Len(t, metas, 2, "history untouched by refused purge")
}

// TestPurgeForceLiveWritesTombstone: with ForceLive, purge deletes the rows AND
// writes a no-content `purge` tombstone (so the sweep won't re-capture).
func TestPurgeForceLiveWritesTombstone(t *testing.T) {
	s, err := pgstore.New(newScopedPool(t))
	require.NoError(t, err)
	t.Cleanup(func() { _ = s.Close() })
	ctx := context.Background()
	seedEntityHistory(t, s, "TKT-1", "v1", "v2secret")

	res, err := s.VersionStore().PurgeVersions(ctx, store.VersionPurgeRequest{
		EntityID: "TKT-1", Selector: store.PurgeSelector{All: true},
		Reason: "erase PII", ForceLive: true,
	})
	require.NoError(t, err)
	require.Equal(t, 2, res.Purged)
	require.True(t, res.TombstoneWritten)

	// The 2 content versions are gone; only the tombstone remains.
	metas, err := s.VersionStore().ListVersions(ctx, "TKT-1")
	require.NoError(t, err)
	require.Len(t, metas, 1)
	require.Equal(t, store.VersionOpPurge, metas[0].Op)
}

// TestPurgeDeletedEntityAll: a deleted entity (no live row) purges cleanly with
// no tombstone, no refusal.
func TestPurgeDeletedEntityAll(t *testing.T) {
	s, err := pgstore.New(newScopedPool(t))
	require.NoError(t, err)
	t.Cleanup(func() { _ = s.Close() })
	ctx := context.Background()
	seedEntityHistory(t, s, "TKT-1", "v1", "v2")
	// Write a delete version + remove the live row.
	del := newVersionInput("TKT-1", "v2", nil)
	del.Op = store.VersionOpDelete
	require.NoError(t, s.VersionStore().WriteVersion(ctx, del))
	_, err = s.DeleteEntity(ctx, "TKT-1", false)
	require.NoError(t, err)

	res, err := s.VersionStore().PurgeVersions(ctx, store.VersionPurgeRequest{
		EntityID: "TKT-1", Selector: store.PurgeSelector{All: true}, Reason: "erase deleted",
	})
	require.NoError(t, err)
	require.False(t, res.LiveRowExists)
	require.False(t, res.TombstoneWritten)
	require.Equal(t, 3, res.Purged) // create + update + delete

	metas, err := s.VersionStore().ListVersions(ctx, "TKT-1")
	require.NoError(t, err)
	require.Empty(t, metas, "all history purged")
}

// TestPurgeByVseq purges exactly one row by its stable vseq.
func TestPurgeByVseq(t *testing.T) {
	pool := newScopedPool(t)
	s, err := pgstore.New(pool)
	require.NoError(t, err)
	t.Cleanup(func() { _ = s.Close() })
	ctx := context.Background()
	seedEntityHistory(t, s, "TKT-1", "v1", "v2", "v3")
	_, err = s.DeleteEntity(ctx, "TKT-1", false) // no live row so purge doesn't refuse
	require.NoError(t, err)

	// Grab the middle version's vseq directly.
	var vseq int64
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT vseq FROM entity_versions WHERE entity_id='TKT-1' ORDER BY vseq ASC OFFSET 1 LIMIT 1`).Scan(&vseq))

	res, err := s.VersionStore().PurgeVersions(ctx, store.VersionPurgeRequest{
		EntityID: "TKT-1", Selector: store.PurgeSelector{Vseq: vseq}, Reason: "single",
	})
	require.NoError(t, err)
	require.Equal(t, 1, res.Purged)

	metas, err := s.VersionStore().ListVersions(ctx, "TKT-1")
	require.NoError(t, err)
	require.Len(t, metas, 2, "only the one vseq purged")
}

// TestPurgeByContentHash purges every row matching a content hash (the GDPR op).
func TestPurgeByContentHash(t *testing.T) {
	pool := newScopedPool(t)
	s, err := pgstore.New(pool)
	require.NoError(t, err)
	t.Cleanup(func() { _ = s.Close() })
	ctx := context.Background()
	// Write three versions directly so two share EXACTLY the same content AND
	// properties (hence the same content_hash), and one differs.
	require.NoError(t, s.CreateEntity(ctx, mkEntity("TKT-1", "ticket", "dup")))
	for i, c := range []struct {
		content string
		props   map[string]any
	}{
		{"dup", map[string]any{"k": "same"}},
		{"other", map[string]any{"k": "diff"}},
		{"dup", map[string]any{"k": "same"}}, // identical to row 0
	} {
		in := newVersionInput("TKT-1", c.content, c.props)
		if i == 0 {
			in.Op = store.VersionOpCreate
		} else {
			in.Op = store.VersionOpUpdate
		}
		require.NoError(t, s.VersionStore().WriteVersion(ctx, in))
	}
	_, err = s.DeleteEntity(ctx, "TKT-1", false)
	require.NoError(t, err)

	var hash string
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT content_hash FROM entity_versions WHERE entity_id='TKT-1' AND content='dup' LIMIT 1`).Scan(&hash))

	res, err := s.VersionStore().PurgeVersions(ctx, store.VersionPurgeRequest{
		EntityID: "TKT-1", Selector: store.PurgeSelector{ContentHash: hash}, Reason: "erase value",
	})
	require.NoError(t, err)
	require.Equal(t, 2, res.Purged, "both dup rows purged")

	var remaining int
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT count(*) FROM entity_versions WHERE entity_id='TKT-1' AND content_hash=$1`, hash).Scan(&remaining))
	require.Zero(t, remaining, "0 rows remain with the hash — verifiable erasure")
}

// TestPurgeRefusesRenameRow is the RR-EQQP1 guard: a target set containing a
// rename row is refused (deletes nothing, flags RenameInTargets).
func TestPurgeRefusesRenameRow(t *testing.T) {
	s, err := pgstore.New(newScopedPool(t))
	require.NoError(t, err)
	t.Cleanup(func() { _ = s.Close() })
	ctx := context.Background()
	// Old A: create+update, rename A->B, then delete B so no live row.
	seedEntityHistory(t, s, "A", "a1", "a2")
	ren := newVersionInput("B", "a2", nil)
	ren.Op = store.VersionOpRename
	ren.PrevID = "A"
	require.NoError(t, s.VersionStore().WriteVersion(ctx, ren))

	// Purge --all of B's lineage includes the rename row → refuse.
	res, err := s.VersionStore().PurgeVersions(ctx, store.VersionPurgeRequest{
		EntityID: "B", Selector: store.PurgeSelector{All: true}, Reason: "test",
	})
	require.NoError(t, err)
	require.True(t, res.RenameInTargets, "rename row in target set")
	require.Equal(t, 0, res.Purged, "must refuse")
}

// TestPurgeDryRun resolves targets without deleting.
func TestPurgeDryRun(t *testing.T) {
	s, err := pgstore.New(newScopedPool(t))
	require.NoError(t, err)
	t.Cleanup(func() { _ = s.Close() })
	ctx := context.Background()
	seedEntityHistory(t, s, "TKT-1", "v1", "v2")
	_, err = s.DeleteEntity(ctx, "TKT-1", false)
	require.NoError(t, err)

	res, err := s.VersionStore().PurgeVersions(ctx, store.VersionPurgeRequest{
		EntityID: "TKT-1", Selector: store.PurgeSelector{All: true}, Reason: "preview", DryRun: true,
	})
	require.NoError(t, err)
	require.Equal(t, 0, res.Purged, "dry-run deletes nothing")
	require.Len(t, res.Targets, 2, "but resolves the targets")

	metas, err := s.VersionStore().ListVersions(ctx, "TKT-1")
	require.NoError(t, err)
	require.Len(t, metas, 2, "history intact after dry-run")
}

// TestPurgeTombstoneSuppressesSweepRecapture is the RR-SH28E end-to-end proof:
// after a force-live --all purge, the sweep must NOT re-capture the live content
// (the tombstone's content_hash == live hash dedups). Without the tombstone the
// sweep would re-insert the PII within one interval.
func TestPurgeTombstoneSuppressesSweepRecapture(t *testing.T) {
	pool := newScopedPool(t)
	s, err := pgstore.New(pool)
	require.NoError(t, err)
	t.Cleanup(func() { _ = s.Close() })
	ctx := context.Background()
	seedEntityHistory(t, s, "TKT-1", "v1", "secret-live")

	// Backdate updated_at so the entity is settled → sweep-eligible.
	_, err = pool.Exec(ctx,
		`UPDATE entities SET updated_at = now() - interval '1 hour' WHERE id='TKT-1'`)
	require.NoError(t, err)

	// Force-live purge all history; tombstone written.
	res, err := s.VersionStore().PurgeVersions(ctx, store.VersionPurgeRequest{
		EntityID: "TKT-1", Selector: store.PurgeSelector{All: true},
		Reason: "erase", ForceLive: true,
	})
	require.NoError(t, err)
	require.True(t, res.TombstoneWritten)

	// Drive the sweep. If the tombstone works, it re-captures NOTHING (the live
	// hash matches the tombstone's content_hash).
	s.StartVersionSweep(stubProvider{hash: "schema-abc", json: []byte(`{"entities":{},"types":{}}`)},
		pgstore.SweepConfig{Interval: 50 * time.Millisecond, Idle: time.Minute, MaxStaleness: time.Hour, Batch: 100})
	time.Sleep(300 * time.Millisecond)

	metas, err := s.VersionStore().ListVersions(ctx, "TKT-1")
	require.NoError(t, err)
	require.Len(t, metas, 1, "only the purge tombstone; sweep did NOT re-capture the live PII")
	require.Equal(t, store.VersionOpPurge, metas[0].Op)
}

// Guard against unused import when entity pkg drifts.
var _ = entity.New
