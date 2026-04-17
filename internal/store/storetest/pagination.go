package storetest

import (
	"fmt"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/store/storeutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// RunPaginationTests runs conformance tests for ListEntitiesPage and
// ListRelationsPage. These verify the cursor contract, limit semantics,
// and the interaction with query filters — properties the dataentry
// handlers (and any other paginating caller) depend on.
func RunPaginationTests(t *testing.T, f Factory) {
	t.Run("EntitiesFullPageWhenLimitZero", func(t *testing.T) {
		s := f(t)
		seedEntities(t, s)

		page, err := s.ListEntitiesPage(ctx(), store.EntityQuery{})
		require.NoError(t, err)
		assert.Len(t, page.Items, 4)
		assert.Empty(t, page.NextCursor, "limit=0 never sets a cursor")
	})

	t.Run("EntitiesWalkWithCursor", func(t *testing.T) {
		s := f(t)
		seedEntities(t, s)

		var ids []string
		cursor := ""
		for pages := 0; pages < 10; pages++ {
			page, err := s.ListEntitiesPage(ctx(), store.EntityQuery{Limit: 2, Cursor: cursor})
			require.NoError(t, err)
			for _, e := range page.Items {
				ids = append(ids, e.ID)
			}
			if page.NextCursor == "" {
				break
			}
			cursor = page.NextCursor
		}
		assert.Equal(t, []string{"FEAT-001", "FEAT-002", "FEAT-013", "REQ-001"}, ids,
			"walking with a cursor should yield every entity exactly once, in stable order")
	})

	t.Run("EntitiesNextCursorEmptyOnLastPage", func(t *testing.T) {
		s := f(t)
		seedEntities(t, s)

		// Page size 4 matches the dataset exactly. After emitting all four
		// items there is nothing more — NextCursor must be empty.
		page, err := s.ListEntitiesPage(ctx(), store.EntityQuery{Limit: 4})
		require.NoError(t, err)
		assert.Len(t, page.Items, 4)
		assert.Empty(t, page.NextCursor,
			"NextCursor must be empty iff no further results exist — "+
				"otherwise callers issue a wasted query that returns no items")
	})

	t.Run("EntitiesCursorPastEndReturnsEmpty", func(t *testing.T) {
		s := f(t)
		seedEntities(t, s)

		page, err := s.ListEntitiesPage(ctx(), store.EntityQuery{Limit: 4})
		require.NoError(t, err)
		require.Empty(t, page.NextCursor)

		// Simulate a stale client that still holds a cursor past the last item.
		// Encode a key lexicographically after every seeded ID.
		page2, err := s.ListEntitiesPage(ctx(), store.EntityQuery{
			Limit:  10,
			Cursor: encodeTestCursor(t, "zzz-sentinel"),
		})
		require.NoError(t, err)
		assert.Empty(t, page2.Items)
		assert.Empty(t, page2.NextCursor)
	})

	t.Run("EntitiesEmptyStore", func(t *testing.T) {
		s := f(t)

		page, err := s.ListEntitiesPage(ctx(), store.EntityQuery{Limit: 10})
		require.NoError(t, err)
		assert.Empty(t, page.Items)
		assert.Empty(t, page.NextCursor)
	})

	t.Run("EntitiesRespectsTypeFilter", func(t *testing.T) {
		s := f(t)
		seedEntities(t, s)

		var ids []string
		cursor := ""
		for pages := 0; pages < 10; pages++ {
			page, err := s.ListEntitiesPage(ctx(), store.EntityQuery{
				Type: "feature", Limit: 2, Cursor: cursor,
			})
			require.NoError(t, err)
			for _, e := range page.Items {
				ids = append(ids, e.ID)
				assert.Equal(t, "feature", e.Type)
			}
			if page.NextCursor == "" {
				break
			}
			cursor = page.NextCursor
		}
		assert.Equal(t, []string{"FEAT-001", "FEAT-002", "FEAT-013"}, ids)
	})

	t.Run("EntitiesRespectsIDFilter", func(t *testing.T) {
		s := f(t)
		seedEntities(t, s)

		page, err := s.ListEntitiesPage(ctx(), store.EntityQuery{
			IDs:   []string{"FEAT-001", "REQ-001"},
			Limit: 10,
		})
		require.NoError(t, err)
		ids := make([]string, len(page.Items))
		for i, e := range page.Items {
			ids[i] = e.ID
		}
		assert.ElementsMatch(t, []string{"FEAT-001", "REQ-001"}, ids)
	})

	t.Run("EntitiesInvalidCursorReturnsError", func(t *testing.T) {
		s := f(t)
		seedEntities(t, s)

		_, err := s.ListEntitiesPage(ctx(), store.EntityQuery{Cursor: "not-base64!!!", Limit: 2})
		assert.Error(t, err, "malformed cursors must not be silently treated as start-of-stream")
	})

	t.Run("EntitiesPageSizeLargerThanDataset", func(t *testing.T) {
		s := f(t)
		seedEntities(t, s)

		page, err := s.ListEntitiesPage(ctx(), store.EntityQuery{Limit: 100})
		require.NoError(t, err)
		assert.Len(t, page.Items, 4)
		assert.Empty(t, page.NextCursor)
	})

	t.Run("EntitiesStableOrderAcrossCalls", func(t *testing.T) {
		// Seed many entities so non-deterministic ordering (e.g. map iteration)
		// has a chance to show up. Insert in reverse order to catch backends
		// that would otherwise happen to match insertion order.
		s := f(t)
		const N = 50
		for i := N; i >= 1; i-- {
			e := entity.New(fmt.Sprintf("T-%03d", i), "t")
			require.NoError(t, s.CreateEntity(ctx(), e))
		}

		collect := func() []string {
			var ids []string
			cursor := ""
			for {
				page, err := s.ListEntitiesPage(ctx(), store.EntityQuery{Limit: 7, Cursor: cursor})
				require.NoError(t, err)
				for _, e := range page.Items {
					ids = append(ids, e.ID)
				}
				if page.NextCursor == "" {
					break
				}
				cursor = page.NextCursor
			}
			return ids
		}
		first := collect()
		second := collect()
		assert.Equal(t, first, second, "paged reads must be stable across calls")
		assert.Len(t, first, N)
	})

	t.Run("RelationsWalkWithCursor", func(t *testing.T) {
		s := f(t)
		seedEntities(t, s)
		// Create a few relations to walk.
		for _, rel := range []struct{ from, typ, to string }{
			{"FEAT-001", "relates-to", "REQ-001"},
			{"FEAT-002", "relates-to", "REQ-001"},
			{"FEAT-013", "relates-to", "REQ-001"},
		} {
			_, err := s.CreateRelation(ctx(), rel.from, rel.typ, rel.to, nil)
			require.NoError(t, err)
		}

		var keys []string
		cursor := ""
		for pages := 0; pages < 10; pages++ {
			page, err := s.ListRelationsPage(ctx(), store.RelationQuery{Limit: 2, Cursor: cursor})
			require.NoError(t, err)
			for _, r := range page.Items {
				keys = append(keys, r.From+"--"+r.Type+"--"+r.To)
			}
			if page.NextCursor == "" {
				break
			}
			cursor = page.NextCursor
		}
		assert.Len(t, keys, 3)
	})

	t.Run("RelationsLastPageHasEmptyCursor", func(t *testing.T) {
		s := f(t)
		seedEntities(t, s)
		_, err := s.CreateRelation(ctx(), "FEAT-001", "relates-to", "REQ-001", nil)
		require.NoError(t, err)
		_, err = s.CreateRelation(ctx(), "FEAT-002", "relates-to", "REQ-001", nil)
		require.NoError(t, err)

		page, err := s.ListRelationsPage(ctx(), store.RelationQuery{Limit: 2})
		require.NoError(t, err)
		assert.Len(t, page.Items, 2)
		assert.Empty(t, page.NextCursor)
	})

	t.Run("RelationsRespectsDirectionFilter", func(t *testing.T) {
		s := f(t)
		seedEntities(t, s)
		_, err := s.CreateRelation(ctx(), "FEAT-001", "relates-to", "REQ-001", nil)
		require.NoError(t, err)
		_, err = s.CreateRelation(ctx(), "REQ-001", "requires", "FEAT-002", nil)
		require.NoError(t, err)

		page, err := s.ListRelationsPage(ctx(), store.RelationQuery{
			EntityID:  "REQ-001",
			Direction: store.DirectionIncoming,
			Limit:     10,
		})
		require.NoError(t, err)
		require.Len(t, page.Items, 1)
		assert.Equal(t, "FEAT-001", page.Items[0].From)
	})
}

// encodeTestCursor produces a cursor for an arbitrary sort key so tests
// can simulate clients holding stale cursors. Production callers get
// cursors from NextCursor and never construct them.
func encodeTestCursor(t *testing.T, key string) string {
	t.Helper()
	return storeutil.EncodeCursor(key)
}
