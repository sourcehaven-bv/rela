package storetest

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/store"
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
			Properties: map[string]any{"weight": 5},
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

	// EntityIDs is the plural of EntityID under the same Direction/FromFace
	// semantics (TKT-1U8XYN): the batch must equal the union of the scalar
	// calls, nil must mean unfiltered and empty must mean nothing.
	t.Run("ListEntityIDsBatchEqualsUnionOfScalarCalls", func(t *testing.T) {
		s := f(t)
		for _, id := range []string{"A", "B", "C", "D"} {
			require.NoError(t, s.CreateEntity(ctx(), entity.New(id, "node")))
		}
		for _, r := range [][3]string{{"A", "links", "B"}, {"B", "links", "C"}, {"C", "links", "D"}, {"D", "other", "A"}} {
			_, err := s.CreateRelation(ctx(), r[0], r[1], r[2], nil)
			require.NoError(t, err)
		}
		keys := func(q store.RelationQuery) map[string]bool {
			out := map[string]bool{}
			for r, err := range s.ListRelations(ctx(), q) {
				require.NoError(t, err)
				out[r.Key()] = true
			}
			return out
		}
		for _, dir := range []store.Direction{store.DirectionOutgoing, store.DirectionIncoming, store.DirectionBoth} {
			for _, relType := range []string{"", "links"} {
				want := map[string]bool{}
				for _, id := range []string{"A", "C"} {
					for k := range keys(store.RelationQuery{EntityID: id, Direction: dir, Type: relType}) {
						want[k] = true
					}
				}
				got := keys(store.RelationQuery{EntityIDs: []string{"A", "C"}, Direction: dir, Type: relType})
				require.Equal(t, want, got, "direction %v type %q", dir, relType)
			}
		}
		// Duplicated and unknown ids are harmless.
		require.Len(t, keys(store.RelationQuery{EntityIDs: []string{"A", "A", "nope"}, Direction: store.DirectionOutgoing}), 1)
	})

	t.Run("ListEntityIDsNilIsUnfilteredEmptyIsNothing", func(t *testing.T) {
		s := f(t)
		require.NoError(t, s.CreateEntity(ctx(), entity.New("A", "node")))
		require.NoError(t, s.CreateEntity(ctx(), entity.New("B", "node")))
		_, err := s.CreateRelation(ctx(), "A", "links", "B", nil)
		require.NoError(t, err)
		count := func(q store.RelationQuery) int {
			n := 0
			for _, err := range s.ListRelations(ctx(), q) {
				require.NoError(t, err)
				n++
			}
			return n
		}
		require.Equal(t, 1, count(store.RelationQuery{EntityIDs: nil}), "nil EntityIDs must not filter")
		require.Equal(t, 0, count(store.RelationQuery{EntityIDs: []string{}}), "empty EntityIDs must match nothing")
		for _, dir := range []store.Direction{store.DirectionOutgoing, store.DirectionIncoming, store.DirectionBoth} {
			c, err := s.CountRelations(ctx(), store.RelationQuery{EntityIDs: []string{}, Direction: dir})
			require.NoError(t, err)
			require.Equal(t, 0, c)
		}
	})

	t.Run("ListEntityIDsComposesWithEntityID", func(t *testing.T) {
		s := f(t)
		require.NoError(t, s.CreateEntity(ctx(), entity.New("A", "node")))
		require.NoError(t, s.CreateEntity(ctx(), entity.New("B", "node")))
		_, err := s.CreateRelation(ctx(), "A", "links", "B", nil)
		require.NoError(t, err)
		n := 0
		for _, err := range s.ListRelations(ctx(), store.RelationQuery{
			EntityID: "B", EntityIDs: []string{"A"}, Direction: store.DirectionOutgoing,
		}) {
			require.NoError(t, err)
			n++
		}
		require.Equal(t, 0, n, "both filters apply: B is not a source in the batch")
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
			Properties: map[string]any{"weight": 10, "note": "critical"},
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
			Properties: map[string]any{"k": "v"},
			Content:    "original",
		})

		updated, err := s.UpdateRelation(ctx(), "A", "requires", "B", store.RelationData{
			Properties: map[string]any{"k": "new"},
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

	t.Run("CountAll", func(t *testing.T) {
		s := f(t)
		require.NoError(t, s.CreateEntity(ctx(), entity.New("A", "feature")))
		require.NoError(t, s.CreateEntity(ctx(), entity.New("B", "req")))
		require.NoError(t, s.CreateEntity(ctx(), entity.New("C", "req")))
		s.CreateRelation(ctx(), "A", "requires", "B", nil)
		s.CreateRelation(ctx(), "A", "blocks", "C", nil)
		s.CreateRelation(ctx(), "B", "requires", "C", nil)

		n, err := s.CountRelations(ctx(), store.RelationQuery{})
		require.NoError(t, err)
		assert.Equal(t, 3, n)
	})

	t.Run("CountByType", func(t *testing.T) {
		s := f(t)
		require.NoError(t, s.CreateEntity(ctx(), entity.New("A", "feature")))
		require.NoError(t, s.CreateEntity(ctx(), entity.New("B", "req")))
		require.NoError(t, s.CreateEntity(ctx(), entity.New("C", "req")))
		s.CreateRelation(ctx(), "A", "requires", "B", nil)
		s.CreateRelation(ctx(), "A", "blocks", "C", nil)
		s.CreateRelation(ctx(), "B", "requires", "C", nil)

		n, err := s.CountRelations(ctx(), store.RelationQuery{Type: "requires"})
		require.NoError(t, err)
		assert.Equal(t, 2, n)

		n, err = s.CountRelations(ctx(), store.RelationQuery{Type: "nonexistent"})
		require.NoError(t, err)
		assert.Equal(t, 0, n)
	})

	t.Run("CountEmpty", func(t *testing.T) {
		s := f(t)
		n, err := s.CountRelations(ctx(), store.RelationQuery{})
		require.NoError(t, err)
		assert.Equal(t, 0, n)
	})
}
