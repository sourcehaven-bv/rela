package storetest

import (
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// RunRelationTests runs relation CRUD conformance tests.
func RunRelationTests(t *testing.T, f Factory) {
	t.Run("CreateAndGet", func(t *testing.T) {
		s := f(t)
		require.NoError(t, s.CreateEntity(ctx(), entity.New("A", "feature")))
		require.NoError(t, s.CreateEntity(ctx(), entity.New("B", "req")))

		r, err := s.CreateRelation(ctx(), "A", "requires", "B", nil)
		require.NoError(t, err)
		assert.Equal(t, "A", r.From)
		assert.Equal(t, "requires", r.Type)
		assert.Equal(t, "B", r.To)

		got, err := s.GetRelation(ctx(), "A", "requires", "B")
		require.NoError(t, err)
		assert.Equal(t, "A", got.From)
	})

	t.Run("CreateWithData", func(t *testing.T) {
		s := f(t)
		require.NoError(t, s.CreateEntity(ctx(), entity.New("A", "feature")))
		require.NoError(t, s.CreateEntity(ctx(), entity.New("B", "req")))

		data := &store.RelationData{
			Properties: map[string]interface{}{"weight": 5},
			Content:    "important link",
		}
		r, err := s.CreateRelation(ctx(), "A", "requires", "B", data)
		require.NoError(t, err)
		assert.Equal(t, 5, r.Properties["weight"])
		assert.Equal(t, "important link", r.Content)
	})

	t.Run("CreateConflict", func(t *testing.T) {
		s := f(t)
		require.NoError(t, s.CreateEntity(ctx(), entity.New("A", "feature")))
		require.NoError(t, s.CreateEntity(ctx(), entity.New("B", "req")))

		_, err := s.CreateRelation(ctx(), "A", "requires", "B", nil)
		require.NoError(t, err)

		_, err = s.CreateRelation(ctx(), "A", "requires", "B", nil)
		assert.ErrorIs(t, err, store.ErrConflict)
	})

	t.Run("GetNotFound", func(t *testing.T) {
		s := f(t)
		_, err := s.GetRelation(ctx(), "X", "nope", "Y")
		assert.ErrorIs(t, err, store.ErrNotFound)
	})

	t.Run("Update", func(t *testing.T) {
		s := f(t)
		require.NoError(t, s.CreateEntity(ctx(), entity.New("A", "feature")))
		require.NoError(t, s.CreateEntity(ctx(), entity.New("B", "req")))
		_, err := s.CreateRelation(ctx(), "A", "requires", "B", nil)
		require.NoError(t, err)

		updated, err := s.UpdateRelation(ctx(), "A", "requires", "B", store.RelationData{
			Content: "updated content",
		})
		require.NoError(t, err)
		assert.Equal(t, "updated content", updated.Content)

		got, _ := s.GetRelation(ctx(), "A", "requires", "B")
		assert.Equal(t, "updated content", got.Content)
	})

	t.Run("UpdateNotFound", func(t *testing.T) {
		s := f(t)
		_, err := s.UpdateRelation(ctx(), "X", "nope", "Y", store.RelationData{})
		assert.ErrorIs(t, err, store.ErrNotFound)
	})

	t.Run("Delete", func(t *testing.T) {
		s := f(t)
		require.NoError(t, s.CreateEntity(ctx(), entity.New("A", "feature")))
		require.NoError(t, s.CreateEntity(ctx(), entity.New("B", "req")))
		_, _ = s.CreateRelation(ctx(), "A", "requires", "B", nil)

		err := s.DeleteRelation(ctx(), "A", "requires", "B")
		require.NoError(t, err)

		_, err = s.GetRelation(ctx(), "A", "requires", "B")
		assert.ErrorIs(t, err, store.ErrNotFound)
	})

	t.Run("DeleteNotFound", func(t *testing.T) {
		s := f(t)
		err := s.DeleteRelation(ctx(), "X", "nope", "Y")
		assert.ErrorIs(t, err, store.ErrNotFound)
	})

	t.Run("ListAll", func(t *testing.T) {
		s := f(t)
		require.NoError(t, s.CreateEntity(ctx(), entity.New("A", "feature")))
		require.NoError(t, s.CreateEntity(ctx(), entity.New("B", "req")))
		require.NoError(t, s.CreateEntity(ctx(), entity.New("C", "req")))
		s.CreateRelation(ctx(), "A", "requires", "B", nil)
		s.CreateRelation(ctx(), "A", "blocks", "C", nil)
		s.CreateRelation(ctx(), "B", "requires", "C", nil)

		var keys []string
		for r, err := range s.ListRelations(ctx(), store.RelationQuery{}) {
			require.NoError(t, err)
			keys = append(keys, r.Key())
		}
		assert.Len(t, keys, 3)
	})

	t.Run("ListByType", func(t *testing.T) {
		s := f(t)
		require.NoError(t, s.CreateEntity(ctx(), entity.New("A", "feature")))
		require.NoError(t, s.CreateEntity(ctx(), entity.New("B", "req")))
		require.NoError(t, s.CreateEntity(ctx(), entity.New("C", "req")))
		s.CreateRelation(ctx(), "A", "requires", "B", nil)
		s.CreateRelation(ctx(), "A", "blocks", "C", nil)

		var keys []string
		for r, err := range s.ListRelations(ctx(), store.RelationQuery{Type: "requires"}) {
			require.NoError(t, err)
			keys = append(keys, r.Key())
		}
		assert.Len(t, keys, 1)
		assert.Equal(t, "A--requires--B", keys[0])
	})

	t.Run("ListByFrom", func(t *testing.T) {
		s := f(t)
		require.NoError(t, s.CreateEntity(ctx(), entity.New("A", "feature")))
		require.NoError(t, s.CreateEntity(ctx(), entity.New("B", "req")))
		require.NoError(t, s.CreateEntity(ctx(), entity.New("C", "req")))
		s.CreateRelation(ctx(), "A", "requires", "B", nil)
		s.CreateRelation(ctx(), "C", "requires", "B", nil)

		var keys []string
		for r, err := range s.ListRelations(ctx(), store.RelationQuery{From: "A"}) {
			require.NoError(t, err)
			keys = append(keys, r.Key())
		}
		assert.Len(t, keys, 1)
		assert.Equal(t, "A--requires--B", keys[0])
	})

	t.Run("ListByTo", func(t *testing.T) {
		s := f(t)
		require.NoError(t, s.CreateEntity(ctx(), entity.New("A", "t")))
		require.NoError(t, s.CreateEntity(ctx(), entity.New("B", "t")))
		require.NoError(t, s.CreateEntity(ctx(), entity.New("C", "t")))
		s.CreateRelation(ctx(), "A", "requires", "B", nil)
		s.CreateRelation(ctx(), "A", "blocks", "C", nil)

		var keys []string
		for r, err := range s.ListRelations(ctx(), store.RelationQuery{To: "C"}) {
			require.NoError(t, err)
			keys = append(keys, r.Key())
		}
		assert.Len(t, keys, 1)
		assert.Equal(t, "A--blocks--C", keys[0])
	})

	t.Run("ListEntityIDOutgoing", func(t *testing.T) {
		s := f(t)
		require.NoError(t, s.CreateEntity(ctx(), entity.New("A", "feature")))
		require.NoError(t, s.CreateEntity(ctx(), entity.New("B", "req")))
		require.NoError(t, s.CreateEntity(ctx(), entity.New("C", "req")))
		s.CreateRelation(ctx(), "A", "requires", "B", nil)
		s.CreateRelation(ctx(), "C", "blocks", "A", nil)

		var keys []string
		q := store.RelationQuery{EntityID: "A", Direction: store.DirectionOutgoing}
		for r, err := range s.ListRelations(ctx(), q) {
			require.NoError(t, err)
			keys = append(keys, r.Key())
		}
		assert.Len(t, keys, 1)
		assert.Equal(t, "A--requires--B", keys[0])
	})

	t.Run("ListEntityIDIncoming", func(t *testing.T) {
		s := f(t)
		require.NoError(t, s.CreateEntity(ctx(), entity.New("A", "feature")))
		require.NoError(t, s.CreateEntity(ctx(), entity.New("B", "req")))
		require.NoError(t, s.CreateEntity(ctx(), entity.New("C", "req")))
		s.CreateRelation(ctx(), "A", "requires", "B", nil)
		s.CreateRelation(ctx(), "C", "blocks", "A", nil)

		var keys []string
		q := store.RelationQuery{EntityID: "A", Direction: store.DirectionIncoming}
		for r, err := range s.ListRelations(ctx(), q) {
			require.NoError(t, err)
			keys = append(keys, r.Key())
		}
		assert.Len(t, keys, 1)
		assert.Equal(t, "C--blocks--A", keys[0])
	})

	t.Run("ListEntityIDBoth", func(t *testing.T) {
		s := f(t)
		require.NoError(t, s.CreateEntity(ctx(), entity.New("A", "feature")))
		require.NoError(t, s.CreateEntity(ctx(), entity.New("B", "req")))
		require.NoError(t, s.CreateEntity(ctx(), entity.New("C", "req")))
		s.CreateRelation(ctx(), "A", "requires", "B", nil)
		s.CreateRelation(ctx(), "C", "blocks", "A", nil)

		var keys []string
		q := store.RelationQuery{EntityID: "A", Direction: store.DirectionBoth}
		for r, err := range s.ListRelations(ctx(), q) {
			require.NoError(t, err)
			keys = append(keys, r.Key())
		}
		assert.Len(t, keys, 2)
	})

	t.Run("ListEntityIDBothNoMatch", func(t *testing.T) {
		s := f(t)
		require.NoError(t, s.CreateEntity(ctx(), entity.New("A", "t")))
		require.NoError(t, s.CreateEntity(ctx(), entity.New("B", "t")))
		s.CreateRelation(ctx(), "A", "requires", "B", nil)

		var keys []string
		q := store.RelationQuery{EntityID: "C", Direction: store.DirectionBoth}
		for r, err := range s.ListRelations(ctx(), q) {
			require.NoError(t, err)
			keys = append(keys, r.Key())
		}
		assert.Empty(t, keys)
	})

	t.Run("CreateRejectsEmptyFrom", func(t *testing.T) {
		s := f(t)
		require.NoError(t, s.CreateEntity(ctx(), entity.New("B", "req")))

		_, err := s.CreateRelation(ctx(), "", "requires", "B", nil)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "empty ID")
	})

	t.Run("CreateRejectsEmptyType", func(t *testing.T) {
		s := f(t)
		require.NoError(t, s.CreateEntity(ctx(), entity.New("A", "feature")))
		require.NoError(t, s.CreateEntity(ctx(), entity.New("B", "req")))

		_, err := s.CreateRelation(ctx(), "A", "", "B", nil)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "empty relation type")
	})

	t.Run("UpdateWithProperties", func(t *testing.T) {
		s := f(t)
		require.NoError(t, s.CreateEntity(ctx(), entity.New("A", "feature")))
		require.NoError(t, s.CreateEntity(ctx(), entity.New("B", "req")))
		_, err := s.CreateRelation(ctx(), "A", "requires", "B", nil)
		require.NoError(t, err)

		updated, err := s.UpdateRelation(ctx(), "A", "requires", "B", store.RelationData{
			Properties: map[string]interface{}{"weight": 10, "note": "critical"},
			Content:    "updated",
		})
		require.NoError(t, err)
		assert.Equal(t, 10, updated.Properties["weight"])
		assert.Equal(t, "critical", updated.Properties["note"])
		assert.Equal(t, "updated", updated.Content)

		got, _ := s.GetRelation(ctx(), "A", "requires", "B")
		assert.Equal(t, 10, got.Properties["weight"])
	})

	t.Run("UpdateReturnsClone", func(t *testing.T) {
		s := f(t)
		require.NoError(t, s.CreateEntity(ctx(), entity.New("A", "feature")))
		require.NoError(t, s.CreateEntity(ctx(), entity.New("B", "req")))
		_, _ = s.CreateRelation(ctx(), "A", "requires", "B", &store.RelationData{
			Properties: map[string]interface{}{"k": "v"},
			Content:    "original",
		})

		updated, err := s.UpdateRelation(ctx(), "A", "requires", "B", store.RelationData{
			Properties: map[string]interface{}{"k": "new"},
			Content:    "new",
		})
		require.NoError(t, err)

		updated.Content = "mutated"
		updated.Properties["k"] = "mutated"

		got, _ := s.GetRelation(ctx(), "A", "requires", "B")
		assert.Equal(t, "new", got.Content)
		assert.Equal(t, "new", got.Properties["k"])
	})

	t.Run("GetReturnsClone", func(t *testing.T) {
		s := f(t)
		require.NoError(t, s.CreateEntity(ctx(), entity.New("A", "feature")))
		require.NoError(t, s.CreateEntity(ctx(), entity.New("B", "req")))
		s.CreateRelation(ctx(), "A", "requires", "B", &store.RelationData{
			Content: "original",
		})

		got, _ := s.GetRelation(ctx(), "A", "requires", "B")
		got.Content = "mutated"

		got2, _ := s.GetRelation(ctx(), "A", "requires", "B")
		assert.Equal(t, "original", got2.Content)
	})

	t.Run("ListEarlyBreak", func(t *testing.T) {
		s := f(t)
		require.NoError(t, s.CreateEntity(ctx(), entity.New("A", "t")))
		require.NoError(t, s.CreateEntity(ctx(), entity.New("B", "t")))
		require.NoError(t, s.CreateEntity(ctx(), entity.New("C", "t")))
		s.CreateRelation(ctx(), "A", "r1", "B", nil)
		s.CreateRelation(ctx(), "A", "r2", "C", nil)

		var keys []string
		for r, err := range s.ListRelations(ctx(), store.RelationQuery{}) {
			require.NoError(t, err)
			keys = append(keys, r.Key())
			if len(keys) == 1 {
				break
			}
		}
		assert.Len(t, keys, 1)
	})

	t.Run("ListStableOrder", func(t *testing.T) {
		s := f(t)
		require.NoError(t, s.CreateEntity(ctx(), entity.New("A", "t")))
		require.NoError(t, s.CreateEntity(ctx(), entity.New("B", "t")))
		require.NoError(t, s.CreateEntity(ctx(), entity.New("C", "t")))
		s.CreateRelation(ctx(), "C", "z", "A", nil)
		s.CreateRelation(ctx(), "A", "a", "B", nil)
		s.CreateRelation(ctx(), "B", "m", "C", nil)

		var keys []string
		for r, err := range s.ListRelations(ctx(), store.RelationQuery{}) {
			require.NoError(t, err)
			keys = append(keys, r.Key())
		}
		assert.Equal(t, []string{"A--a--B", "B--m--C", "C--z--A"}, keys)
	})
}
