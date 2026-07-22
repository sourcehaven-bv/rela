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

func (p stubProvider) Projection() (hash string, projectionJSON []byte) { return p.hash, p.json }

func newVersionInput(id, content string, props map[string]any) store.VersionInput {
	return store.VersionInput{
		EntityID:      id,
		Type:          "ticket",
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

	in := newVersionInput("TKT-1", "first", map[string]any{"title": "one"})
	in.Op = store.VersionOpCreate
	require.NoError(t, s.VersionStore().WriteVersion(ctx, in))

	in2 := newVersionInput("TKT-1", "second", map[string]any{"title": "two"})
	in2.Op = store.VersionOpUpdate
	require.NoError(t, s.VersionStore().WriteVersion(ctx, in2))

	metas, err := s.VersionStore().ListVersions(ctx, "TKT-1")
	require.NoError(t, err)
	require.Len(t, metas, 2)
	require.Equal(t, 1, metas[0].Version)
	require.Equal(t, store.VersionOpCreate, metas[0].Op)
	require.Equal(t, 2, metas[1].Version)
	require.Equal(t, store.VersionOpUpdate, metas[1].Op)
	require.Equal(t, "alice", metas[1].PrincipalUser)

	// GetVersion returns the full snapshot for a 1-based ordinal.
	snap, err := s.VersionStore().GetVersion(ctx, "TKT-1", 1)
	require.NoError(t, err)
	require.Equal(t, "first", snap.Content)
	require.Equal(t, "one", snap.Properties["title"])
	require.NotEmpty(t, snap.Projection)

	// Out-of-range ordinals are ErrNotFound.
	_, err = s.VersionStore().GetVersion(ctx, "TKT-1", 99)
	require.ErrorIs(t, err, store.ErrNotFound)
	_, err = s.VersionStore().GetVersion(ctx, "TKT-1", 0)
	require.ErrorIs(t, err, store.ErrNotFound)
}

// TestListVersionsUnknownEntity returns an empty timeline (not an error).
func TestListVersionsUnknownEntity(t *testing.T) {
	s, err := pgstore.New(newScopedPool(t))
	require.NoError(t, err)
	t.Cleanup(func() { _ = s.Close() })
	metas, err := s.VersionStore().ListVersions(context.Background(), "NOPE-1")
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
	old := newVersionInput("OLD-1", "content-old", nil)
	old.Op = store.VersionOpCreate
	require.NoError(t, s.VersionStore().WriteVersion(ctx, old))

	// rename OLD-1 -> NEW-1: a version row under NEW-1 carrying prev_id=OLD-1.
	ren := newVersionInput("NEW-1", "content-old", nil)
	ren.Op = store.VersionOpRename
	ren.PrevID = "OLD-1"
	require.NoError(t, s.VersionStore().WriteVersion(ctx, ren))

	// v under NEW id.
	upd := newVersionInput("NEW-1", "content-new", nil)
	upd.Op = store.VersionOpUpdate
	require.NoError(t, s.VersionStore().WriteVersion(ctx, upd))

	// Listing by the NEW id returns the FULL lineage, oldest first.
	metas, err := s.VersionStore().ListVersions(ctx, "NEW-1")
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
		metas, e := s.VersionStore().ListVersions(ctx, "SET-1")
		return e == nil && len(metas) == 1
	}, 3*time.Second, 25*time.Millisecond, "sweep should capture the settled entity exactly once")

	// The fresh entity must NOT be versioned (hasn't settled).
	fresh, err := s.VersionStore().ListVersions(ctx, "FRESH-1")
	require.NoError(t, err)
	require.Empty(t, fresh, "fresh entity should not be versioned yet")

	// Dedup: after the first capture, further ticks add no new version (content
	// unchanged). Give it a few ticks and re-check the count is still 1.
	time.Sleep(200 * time.Millisecond)
	metas, err := s.VersionStore().ListVersions(ctx, "SET-1")
	require.NoError(t, err)
	require.Len(t, metas, 1, "unchanged content must not produce duplicate versions")
}

// TestReusedIDDoesNotMergeHistories is the regression for the id-reuse hazard:
// after rename A->B, a brand-new entity reclaiming id A must NOT have its
// versions bleed into A's old timeline OR into B's lineage.
func TestReusedIDDoesNotMergeHistories(t *testing.T) {
	s, err := pgstore.New(newScopedPool(t))
	require.NoError(t, err)
	t.Cleanup(func() { _ = s.Close() })
	ctx := context.Background()

	// Old A: create + update, then rename A -> B.
	a1 := newVersionInput("A", "a-content-1", nil)
	a1.Op = store.VersionOpCreate
	require.NoError(t, s.VersionStore().WriteVersion(ctx, a1))
	a2 := newVersionInput("A", "a-content-2", nil)
	a2.Op = store.VersionOpUpdate
	require.NoError(t, s.VersionStore().WriteVersion(ctx, a2))
	ren := newVersionInput("B", "a-content-2", nil)
	ren.Op = store.VersionOpRename
	ren.PrevID = "A"
	require.NoError(t, s.VersionStore().WriteVersion(ctx, ren))

	// A brand-new, unrelated entity reclaims id A.
	newA := newVersionInput("A", "UNRELATED-new-A", nil)
	newA.Op = store.VersionOpCreate
	require.NoError(t, s.VersionStore().WriteVersion(ctx, newA))

	// B's history is A(create) -> A(update) -> B(rename): the pre-rename life,
	// NOT the unrelated new A.
	bHist, err := s.VersionStore().ListVersions(ctx, "B")
	require.NoError(t, err)
	require.Len(t, bHist, 3, "B lineage must be its pre-rename life only")
	require.Equal(t, store.VersionOpRename, bHist[2].Op)
	for _, m := range bHist {
		snap, gErr := s.VersionStore().GetVersion(ctx, "B", m.Version)
		require.NoError(t, gErr)
		require.NotEqual(t, "UNRELATED-new-A", snap.Content,
			"the unrelated new-A content must not appear in B's history")
	}

	// The new A's history is ONLY its own create, not the old A's create+update.
	aHist, err := s.VersionStore().ListVersions(ctx, "A")
	require.NoError(t, err)
	require.Len(t, aHist, 1, "reclaimed id A must see only its own lifecycle")
	snap, err := s.VersionStore().GetVersion(ctx, "A", 1)
	require.NoError(t, err)
	require.Equal(t, "UNRELATED-new-A", snap.Content)
}

// TestDeleteThenRecreateIdenticalContent is the regression for the content-only
// dedup hazard: after a delete captured with content H, re-creating the entity
// with the same content H must still record a create — the timeline must not end
// in `delete` for a live entity.
func TestDeleteThenRecreateIdenticalContent(t *testing.T) {
	pool := newScopedPool(t)
	s, err := pgstore.New(pool)
	require.NoError(t, err)
	t.Cleanup(func() { _ = s.Close() })
	ctx := context.Background()

	// A prior lifecycle: create (via sweep-style write) then a delete capturing
	// the same content.
	c1 := newVersionInput("X", "same-content", nil)
	c1.Op = store.VersionOpCreate
	require.NoError(t, s.VersionStore().WriteVersion(ctx, c1))
	del := newVersionInput("X", "same-content", nil)
	del.Op = store.VersionOpDelete
	require.NoError(t, s.VersionStore().WriteVersion(ctx, del))

	// Re-create the live entity with IDENTICAL content, then sweep it.
	require.NoError(t, s.CreateEntity(ctx, mkEntity("X", "ticket", "same-content")))
	_, err = pool.Exec(ctx, `UPDATE entities SET updated_at = now() - interval '1 hour' WHERE id = 'X'`)
	require.NoError(t, err)
	s.StartVersionSweep(stubProvider{hash: "schema-abc", json: []byte(`{"entities":{},"types":{}}`)},
		pgstore.SweepConfig{Interval: 50 * time.Millisecond, Idle: time.Minute, MaxStaleness: time.Hour, Batch: 100})

	require.Eventually(t, func() bool {
		metas, e := s.VersionStore().ListVersions(ctx, "X")
		if e != nil || len(metas) == 0 {
			return false
		}
		// The newest version must be a create (the re-creation), NOT the delete.
		return metas[len(metas)-1].Op == store.VersionOpCreate
	}, 3*time.Second, 25*time.Millisecond,
		"re-creating with identical content must record a create, not dedup away leaving the timeline at delete")
}

//nolint:unparam // typ is "ticket" in every current caller but is a real knob for future entity-type tests.
func mkEntity(id, typ, content string) *entity.Entity {
	e := entity.New(id, typ)
	e.Content = content
	return e
}
