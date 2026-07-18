package pgstore_test

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/store/pgstore"
)

// relRecordID reads the surrogate rel_record_id off a live relation row. Tests
// need it because WriteRelationVersion requires the caller to supply the id that
// the store stamps on the row (mirroring how the real sync/sweep paths read it).
func relRecordID(ctx context.Context, t *testing.T, pool *pgxpool.Pool, from, relType, to string) int64 {
	t.Helper()
	var id int64
	err := pool.QueryRow(ctx,
		`SELECT rel_record_id FROM relations WHERE from_id=$1 AND rel_type=$2 AND to_id=$3`,
		from, relType, to).Scan(&id)
	require.NoError(t, err)
	require.NotZero(t, id)
	return id
}

func newRelVersionInput(recordID int64, from, relType, to, content string) store.RelationVersionInput {
	return store.RelationVersionInput{
		RecordID:      recordID,
		From:          from,
		Type:          relType,
		To:            to,
		Content:       content,
		Properties:    map[string]any{"weight": "high"},
		SchemaHash:    "schema-rel",
		Projection:    []byte(`{"relations":{}}`),
		PrincipalUser: "alice",
		PrincipalTool: "cli",
	}
}

// TestRelationVersionWriteAndList captures two versions of a live relation and
// reads them back with correct ordinals, ops, and attribution.
func TestRelationVersionWriteAndList(t *testing.T) {
	pool := newScopedPool(t)
	s, err := pgstore.New(pool)
	require.NoError(t, err)
	t.Cleanup(func() { _ = s.Close() })
	ctx := context.Background()

	_, err = s.CreateRelation(ctx, "TKT-1", "blocks", "TKT-2",
		&store.RelationData{Content: "first", Properties: map[string]any{"weight": "high"}})
	require.NoError(t, err)
	rid := relRecordID(ctx, t, pool, "TKT-1", "blocks", "TKT-2")

	c := newRelVersionInput(rid, "TKT-1", "blocks", "TKT-2", "first")
	c.Op = store.VersionOpCreate
	require.NoError(t, s.WriteRelationVersion(ctx, c))

	u := newRelVersionInput(rid, "TKT-1", "blocks", "TKT-2", "second")
	u.Op = store.VersionOpUpdate
	require.NoError(t, s.WriteRelationVersion(ctx, u))

	metas, err := s.ListRelationVersions(ctx, "TKT-1", "blocks", "TKT-2")
	require.NoError(t, err)
	require.Len(t, metas, 2)
	require.Equal(t, 1, metas[0].Version)
	require.Equal(t, store.VersionOpCreate, metas[0].Op)
	require.Equal(t, 2, metas[1].Version)
	require.Equal(t, store.VersionOpUpdate, metas[1].Op)
	require.Equal(t, "alice", metas[1].PrincipalUser)

	snap, err := s.GetRelationVersion(ctx, "TKT-1", "blocks", "TKT-2", 1)
	require.NoError(t, err)
	require.Equal(t, "first", snap.Content)
	require.Equal(t, "high", snap.Properties["weight"])
	require.Equal(t, "TKT-1", snap.From)
	require.Equal(t, "TKT-2", snap.To)
}

// TestRelationVersionDeletedKeyResolves proves history survives the deletion of
// the live relation row: recordIDForKey falls back to the most-recent lineage.
func TestRelationVersionDeletedKeyResolves(t *testing.T) {
	pool := newScopedPool(t)
	s, err := pgstore.New(pool)
	require.NoError(t, err)
	t.Cleanup(func() { _ = s.Close() })
	ctx := context.Background()

	_, err = s.CreateRelation(ctx, "A", "links", "B", &store.RelationData{Content: "x"})
	require.NoError(t, err)
	rid := relRecordID(ctx, t, pool, "A", "links", "B")

	c := newRelVersionInput(rid, "A", "links", "B", "x")
	c.Op = store.VersionOpCreate
	require.NoError(t, s.WriteRelationVersion(ctx, c))

	// Capture a delete version (as the sync path will), then remove the live row.
	d := newRelVersionInput(rid, "A", "links", "B", "x")
	d.Op = store.VersionOpDelete
	require.NoError(t, s.WriteRelationVersion(ctx, d))
	require.NoError(t, s.DeleteRelation(ctx, "A", "links", "B"))

	// History must still be readable via the composite key (no live row now).
	metas, err := s.ListRelationVersions(ctx, "A", "links", "B")
	require.NoError(t, err)
	require.Len(t, metas, 2)
	require.Equal(t, store.VersionOpDelete, metas[1].Op)
}

// TestRelationVersionRecreateStartsFreshLineage proves a delete+recreate of the
// same (from,type,to) gets a NEW rel_record_id, so histories are NOT merged.
func TestRelationVersionRecreateStartsFreshLineage(t *testing.T) {
	pool := newScopedPool(t)
	s, err := pgstore.New(pool)
	require.NoError(t, err)
	t.Cleanup(func() { _ = s.Close() })
	ctx := context.Background()

	_, err = s.CreateRelation(ctx, "A", "links", "B", &store.RelationData{Content: "gen1"})
	require.NoError(t, err)
	rid1 := relRecordID(ctx, t, pool, "A", "links", "B")
	c1 := newRelVersionInput(rid1, "A", "links", "B", "gen1")
	c1.Op = store.VersionOpCreate
	require.NoError(t, s.WriteRelationVersion(ctx, c1))
	require.NoError(t, s.DeleteRelation(ctx, "A", "links", "B"))

	// Recreate the same triple — the row gets a fresh rel_record_id.
	_, err = s.CreateRelation(ctx, "A", "links", "B", &store.RelationData{Content: "gen2"})
	require.NoError(t, err)
	rid2 := relRecordID(ctx, t, pool, "A", "links", "B")
	require.NotEqual(t, rid1, rid2, "recreated relation must get a fresh rel_record_id")

	c2 := newRelVersionInput(rid2, "A", "links", "B", "gen2")
	c2.Op = store.VersionOpCreate
	require.NoError(t, s.WriteRelationVersion(ctx, c2))

	// ListRelationVersions resolves to the CURRENT (gen2) lineage only — the
	// gen1 history is not merged in.
	metas, err := s.ListRelationVersions(ctx, "A", "links", "B")
	require.NoError(t, err)
	require.Len(t, metas, 1, "current lineage only; gen1 not merged")
	snap, err := s.GetRelationVersion(ctx, "A", "links", "B", 1)
	require.NoError(t, err)
	require.Equal(t, "gen2", snap.Content)
}

// TestRelationVersionRenameAtomicPath verifies relation-rename versioning
// against the REAL production path (TKT-9TQ6I). Since #1127, the entitymanager
// renames an entity via the store's ATOMIC RenameEntity (a single bulk
// `UPDATE relations SET from_id=$2 ...`), NOT the old rename.Rename
// decomposition (delete-old + create-new). Two consequences this test pins:
//
//  1. The relation row keeps the SAME rel_record_id across the rename (in-place
//     UPDATE, not delete+create). So the lineage is ALREADY continuous on one
//     id — no fork, and the prev_from/prev_to stitch walk is redundant (it just
//     finds no predecessor). The rename `version` the entitymanager records
//     (via WriteRelationVersion with RecordID=0 → resolves the key → the
//     surviving rel_record_id) simply appends to that one lineage.
//  2. The atomic re-key does NOT bump relations.updated_at (RR-N5YK81, now the
//     live path) — see TestRelationRenameDoesNotBumpUpdatedAt. That means the
//     sweep cannot back-fill a missed rename capture; documented as best-effort
//     (the lineage stays continuous regardless, so a missed rename version only
//     loses the rename MARKER, not history continuity).
//
// This is the STORE half of rename-version coverage: it asserts the store
// persists the rename version on the surviving rel_record_id. The MANAGER half —
// that Manager.RenameEntity emits the right record (new triple, prev_from/
// prev_to, per incident edge) — is asserted in the entitymanager package by
// TestRelationVersionHook_RenameStitchesEndpoints. Neither is redundant; keep
// both.
func TestRelationVersionRenameAtomicPath(t *testing.T) {
	pool := newScopedPool(t)
	s, err := pgstore.New(pool)
	require.NoError(t, err)
	t.Cleanup(func() { _ = s.Close() })
	ctx := context.Background()

	// A live edge A--links-->X with a captured create version.
	require.NoError(t, s.CreateEntity(ctx, mkEntity("A", "ticket", "")))
	require.NoError(t, s.CreateEntity(ctx, mkEntity("X", "ticket", "")))
	_, err = s.CreateRelation(ctx, "A", "links", "X", &store.RelationData{Content: "v1"})
	require.NoError(t, err)
	rid := relRecordID(ctx, t, pool, "A", "links", "X")
	c := newRelVersionInput(rid, "A", "links", "X", "v1")
	c.Op = store.VersionOpCreate
	require.NoError(t, s.WriteRelationVersion(ctx, c))

	// Rename A->A2 through the REAL atomic store path.
	_, err = s.RenameEntity(ctx, "A", "A2")
	require.NoError(t, err)

	// The relation kept its rel_record_id — continuous single lineage, no fork.
	ridAfter := relRecordID(ctx, t, pool, "A2", "links", "X")
	require.Equal(t, rid, ridAfter, "atomic rename carries rel_record_id (no fork)")

	// Capture the rename version exactly as the entitymanager does: keyed on the
	// NEW triple, RecordID=0 (WriteRelationVersion resolves it to the surviving
	// rel_record_id), carrying the pre-rename endpoints.
	ren := newRelVersionInput(0, "A2", "links", "X", "v1")
	ren.Op = store.VersionOpRename
	ren.PrevFrom = "A"
	ren.PrevTo = "X"
	require.NoError(t, s.WriteRelationVersion(ctx, ren))

	// History via the new key is one continuous timeline on the surviving
	// lineage: the pre-rename create + the rename row. No orphaned lineage.
	metas, err := s.ListRelationVersions(ctx, "A2", "links", "X")
	require.NoError(t, err)
	require.Len(t, metas, 2, "continuous timeline on the surviving rel_record_id")
	require.Equal(t, store.VersionOpCreate, metas[0].Op)
	require.Equal(t, "A", metas[0].From, "the create version still carries the pre-rename endpoint")
	require.Equal(t, store.VersionOpRename, metas[1].Op)
	require.Equal(t, "A2", metas[1].From)
	require.Equal(t, "A", metas[1].PrevFrom)

	// Prove no FORK directly, not just via Len==2 (which the stitch-walk could
	// satisfy from two lineages): both version rows must sit on the ONE surviving
	// rel_record_id, and that id must be `rid`.
	var distinctLineages int
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT COUNT(DISTINCT rel_record_id) FROM relation_versions WHERE rel_record_id = $1`, rid).Scan(&distinctLineages))
	require.Equal(t, 1, distinctLineages)
	var total, onRid int
	require.NoError(t, pool.QueryRow(ctx, `SELECT COUNT(*) FROM relation_versions`).Scan(&total))
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM relation_versions WHERE rel_record_id = $1`, rid).Scan(&onRid))
	require.Equal(t, total, onRid, "every version row is on the surviving lineage — no forked rel_record_id")
}

// TestRelationRenameDoesNotBumpUpdatedAt pins the RR-N5YK81 fact (now the live
// atomic path): the entity rename's relation re-key does NOT touch
// relations.updated_at. Consequence: the sweep (which uses updated_at) cannot
// back-fill a rename capture that the synchronous hook missed. This is accepted
// as best-effort — the lineage stays continuous on the surviving rel_record_id
// regardless, so a missed rename version loses only the rename MARKER, not
// history continuity. If a future change wants sweep back-fill for renames, bump
// updated_at in the atomic re-key (entity.go RenameEntity) — this test guards
// the current documented behavior.
func TestRelationRenameDoesNotBumpUpdatedAt(t *testing.T) {
	pool := newScopedPool(t)
	s, err := pgstore.New(pool)
	require.NoError(t, err)
	t.Cleanup(func() { _ = s.Close() })
	ctx := context.Background()

	require.NoError(t, s.CreateEntity(ctx, mkEntity("A", "ticket", "")))
	require.NoError(t, s.CreateEntity(ctx, mkEntity("X", "ticket", "")))
	_, err = s.CreateRelation(ctx, "A", "links", "X", &store.RelationData{Content: "v1"})
	require.NoError(t, err)

	var before string
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT updated_at::text FROM relations WHERE from_id='A' AND rel_type='links' AND to_id='X'`).Scan(&before))

	_, err = s.RenameEntity(ctx, "A", "A2")
	require.NoError(t, err)

	var after string
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT updated_at::text FROM relations WHERE from_id='A2' AND rel_type='links' AND to_id='X'`).Scan(&after))

	require.Equal(t, before, after,
		"atomic rename re-key does not bump relations.updated_at (documented best-effort; sweep can't back-fill a missed rename)")
}

// TestSweepCapturesSettledRelations drives the sweep against a settled relation
// and asserts it snapshots it once (as create), dedups a no-op re-run, and skips
// a relation that hasn't settled.
func TestSweepCapturesSettledRelations(t *testing.T) {
	pool := newScopedPool(t)
	s, err := pgstore.New(pool)
	require.NoError(t, err)
	t.Cleanup(func() { _ = s.Close() })
	ctx := context.Background()

	_, err = s.CreateRelation(ctx, "SET-A", "blocks", "SET-B",
		&store.RelationData{Content: "settled"})
	require.NoError(t, err)
	_, err = s.CreateRelation(ctx, "FRESH-A", "blocks", "FRESH-B",
		&store.RelationData{Content: "fresh"})
	require.NoError(t, err)
	// Backdate the settled relation so the idle filter admits it.
	_, err = pool.Exec(ctx,
		`UPDATE relations SET updated_at = now() - interval '1 hour' WHERE from_id = 'SET-A'`)
	require.NoError(t, err)

	s.StartVersionSweep(stubProvider{hash: "schema-rel", json: []byte(`{"relations":{}}`)},
		pgstore.SweepConfig{Interval: 50 * time.Millisecond, Idle: time.Minute, MaxStaleness: time.Hour, Batch: 100})

	require.Eventually(t, func() bool {
		metas, e := s.ListRelationVersions(ctx, "SET-A", "blocks", "SET-B")
		return e == nil && len(metas) == 1 && metas[0].Op == store.VersionOpCreate
	}, 3*time.Second, 25*time.Millisecond, "sweep should capture the settled relation once as create")

	fresh, err := s.ListRelationVersions(ctx, "FRESH-A", "blocks", "FRESH-B")
	require.NoError(t, err)
	require.Empty(t, fresh, "fresh relation should not be versioned yet")

	// Dedup: unchanged content produces no further versions across ticks.
	time.Sleep(200 * time.Millisecond)
	metas, err := s.ListRelationVersions(ctx, "SET-A", "blocks", "SET-B")
	require.NoError(t, err)
	require.Len(t, metas, 1, "unchanged relation content must not duplicate versions")
}
