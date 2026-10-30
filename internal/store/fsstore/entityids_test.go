package fsstore_test

import (
	"context"
	"sort"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/storage"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/store/fsstore"
)

// TestEntityIDs_MatchesAllFacesScan pins that EntityIDs lists exactly the
// ids a full all-faces scan finds, each family once, both on the store that
// wrote the entities and on one that rebuilt its index from the files.
// entitymanager mints ids from this list, so a missing id would let it hand
// out one that is already taken.
func TestEntityIDs_MatchesAllFacesScan(t *testing.T) {
	memfs := storage.NewMemFS()
	s, err := fsstore.New(newConfig(memfs))
	require.NoError(t, err)
	ctx := context.Background()

	for _, e := range []*entity.Entity{
		{ID: "PAGE-1", Type: "page"},
		{ID: "PAGE-1", Type: "page", Face: "draft"},
		{ID: "PAGE-2", Type: "page", Face: "draft"},
		{ID: "SPEC-1", Type: "page"},
	} {
		require.NoError(t, s.CreateEntity(ctx, e))
	}

	scanned := func(st store.Store) []string {
		seen := map[string]bool{}
		var ids []string
		for e, err := range st.ListEntities(ctx, store.EntityQuery{Faces: store.AllFaces()}) {
			require.NoError(t, err)
			if !seen[e.ID] {
				seen[e.ID] = true
				ids = append(ids, e.ID)
			}
		}
		sort.Strings(ids)
		return ids
	}
	listed := func(st *fsstore.FSStore) []string {
		ids, listErr := st.EntityIDs(ctx)
		require.NoError(t, listErr)
		sort.Strings(ids)
		return ids
	}

	want := []string{"PAGE-1", "PAGE-2", "SPEC-1"}
	require.Equal(t, want, scanned(s))
	require.Equal(t, want, listed(s))

	reopened, err := fsstore.New(newConfig(memfs))
	require.NoError(t, err)
	require.Equal(t, want, listed(reopened))

	_, err = s.DeleteFamily(ctx, "SPEC-1", false)
	require.NoError(t, err)
	require.Equal(t, []string{"PAGE-1", "PAGE-2"}, listed(s))
}
