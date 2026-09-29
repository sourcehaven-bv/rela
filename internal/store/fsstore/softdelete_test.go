package fsstore_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/storage"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/store/fsstore"
)

// seedMarked creates REQ-1 and SOL-1 with an edge between them, then
// soft-deletes REQ-1.
func seedMarked(t *testing.T, fs *storage.MemFS) *fsstore.FSStore {
	t.Helper()
	ctx := context.Background()
	s := openStore(t, fs)
	req := entity.New("REQ-1", "requirement")
	req.Properties["title"] = "Req"
	require.NoError(t, s.CreateEntity(ctx, req))
	sol := entity.New("SOL-1", "solution")
	sol.Properties["title"] = "Sol"
	require.NoError(t, s.CreateEntity(ctx, sol))
	_, err := s.CreateRelation(ctx, "SOL-1", "implements", "REQ-1", nil)
	require.NoError(t, err)
	_, err = s.SoftDelete().MarkDeleted(ctx, "REQ-1", "alice")
	require.NoError(t, err)
	return s
}

func assertStillMarked(t *testing.T, s *fsstore.FSStore) {
	t.Helper()
	ctx := context.Background()
	_, err := s.GetEntity(ctx, "REQ-1")
	require.ErrorIs(t, err, store.ErrNotFound)
	n, err := s.CountRelations(ctx, store.RelationQuery{})
	require.NoError(t, err)
	assert.Zero(t, n, "the hidden edge must stay hidden")
	require.ErrorIs(t, s.CreateEntity(ctx, entity.New("REQ-1", "requirement")), store.ErrConflict)
	marked, err := s.SoftDelete().ListMarked(ctx)
	require.NoError(t, err)
	require.Len(t, marked, 1)
	assert.Equal(t, "REQ-1", marked[0].ID)
	assert.Equal(t, "alice", marked[0].DeletedBy)
}

// A mark must survive a restart, whether the next open restores the cached
// index (clean Close) or rescans the directories (no Close, as after a crash).
func TestSoftDelete_SurvivesReopen(t *testing.T) {
	for _, tc := range []struct {
		name  string
		close bool
	}{
		{name: "cached index", close: true},
		{name: "rescan", close: false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			fs := storage.NewMemFS()
			s1 := seedMarked(t, fs)
			if tc.close {
				require.NoError(t, s1.Close())
			}

			s2 := openStore(t, fs)
			assertStillMarked(t, s2)

			_, err := s2.SoftDelete().Unmark(ctx, "REQ-1")
			require.NoError(t, err)
			require.NoError(t, s2.Close())

			s3 := openStore(t, fs)
			defer s3.Close()
			_, err = s3.GetEntity(ctx, "REQ-1")
			require.NoError(t, err)
			n, err := s3.CountRelations(ctx, store.RelationQuery{})
			require.NoError(t, err)
			assert.Equal(t, 1, n)
			marked, err := s3.SoftDelete().ListMarked(ctx)
			require.NoError(t, err)
			assert.Empty(t, marked)
		})
	}
}

func TestSoftDelete_PurgeAfterReopenRemovesFiles(t *testing.T) {
	ctx := context.Background()
	fs := storage.NewMemFS()
	require.NoError(t, seedMarked(t, fs).Close())

	s2 := openStore(t, fs)
	res, err := s2.SoftDelete().PurgeMarked(ctx, "REQ-1")
	require.NoError(t, err)
	assert.Len(t, res.DeletedEntities, 1)
	assert.Len(t, res.DeletedRelations, 1)
	require.NoError(t, s2.Close())

	s3 := openStore(t, fs)
	defer s3.Close()
	_, err = s3.GetEntity(ctx, "REQ-1")
	require.ErrorIs(t, err, store.ErrNotFound)
	marked, err := s3.SoftDelete().ListMarked(ctx)
	require.NoError(t, err)
	assert.Empty(t, marked)
	require.NoError(t, s3.CreateEntity(ctx, entity.New("REQ-1", "requirement")), "a purged id is free again")
}

// An entry whose files are gone by the next open is dropped rather than
// holding the id forever.
func TestSoftDelete_StaleEntryIsDropped(t *testing.T) {
	ctx := context.Background()
	fs := storage.NewMemFS()
	seedMarked(t, fs) // left open: the next open must not depend on Close
	require.NoError(t, fs.Remove("/entities/requirements/REQ-1.md"))
	require.NoError(t, fs.Remove("/relations/SOL-1--implements--REQ-1.md"))

	s2 := openStore(t, fs)
	defer s2.Close()
	marked, err := s2.SoftDelete().ListMarked(ctx)
	require.NoError(t, err)
	assert.Empty(t, marked)
	require.NoError(t, s2.CreateEntity(ctx, entity.New("REQ-1", "requirement")))
}

// A hidden edge dropped by a hard delete of its other end stays dropped after
// a restart: its file is gone, so neither the cached index nor a rescan can
// bring it back.
func TestSoftDelete_DroppedEdgeStaysDroppedAfterReopen(t *testing.T) {
	for _, tc := range []struct {
		name  string
		close bool
	}{
		{name: "cached index", close: true},
		{name: "rescan", close: false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			fs := storage.NewMemFS()
			s1 := seedMarked(t, fs)
			_, err := s1.DeleteEntity(ctx, "SOL-1", true)
			require.NoError(t, err)
			if tc.close {
				require.NoError(t, s1.Close())
			}

			s2 := openStore(t, fs)
			defer s2.Close()
			sol := entity.New("SOL-1", "solution")
			sol.Properties["title"] = "New sol"
			require.NoError(t, s2.CreateEntity(ctx, sol))
			_, err = s2.SoftDelete().Unmark(ctx, "REQ-1")
			require.NoError(t, err)
			_, err = s2.GetRelation(ctx, "SOL-1", "implements", "REQ-1")
			require.ErrorIs(t, err, store.ErrNotFound)
		})
	}
}
