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

// stubProvider is a fixed render-schema projection for the sweep under test.
type stubProvider struct {
	hash string
	json []byte
}

func (p stubProvider) Projection() (string, []byte) { return p.hash, p.json }

func newVersionInput(id, typ, content string, props map[string]interface{}) store.VersionInput {
	return store.VersionInput{
		EntityID:      id,
		Type:          typ,
		Content:       content,
		Properties:    props,
		SchemaHash:    "schema-abc",
		Projection:    []byte(`{"entities":{},"types":{}}`),
		PrincipalUser: "alice",
		PrincipalTool: "cli",
	}
}

// TestWriteVersionAndList captures a couple of versions synchronously and reads
// them back, asserting attribution, op, and 1-based ordinals.
func TestWriteVersionAndList(t *testing.T) {
	s, err := pgstore.New(newScopedPool(t))
	require.NoError(t, err)
	t.Cleanup(func() { _ = s.Close() })
	ctx := context.Background()

	in := newVersionInput("TKT-1", "ticket", "first", map[string]interface{}{"title": "one"})
	in.Op = store.VersionOpCreate
	require.NoError(t, s.WriteVersion(ctx, in))

	in2 := newVersionInput("TKT-1", "ticket", "second", map[string]interface{}{"title": "two"})
	in2.Op = store.VersionOpUpdate
	require.NoError(t, s.WriteVersion(ctx, in2))

	metas, err := s.ListVersions(ctx, "TKT-1")
	require.NoError(t, err)
	require.Len(t, metas, 2)
	require.Equal(t, 1, metas[0].Version)
	require.Equal(t, store.VersionOpCreate, metas[0].Op)
	require.Equal(t, 2, metas[1].Version)
	require.Equal(t, store.VersionOpUpdate, metas[1].Op)
	require.Equal(t, "alice", metas[1].PrincipalUser)

	// GetVersion returns the full snapshot for a 1-based ordinal.
	snap, err := s.GetVersion(ctx, "TKT-1", 1)
	require.NoError(t, err)
	require.Equal(t, "first", snap.Content)
	require.Equal(t, "one", snap.Properties["title"])
	require.NotEmpty(t, snap.Projection)

	// Out-of-range ordinals are ErrNotFound.
	_, err = s.GetVersion(ctx, "TKT-1", 99)
	require.ErrorIs(t, err, store.ErrNotFound)
	_, err = s.GetVersion(ctx, "TKT-1", 0)
	require.ErrorIs(t, err, store.ErrNotFound)
}

// TestListVersionsUnknownEntity returns an empty timeline (not an error).
func TestListVersionsUnknownEntity(t *testing.T) {
	s, err := pgstore.New(newScopedPool(t))
	require.NoError(t, err)
	t.Cleanup(func() { _ = s.Close() })
	metas, err := s.ListVersions(context.Background(), "NOPE-1")
	require.NoError(t, err)
	require.Empty(t, metas)
}

// TestRenameLineage stitches an entity's history across a rename: a version
// under the OLD id, a rename event carrying prev_id, and a version under the NEW
// id all appear in one lineage keyed by the new id.
func TestRenameLineage(t *testing.T) {
	s, err := pgstore.New(newScopedPool(t))
	require.NoError(t, err)
	t.Cleanup(func() { _ = s.Close() })
	ctx := context.Background()

	// v1 under OLD id.
	old := newVersionInput("OLD-1", "ticket", "content-old", nil)
	old.Op = store.VersionOpCreate
	require.NoError(t, s.WriteVersion(ctx, old))

	// rename OLD-1 -> NEW-1: a version row under NEW-1 carrying prev_id=OLD-1.
	ren := newVersionInput("NEW-1", "ticket", "content-old", nil)
	ren.Op = store.VersionOpRename
	ren.PrevID = "OLD-1"
	require.NoError(t, s.WriteVersion(ctx, ren))

	// v under NEW id.
	upd := newVersionInput("NEW-1", "ticket", "content-new", nil)
	upd.Op = store.VersionOpUpdate
	require.NoError(t, s.WriteVersion(ctx, upd))

	// Listing by the NEW id returns the FULL lineage, oldest first.
	metas, err := s.ListVersions(ctx, "NEW-1")
	require.NoError(t, err)
	require.Len(t, metas, 3, "lineage should include the pre-rename version")
	require.Equal(t, store.VersionOpCreate, metas[0].Op)
	require.Equal(t, store.VersionOpRename, metas[1].Op)
	require.Equal(t, "OLD-1", metas[1].PrevID)
	require.Equal(t, store.VersionOpUpdate, metas[2].Op)
}

// TestSweepCapturesSettledEntities drives the sweep against settled entities and
// asserts it snapshots each once, dedups a no-op re-run, and skips entities that
// haven't settled.
func TestSweepCapturesSettledEntities(t *testing.T) {
	pool := newScopedPool(t)
	s, err := pgstore.New(pool)
	require.NoError(t, err)
	t.Cleanup(func() { _ = s.Close() })

	ctx := context.Background()

	// Two entities; backdate one's updated_at so it counts as settled, leave the
	// other fresh so it is excluded by the idle filter.
	require.NoError(t, s.CreateEntity(ctx, mkEntity("SET-1", "ticket", "settled")))
	require.NoError(t, s.CreateEntity(ctx, mkEntity("FRESH-1", "ticket", "fresh")))
	_, err = pool.Exec(ctx, `UPDATE entities SET updated_at = now() - interval '1 hour' WHERE id = 'SET-1'`)
	require.NoError(t, err)

	// Start a fast sweep with a short idle so SET-1 qualifies and FRESH-1 doesn't.
	s.StartVersionSweep(stubProvider{hash: "schema-abc", json: []byte(`{"entities":{},"types":{}}`)},
		pgstore.SweepConfig{Interval: 50 * time.Millisecond, Idle: time.Minute, MaxStaleness: time.Hour, Batch: 100})

	require.Eventually(t, func() bool {
		metas, e := s.ListVersions(ctx, "SET-1")
		return e == nil && len(metas) == 1
	}, 3*time.Second, 25*time.Millisecond, "sweep should capture the settled entity exactly once")

	// The fresh entity must NOT be versioned (hasn't settled).
	fresh, err := s.ListVersions(ctx, "FRESH-1")
	require.NoError(t, err)
	require.Empty(t, fresh, "fresh entity should not be versioned yet")

	// Dedup: after the first capture, further ticks add no new version (content
	// unchanged). Give it a few ticks and re-check the count is still 1.
	time.Sleep(200 * time.Millisecond)
	metas, err := s.ListVersions(ctx, "SET-1")
	require.NoError(t, err)
	require.Len(t, metas, 1, "unchanged content must not produce duplicate versions")
}

func mkEntity(id, typ, content string) *entity.Entity {
	e := entity.New(id, typ)
	e.Content = content
	return e
}
