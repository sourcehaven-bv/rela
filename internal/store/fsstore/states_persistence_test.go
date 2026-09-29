package fsstore_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/storage"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

func mustPtr(t *testing.T, v string) entity.Face {
	t.Helper()
	p, err := entity.ParseFace(v)
	require.NoError(t, err)
	return p
}

// TestStatePersistence_FamilySurvivesReopen pins the state family across
// a store restart (TKT-DOFYR1): the filename serialization round-trips,
// the reopened index keys states correctly, and deleting the family
// after the reopen removes every face.
func TestStatePersistence_FamilySurvivesReopen(t *testing.T) {
	fs := storage.NewMemFS()
	ctx := context.Background()

	s1 := openStore(t, fs)
	def := entity.New("REQ-1", "requirement")
	def.Properties["title"] = "Default face"
	def.Properties["status"] = "open"
	require.NoError(t, s1.CreateEntity(ctx, def))

	draft := entity.New("REQ-1", "requirement")
	draft.Face = mustPtr(t, "draft")
	draft.Properties["title"] = "Draft face"
	draft.Properties["status"] = "open"
	require.NoError(t, s1.CreateEntity(ctx, draft))

	_, err := s1.CreateRelation(ctx, "REQ-1", "refines", "REQ-1", nil)
	require.NoError(t, err)
	require.NoError(t, s1.Close())

	// Reopen: the family and the state-tailed addressing survive.
	s2 := openStore(t, fs)
	defer func() { require.NoError(t, s2.Close()) }()

	got, err := s2.GetEntity(ctx, entity.Ref{ID: "REQ-1", Face: mustPtr(t, "draft")})
	require.NoError(t, err)
	assert.Equal(t, "REQ-1", got.ID)
	assert.Equal(t, "Draft face", got.Properties["title"])

	_, err = s2.DeleteFamily(ctx, "REQ-1", true)
	require.NoError(t, err)
	for _, face := range []entity.Face{"", mustPtr(t, "draft")} {
		_, err = s2.GetEntity(ctx, entity.Ref{ID: "REQ-1", Face: face})
		require.ErrorIs(t, err, store.ErrNotFound, "face %q must be gone", face)
	}
}

// TestStatePersistence_StateWriteFreshensCache pins that an external
// edit to a STATE file invalidates the persisted index freshness check:
// newestEntityFileMtime must stat the state file, not the default file
// (the review-found staleness).
func TestStatePersistence_StateWriteFreshensCache(t *testing.T) {
	fs := storage.NewMemFS()
	ctx := context.Background()

	s1 := openStore(t, fs)
	def := entity.New("REQ-2", "requirement")
	def.Properties["status"] = "open"
	require.NoError(t, s1.CreateEntity(ctx, def))
	draft := entity.New("REQ-2", "requirement")
	draft.Face = mustPtr(t, "review-2")
	draft.Properties["status"] = "open"
	require.NoError(t, s1.CreateEntity(ctx, draft))

	before, err := s1.LastModified(ctx)
	require.NoError(t, err)

	// A state-only write must move LastModified forward.
	draft.Properties["status"] = "in-review"
	require.NoError(t, s1.UpdateEntity(ctx, draft))
	after, err := s1.LastModified(ctx)
	require.NoError(t, err)
	assert.True(t, after.After(before), "state write must be visible to LastModified (got before=%v after=%v)", before, after)
	require.NoError(t, s1.Close())
}
