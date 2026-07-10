package pgstore_test

import (
	"context"
	"testing"

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
		Properties:    map[string]interface{}{"weight": "high"},
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
		&store.RelationData{Content: "first", Properties: map[string]interface{}{"weight": "high"}})
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
