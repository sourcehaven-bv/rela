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

// TestReopenPreservesStateKeyOrdering pins the index CONSTRUCTION paths
// to the same comparator the mutation paths use (TKT-WAV8XP PR-B).
//
// entityOrder is maintained by storeutil.SortedInsertFunc /
// SortedRemoveFunc under storeutil.CompareStateKeys — the (bare id,
// pointer) tuple. If a construction path sorts by plain string order
// instead, the slice is sorted one way and binary-searched another. The
// symptom is NOT subtle and NOT world-specific: SortedRemoveFunc misses
// a key that is present and PANICS on the ordinary default-world delete
// path, on every process restart.
//
// The ids are the Ruling-1 hazard set: '@' is 0x40 and the digits are
// 0x30-0x39, so '@' sorts after any digit and PAGE-10's family lands
// inside PAGE-1's under plain string order.
//
// The shared conformance suite cannot catch this — its factory only ever
// builds a fresh store and never reopens one, so it never exercises
// index reconstruction at all.
func TestReopenPreservesStateKeyOrdering(t *testing.T) {
	hazardIDs := []string{"PAGE-1", "PAGE-10", "PAGE-2"}

	seed := func(t *testing.T, fs *storage.MemFS) {
		t.Helper()
		s := openStore(t, fs)
		ctx := context.Background()
		for _, id := range hazardIDs {
			require.NoError(t, s.CreateEntity(ctx, &entity.Entity{
				ID: id, Type: "thing", Properties: map[string]any{"title": id + " default"},
			}))
			require.NoError(t, s.CreateEntity(ctx, &entity.Entity{
				ID: id, Type: "thing", Pointer: entity.Pointer("published"),
				Properties: map[string]any{"title": id + " published"},
			}))
		}
		require.NoError(t, s.Close())
	}

	// Both construction paths must be covered: syncEntities restores from
	// the cached index when the entities-dir mtime is unchanged, and cold-
	// scans the directory tree otherwise. They sort entityOrder separately.
	for _, tc := range []struct {
		name      string
		dropCache bool
	}{
		{name: "CachedIndexRestore", dropCache: false},
		{name: "ColdDirectoryScan", dropCache: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			fs := storage.NewMemFS()
			seed(t, fs)
			if tc.dropCache {
				// Force the cold scan by removing the persisted index.
				_ = fs.Remove("/.rela/fsstore-index.json")
			}

			s := openStore(t, fs)
			defer func() { _ = s.Close() }()
			ctx := context.Background()

			// A world-scoped page must resolve each family to its
			// published face, one prime per entity, across every page
			// boundary. Under a mis-sorted index PAGE-1 yields twice.
			world := store.NewWorldScope(map[string]store.TypeResolution{
				"thing": {
					Chain:    []entity.Pointer{entity.Pointer("published")},
					Fallback: store.FallbackDefaultState,
				},
			})
			want := map[string]string{
				"PAGE-1":  "PAGE-1 published",
				"PAGE-10": "PAGE-10 published",
				"PAGE-2":  "PAGE-2 published",
			}
			for _, limit := range []int{1, 2, 3} {
				got := map[string]string{}
				cursor := ""
				for range 10 { // bounded: 3 primes
					page, err := s.ListEntitiesPage(ctx, store.EntityQuery{
						Type: "thing", World: world, Limit: limit, Cursor: cursor,
					})
					require.NoError(t, err)
					for _, e := range page.Items {
						if prev, dup := got[e.ID]; dup {
							t.Fatalf("limit %d: %s yielded twice (%q then %q)",
								limit, e.ID, prev, e.GetString("title"))
						}
						got[e.ID] = e.GetString("title")
					}
					if page.NextCursor == "" {
						break
					}
					cursor = page.NextCursor
				}
				assert.Equal(t, want, got, "paging with limit %d after reopen", limit)
			}

			// The default-world delete path must not panic. This is the
			// symptom that has nothing to do with worlds.
			_, err := s.DeleteEntity(ctx, "PAGE-1", false)
			require.NoError(t, err, "delete after reopen")

			n, err := s.CountEntities(ctx, store.EntityQuery{Type: "thing"})
			require.NoError(t, err)
			assert.Equal(t, 2, n, "default-state count after deleting one family")
		})
	}
}
