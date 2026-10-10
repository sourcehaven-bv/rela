package storetest

import (
	"fmt"
	"math"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// sweepBacklogRows is the backlog size [RunSweepBacklogTests] uses. It must
// exceed the batch of the Capabilities.SweepNow driver, which the tests check.
const sweepBacklogRows = 250

// RunSweepBacklogTests pins that the sweep captures every change when more rows
// are settled than one tick processes (BUG-1DWMYO).
//
// The sweep dedups in Go, after its candidate query. A row the query selects
// and the dedup skips is selected again on every tick, so once a batch of them
// exists the rows behind them are never reached: not slowly, never. The cases
// below are the ways such rows arose: settled rows that were already captured,
// numbers whose stored form differs from their hashed form, and rows saved
// unchanged after a force-live purge.
//
// sweepNow must run one tick that treats every row as settled, with a batch
// smaller than sweepBacklogRows.
func RunSweepBacklogTests(t *testing.T, f Factory, sweepNow func(t *testing.T, s store.Store)) {
	id := func(i int) string { return fmt.Sprintf("FEAT-%03d", i) }
	last := sweepBacklogRows - 1
	// newEntity carries a uint64 above 2^53. The canonical hash folds it to the
	// float64 a JSON round trip reads back, so its stored text and the hashed
	// value differ; a candidate query comparing stored columns rather than
	// hashes would keep every seeded row dirty.
	newEntity := func(id, title string) *entity.Entity {
		e := entity.New(id, "feature")
		e.SetString("title", title)
		e.Properties["big"] = uint64(math.MaxUint64)
		return e
	}
	relKey := func(i int) entity.RelationKey {
		return entity.RelationKey{From: id(0), Type: "depends-on", To: id(i)}
	}
	newRelation := func(content string) *store.RelationData {
		return &store.RelationData{
			Content:    content,
			Properties: map[string]any{"big": uint64(math.MaxUint64)},
		}
	}
	// drain runs enough ticks to capture the backlog if every tick makes
	// progress, plus a spare, so the assertions are about draining rather than
	// an exact tick count.
	drain := func(t *testing.T, s store.Store) {
		t.Helper()
		for range 4 {
			sweepNow(t, s)
		}
	}
	// seed creates the backlog: sweepBacklogRows entities, and a relation from
	// the first to each of the others.
	seed := func(t *testing.T, s store.Store) {
		t.Helper()
		for i := range sweepBacklogRows {
			require.NoError(t, s.CreateEntity(ctx(), newEntity(id(i), "v1")))
		}
		for i := 1; i < sweepBacklogRows; i++ {
			_, err := s.CreateRelation(ctx(), relKey(i), newRelation("v1"))
			require.NoError(t, err)
		}
	}
	entityVersions := func(t *testing.T, v store.VersionService, i int) []store.VersionMeta {
		t.Helper()
		metas, err := v.ListVersions(ctx(), entity.Ref{ID: id(i)})
		require.NoError(t, err)
		return metas
	}
	relationVersions := func(t *testing.T, v store.VersionService, i int) int {
		t.Helper()
		metas, err := v.ListRelationVersions(ctx(), store.RelationHistoryQuery{Key: relKey(i)})
		require.NoError(t, err)
		return len(metas)
	}
	uncaptured := func(t *testing.T, v store.VersionService) []string {
		t.Helper()
		var missing []string
		for i := range sweepBacklogRows {
			if len(entityVersions(t, v, i)) == 0 {
				missing = append(missing, id(i))
			}
			if i > 0 && relationVersions(t, v, i) == 0 {
				missing = append(missing, relKey(i).String())
			}
		}
		return missing
	}

	t.Run("NeverCapturedBacklogDrains", func(t *testing.T) {
		s := f(t)
		v := versionsOf(t, s)
		seed(t, s)

		sweepNow(t, s)
		require.NotEmpty(t, uncaptured(t, v),
			"one tick captured the whole backlog: the SweepNow batch must be below sweepBacklogRows")

		drain(t, s)
		missing := uncaptured(t, v)
		require.Emptyf(t, missing, "%d rows never got a version: captured rows fill every batch",
			len(missing))
	})

	// Each edit lands on the last row, which was written and captured last, so a
	// batch of older, unchanged rows sorts ahead of it by any timestamp. One case
	// per hashed column, so a candidate query that ignores one of them fails.
	edits := []struct {
		name     string
		edit     func(t *testing.T, s store.Store)
		relation bool
	}{
		{name: "EntityProperties", edit: func(t *testing.T, s store.Store) {
			require.NoError(t, s.UpdateEntity(ctx(), newEntity(id(last), "v2")))
		}},
		{name: "EntityContent", edit: func(t *testing.T, s store.Store) {
			e := newEntity(id(last), "v1")
			e.Content = "a body"
			require.NoError(t, s.UpdateEntity(ctx(), e))
		}},
		{name: "EntityType", edit: func(t *testing.T, s store.Store) {
			e := newEntity(id(last), "v1")
			e.Type = "requirement"
			require.NoError(t, s.UpdateEntity(ctx(), e))
		}},
		{name: "RelationContent", relation: true, edit: func(t *testing.T, s store.Store) {
			_, err := s.UpdateRelation(ctx(), relKey(last), *newRelation("v2"))
			require.NoError(t, err)
		}},
		{name: "RelationProperties", relation: true, edit: func(t *testing.T, s store.Store) {
			data := newRelation("v1")
			data.Properties["note"] = "added"
			_, err := s.UpdateRelation(ctx(), relKey(last), *data)
			require.NoError(t, err)
		}},
	}
	for _, tc := range edits {
		t.Run("EditBehindAFullBatch/"+tc.name, func(t *testing.T) {
			s := f(t)
			v := versionsOf(t, s)
			seed(t, s)
			drain(t, s)

			tc.edit(t, s)
			sweepNow(t, s)

			if tc.relation {
				require.Equal(t, 2, relationVersions(t, v, last), "relation edit was not captured")
			} else {
				require.Len(t, entityVersions(t, v, last), 2, "entity edit was not captured")
			}
		})
	}

	// A store-level rename re-keys every relation from the renamed entity
	// without recording a rename version; that is the synchronous hook missing.
	// The sweep must still record the new endpoints, for more relations than
	// one batch holds.
	t.Run("RenameWithoutRenameVersion", func(t *testing.T) {
		s := f(t)
		v := versionsOf(t, s)
		seed(t, s)
		drain(t, s)

		const renamed = "FEAT-RENAMED"
		_, err := s.RenameFamily(ctx(), id(0), renamed)
		require.NoError(t, err)
		drain(t, s)

		var stale []string
		for i := 1; i < sweepBacklogRows; i++ {
			key := entity.RelationKey{From: renamed, Type: "depends-on", To: id(i)}
			metas, err := v.ListRelationVersions(ctx(), store.RelationHistoryQuery{Key: key})
			require.NoError(t, err)
			if len(metas) != 2 {
				stale = append(stale, key.String())
			}
		}
		require.Emptyf(t, stale, "%d relations kept their pre-rename endpoints in history", len(stale))
	})

	// The tombstone must carry the hash of the live row as stored. A wrong
	// hash (GitHub #1807: the purge read the row's columns in the wrong
	// order) makes the next tick capture the erased content again.
	for _, face := range []entity.Face{"", "draft"} {
		t.Run("ForceLivePurgeIsNotRecaptured/face="+string(face), func(t *testing.T) {
			s := f(t)
			v := versionsOf(t, s)
			e := newEntity(id(0), "secret")
			e.Face = face
			require.NoError(t, s.CreateEntity(ctx(), e))
			drain(t, s)
			res, err := v.PurgeVersions(ctx(), store.VersionPurgeRequest{
				Ref: e.Ref(), Selector: store.PurgeSelector{All: true}, ForceLive: true,
			})
			require.NoError(t, err)
			require.True(t, res.TombstoneWritten)
			drain(t, s)

			metas, err := v.ListVersions(ctx(), e.Ref())
			require.NoError(t, err)
			require.Len(t, metas, 1, "the sweep captured the purged content again")
			require.Equal(t, store.VersionOpPurge, metas[0].Op)
		})
	}

	t.Run("ForceLivePurgeIsNotRecaptured/relation", func(t *testing.T) {
		s := f(t)
		v := versionsOf(t, s)
		for i := range 2 {
			require.NoError(t, s.CreateEntity(ctx(), newEntity(id(i), "v1")))
		}
		_, err := s.CreateRelation(ctx(), relKey(1), newRelation("secret"))
		require.NoError(t, err)
		drain(t, s)
		res, err := v.PurgeRelationVersions(ctx(), store.RelationVersionPurgeRequest{
			Key: relKey(1), Selector: store.PurgeSelector{All: true}, ForceLive: true,
		})
		require.NoError(t, err)
		require.True(t, res.TombstoneWritten)
		drain(t, s)

		metas, err := v.ListRelationVersions(ctx(), store.RelationHistoryQuery{Key: relKey(1)})
		require.NoError(t, err)
		require.Len(t, metas, 1, "the sweep captured the purged relation content again")
		require.Equal(t, store.VersionOpPurge, metas[0].Op)
	})

	// A force-live purge leaves a tombstone carrying the live row's hash. Saving
	// the rows unchanged afterwards must neither re-capture the purged content
	// nor leave the rows selected on every tick.
	t.Run("UnchangedSaveAfterForceLivePurge", func(t *testing.T) {
		s := f(t)
		v := versionsOf(t, s)
		seed(t, s)
		drain(t, s)
		for i := range sweepBacklogRows {
			res, err := v.PurgeVersions(ctx(), store.VersionPurgeRequest{
				Ref: entity.Ref{ID: id(i)}, Selector: store.PurgeSelector{All: true}, ForceLive: true,
			})
			require.NoError(t, err)
			require.True(t, res.TombstoneWritten)
			require.NoError(t, s.UpdateEntity(ctx(), newEntity(id(i), "v1")))
		}
		require.NoError(t, s.UpdateEntity(ctx(), newEntity(id(last), "v2")))
		drain(t, s)

		for i := range last {
			metas := entityVersions(t, v, i)
			require.Len(t, metas, 1, "%s: the purged content was captured again", id(i))
			require.Equal(t, store.VersionOpPurge, metas[0].Op)
		}
		metas := entityVersions(t, v, last)
		require.Len(t, metas, 2, "edit behind a batch of purged rows was not captured")
		require.Equal(t, store.VersionOpUpdate, metas[1].Op)
	})
}
