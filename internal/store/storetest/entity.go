package storetest

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// RunEntityTests runs entity CRUD conformance tests.
func RunEntityTests(t *testing.T, f Factory) {
	t.Run("CreateAndGet", func(t *testing.T) {
		s := f(t)
		e := entity.New("FEAT-001", "feature")
		e.SetString("title", "Login")

		err := s.CreateEntity(ctx(), e)
		require.NoError(t, err)

		got, err := s.GetEntity(ctx(), "FEAT-001")
		require.NoError(t, err)
		assert.Equal(t, e.ID, got.ID)
		assert.Equal(t, e.Type, got.Type)
		assert.Equal(t, "Login", got.GetString("title"))
	})

	t.Run("GetNotFound", func(t *testing.T) {
		s := f(t)
		_, err := s.GetEntity(ctx(), "NOPE")
		assert.ErrorIs(t, err, store.ErrNotFound)
	})

	t.Run("CreateConflict", func(t *testing.T) {
		s := f(t)
		e := entity.New("FEAT-001", "feature")
		require.NoError(t, s.CreateEntity(ctx(), e))

		err := s.CreateEntity(ctx(), entity.New("FEAT-001", "feature"))
		assert.ErrorIs(t, err, store.ErrConflict)
	})

	t.Run("GetReturnsClone", func(t *testing.T) {
		s := f(t)
		e := entity.New("T-1", "ticket")
		e.SetString("title", "Original")
		require.NoError(t, s.CreateEntity(ctx(), e))

		got, _ := s.GetEntity(ctx(), "T-1")
		got.SetString("title", "Mutated")

		got2, _ := s.GetEntity(ctx(), "T-1")
		assert.Equal(t, "Original", got2.GetString("title"))
	})

	t.Run("CreateStoresClone", func(t *testing.T) {
		s := f(t)
		e := entity.New("T-1", "ticket")
		e.SetString("title", "Before")
		require.NoError(t, s.CreateEntity(ctx(), e))

		e.SetString("title", "After")

		got, _ := s.GetEntity(ctx(), "T-1")
		assert.Equal(t, "Before", got.GetString("title"))
	})

	t.Run("Update", func(t *testing.T) {
		s := f(t)
		e := entity.New("T-1", "ticket")
		e.SetString("title", "v1")
		require.NoError(t, s.CreateEntity(ctx(), e))

		updated := entity.New("T-1", "ticket")
		updated.SetString("title", "v2")
		err := s.UpdateEntity(ctx(), updated)
		require.NoError(t, err)

		got, _ := s.GetEntity(ctx(), "T-1")
		assert.Equal(t, "v2", got.GetString("title"))
	})

	t.Run("UpdateNotFound", func(t *testing.T) {
		s := f(t)
		err := s.UpdateEntity(ctx(), entity.New("NOPE", "ticket"))
		assert.ErrorIs(t, err, store.ErrNotFound)
	})

	t.Run("Delete", func(t *testing.T) {
		s := f(t)
		e := entity.New("T-1", "ticket")
		e.SetString("title", "Bye")
		require.NoError(t, s.CreateEntity(ctx(), e))

		result, err := s.DeleteEntity(ctx(), "T-1", false)
		require.NoError(t, err)
		require.Len(t, result.DeletedEntities, 1)
		assert.Equal(t, "T-1", result.DeletedEntities[0].ID)

		_, err = s.GetEntity(ctx(), "T-1")
		assert.ErrorIs(t, err, store.ErrNotFound)
	})

	t.Run("DeleteNotFound", func(t *testing.T) {
		s := f(t)
		_, err := s.DeleteEntity(ctx(), "NOPE", false)
		assert.ErrorIs(t, err, store.ErrNotFound)
	})

	t.Run("DeleteCascadeRelations", func(t *testing.T) {
		s := f(t)
		require.NoError(t, s.CreateEntity(ctx(), entity.New("A", "feature")))
		require.NoError(t, s.CreateEntity(ctx(), entity.New("B", "req")))
		_, err := s.CreateRelation(ctx(), "A", "requires", "B", nil)
		require.NoError(t, err)

		result, err := s.DeleteEntity(ctx(), "A", true)
		require.NoError(t, err)
		assert.Len(t, result.DeletedRelations, 1)
		assert.Equal(t, "requires", result.DeletedRelations[0].Type)
	})

	t.Run("DeleteCascadeRelationsToSide", func(t *testing.T) {
		s := f(t)
		require.NoError(t, s.CreateEntity(ctx(), entity.New("A", "feature")))
		require.NoError(t, s.CreateEntity(ctx(), entity.New("B", "req")))
		_, err := s.CreateRelation(ctx(), "A", "requires", "B", nil)
		require.NoError(t, err)

		result, err := s.DeleteEntity(ctx(), "B", true)
		require.NoError(t, err)
		assert.Len(t, result.DeletedRelations, 1)
		assert.Equal(t, "requires", result.DeletedRelations[0].Type)

		_, err = s.GetRelation(ctx(), "A", "requires", "B")
		assert.ErrorIs(t, err, store.ErrNotFound)
	})

	t.Run("DeleteNoCascadeWithRelations", func(t *testing.T) {
		s := f(t)
		require.NoError(t, s.CreateEntity(ctx(), entity.New("A", "feature")))
		require.NoError(t, s.CreateEntity(ctx(), entity.New("B", "req")))
		_, err := s.CreateRelation(ctx(), "A", "requires", "B", nil)
		require.NoError(t, err)

		_, err = s.DeleteEntity(ctx(), "A", false)
		assert.ErrorIs(t, err, store.ErrHasRelations)

		_, err = s.GetEntity(ctx(), "A")
		assert.NoError(t, err)
	})

	t.Run("Rename", func(t *testing.T) {
		s := f(t)
		e := entity.New("OLD-1", "ticket")
		e.SetString("title", "Keep me")
		require.NoError(t, s.CreateEntity(ctx(), e))

		result, err := s.RenameEntity(ctx(), "OLD-1", "NEW-1")
		require.NoError(t, err)
		assert.Equal(t, 0, result.RelationsUpdated)

		_, err = s.GetEntity(ctx(), "OLD-1")
		assert.ErrorIs(t, err, store.ErrNotFound)

		got, err := s.GetEntity(ctx(), "NEW-1")
		require.NoError(t, err)
		assert.Equal(t, "Keep me", got.GetString("title"))
	})

	t.Run("RenameUpdatesRelations", func(t *testing.T) {
		s := f(t)
		require.NoError(t, s.CreateEntity(ctx(), entity.New("A", "feature")))
		require.NoError(t, s.CreateEntity(ctx(), entity.New("B", "req")))
		require.NoError(t, s.CreateEntity(ctx(), entity.New("C", "req")))
		s.CreateRelation(ctx(), "A", "requires", "B", nil)
		s.CreateRelation(ctx(), "C", "blocks", "A", nil)

		result, err := s.RenameEntity(ctx(), "A", "A2")
		require.NoError(t, err)
		assert.Equal(t, 2, result.RelationsUpdated)

		_, err = s.GetRelation(ctx(), "A", "requires", "B")
		assert.ErrorIs(t, err, store.ErrNotFound)
		_, err = s.GetRelation(ctx(), "C", "blocks", "A")
		assert.ErrorIs(t, err, store.ErrNotFound)

		r1, err := s.GetRelation(ctx(), "A2", "requires", "B")
		require.NoError(t, err)
		assert.Equal(t, "A2", r1.From)

		r2, err := s.GetRelation(ctx(), "C", "blocks", "A2")
		require.NoError(t, err)
		assert.Equal(t, "A2", r2.To)
	})

	t.Run("RenameNotFound", func(t *testing.T) {
		s := f(t)
		_, err := s.RenameEntity(ctx(), "NOPE", "NEW")
		assert.ErrorIs(t, err, store.ErrNotFound)
	})

	t.Run("RenameConflict", func(t *testing.T) {
		s := f(t)
		require.NoError(t, s.CreateEntity(ctx(), entity.New("A", "feature")))
		require.NoError(t, s.CreateEntity(ctx(), entity.New("B", "feature")))

		_, err := s.RenameEntity(ctx(), "A", "B")
		assert.ErrorIs(t, err, store.ErrConflict)
	})

	t.Run("RenameRejectsDoubleDash", func(t *testing.T) {
		s := f(t)
		require.NoError(t, s.CreateEntity(ctx(), entity.New("A", "feature")))

		_, err := s.RenameEntity(ctx(), "A", "B--C")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "consecutive dashes")
	})

	t.Run("CreateRejectsEmptyID", func(t *testing.T) {
		s := f(t)
		err := s.CreateEntity(ctx(), entity.New("", "feature"))
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "empty ID")
	})

	t.Run("ListStableOrder", func(t *testing.T) {
		s := f(t)
		for _, id := range []string{"C", "A", "B"} {
			require.NoError(t, s.CreateEntity(ctx(), entity.New(id, "t")))
		}

		var ids []string
		for e, err := range s.ListEntities(ctx(), store.EntityQuery{}) {
			require.NoError(t, err)
			ids = append(ids, e.ID)
		}
		assert.Equal(t, []string{"A", "B", "C"}, ids)
	})

	t.Run("ListEarlyBreak", func(t *testing.T) {
		s := f(t)
		require.NoError(t, s.CreateEntity(ctx(), entity.New("A", "t")))
		require.NoError(t, s.CreateEntity(ctx(), entity.New("B", "t")))
		require.NoError(t, s.CreateEntity(ctx(), entity.New("C", "t")))

		var ids []string
		for e, err := range s.ListEntities(ctx(), store.EntityQuery{}) {
			require.NoError(t, err)
			ids = append(ids, e.ID)
			if len(ids) == 1 {
				break
			}
		}
		assert.Len(t, ids, 1)
	})

	t.Run("CountWithIDs", func(t *testing.T) {
		s := f(t)
		require.NoError(t, s.CreateEntity(ctx(), entity.New("A", "feature")))
		require.NoError(t, s.CreateEntity(ctx(), entity.New("B", "feature")))
		require.NoError(t, s.CreateEntity(ctx(), entity.New("C", "req")))

		n, err := s.CountEntities(ctx(), store.EntityQuery{IDs: []string{"A", "C"}})
		require.NoError(t, err)
		assert.Equal(t, 2, n)

		n, err = s.CountEntities(ctx(), store.EntityQuery{Type: "feature", IDs: []string{"A", "C"}})
		require.NoError(t, err)
		assert.Equal(t, 1, n)
	})

	t.Run("RenameSkipsUnrelatedRelations", func(t *testing.T) {
		s := f(t)
		require.NoError(t, s.CreateEntity(ctx(), entity.New("A", "t")))
		require.NoError(t, s.CreateEntity(ctx(), entity.New("B", "t")))
		require.NoError(t, s.CreateEntity(ctx(), entity.New("C", "t")))
		s.CreateRelation(ctx(), "B", "links", "C", nil)

		result, err := s.RenameEntity(ctx(), "A", "A2")
		require.NoError(t, err)
		assert.Equal(t, 0, result.RelationsUpdated)

		r, err := s.GetRelation(ctx(), "B", "links", "C")
		require.NoError(t, err)
		assert.Equal(t, "B", r.From)
	})

	t.Run("RenameEmitsEvent", func(t *testing.T) {
		s := f(t)
		require.NoError(t, s.CreateEntity(ctx(), entity.New("A", "feature")))

		events, cancel := s.Subscribe(10)
		defer cancel()

		_, err := s.RenameEntity(ctx(), "A", "B")
		require.NoError(t, err)

		ev := <-events
		assert.Equal(t, store.EventEntityUpdated, ev.Op)
		assert.Equal(t, "B", ev.EntityID)
		assert.Equal(t, "feature", ev.EntityType)
	})
}
