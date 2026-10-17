package fsstore_test

import (
	"context"
	"errors"
	"sort"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/storage"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/store/fsstore"
)

// fsstore's Tx cannot roll back, so a conversion that fails halfway must undo
// its own file writes: disk may never hold the old-type and the new-type file
// for one id, and a reopened store must see exactly the original family.
func TestReplaceEntityFamily_FailedRemoveRestoresFamily(t *testing.T) {
	mem := storage.NewMemFS()
	ctx := context.Background()
	s1 := openStore(t, mem)
	for _, face := range []entity.Face{"draft", "approved"} {
		e := entity.New("FEAT-050", "feature")
		e.Face = face
		e.SetString("title", "at "+face.String())
		require.NoError(t, s1.CreateEntity(ctx, e))
	}
	require.NoError(t, s1.Close())

	// Fail the second source-file removal, so one source file is already
	// gone and every target file is written when the error fires.
	removals := 0
	errFS := storage.NewErrorFS(mem)
	errFS.RemoveError = errors.New("simulated I/O failure")
	errFS.RemoveErrorOn = func(path string) bool {
		if !strings.Contains(path, "FEAT-050") || strings.Contains(path, "requirement") {
			return false
		}
		removals++
		return removals == 2
	}
	rooted, err := storage.NewRootedFS(errFS, "/")
	require.NoError(t, err)
	cfg := newConfig(mem)
	cfg.FS = errFS
	cfg.Rooted = rooted
	s2, err := fsstore.New(cfg)
	require.NoError(t, err)

	var plan []store.FamilyReplacement
	for _, face := range []entity.Face{"draft", "approved"} {
		e := entity.New("FEAT-050", "requirement")
		e.Face = face
		plan = append(plan, store.FamilyReplacement{SourceFace: face, Entity: e})
	}
	require.Error(t, s2.ReplaceEntityFamily(ctx, "FEAT-050", plan))
	assert.Equal(t, []string{"feature|approved", "feature|draft"}, familyRows(t, s2), "index after failure")
	require.NoError(t, s2.Close())

	s3 := openStore(t, mem)
	defer s3.Close()
	assert.Equal(t, []string{"feature|approved", "feature|draft"}, familyRows(t, s3), "disk after failure")
	got, err := s3.GetEntity(ctx, entity.Ref{ID: "FEAT-050", Face: "draft"})
	require.NoError(t, err)
	assert.Equal(t, "at draft", got.GetString("title"))
}

func familyRows(t *testing.T, s store.Store) []string {
	t.Helper()
	var out []string
	for e, err := range s.ListEntities(context.Background(),
		store.EntityQuery{IDs: []string{"FEAT-050"}, Faces: store.AllFaces()}) {
		require.NoError(t, err)
		out = append(out, e.Type+"|"+e.Face.String())
	}
	sort.Strings(out)
	return out
}
