package storetest

import (
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// RunQueryTests runs entity query conformance tests.
func RunQueryTests(t *testing.T, f Factory) {
	t.Run("ListAll", func(t *testing.T) {
		s := f(t)
		seedEntities(t, s)

		var ids []string
		for e, err := range s.ListEntities(ctx(), store.EntityQuery{}) {
			require.NoError(t, err)
			ids = append(ids, e.ID)
		}
		assert.Len(t, ids, 4)
	})

	t.Run("ListByType", func(t *testing.T) {
		s := f(t)
		seedEntities(t, s)

		var ids []string
		for e, err := range s.ListEntities(ctx(), store.EntityQuery{Type: "feature"}) {
			require.NoError(t, err)
			ids = append(ids, e.ID)
		}
		assert.Len(t, ids, 3)
	})

	t.Run("ListByIDs", func(t *testing.T) {
		s := f(t)
		seedEntities(t, s)

		var ids []string
		for e, err := range s.ListEntities(ctx(), store.EntityQuery{IDs: []string{"FEAT-001", "REQ-001"}}) {
			require.NoError(t, err)
			ids = append(ids, e.ID)
		}
		assert.Len(t, ids, 2)
	})

	t.Run("ListByTypeAndIDs", func(t *testing.T) {
		s := f(t)
		seedEntities(t, s)

		var ids []string
		q := store.EntityQuery{Type: "feature", IDs: []string{"FEAT-001", "REQ-001"}}
		for e, err := range s.ListEntities(ctx(), q) {
			require.NoError(t, err)
			ids = append(ids, e.ID)
		}
		assert.Len(t, ids, 1)
		assert.Contains(t, ids, "FEAT-001")
	})

	t.Run("Count", func(t *testing.T) {
		s := f(t)
		seedEntities(t, s)

		n, err := s.CountEntities(ctx(), store.EntityQuery{})
		require.NoError(t, err)
		assert.Equal(t, 4, n)

		n, err = s.CountEntities(ctx(), store.EntityQuery{Type: "feature"})
		require.NoError(t, err)
		assert.Equal(t, 3, n)

		n, err = s.CountEntities(ctx(), store.EntityQuery{Type: "nonexistent"})
		require.NoError(t, err)
		assert.Equal(t, 0, n)
	})

	t.Run("HighestID", func(t *testing.T) {
		s := f(t)
		seedEntities(t, s)

		n, err := s.HighestID(ctx(), "FEAT")
		require.NoError(t, err)
		assert.Equal(t, 13, n)

		n, err = s.HighestID(ctx(), "REQ")
		require.NoError(t, err)
		assert.Equal(t, 1, n)

		n, err = s.HighestID(ctx(), "NOPE")
		require.NoError(t, err)
		assert.Equal(t, 0, n)
	})

	t.Run("PropertyValues", func(t *testing.T) {
		s := f(t)
		seedEntities(t, s)

		vals, err := s.PropertyValues(ctx(), "status", 10)
		require.NoError(t, err)
		require.Len(t, vals, 2)
		assert.Equal(t, "open", vals[0])
		assert.Equal(t, "done", vals[1])
	})

	t.Run("PropertyValuesWithLimit", func(t *testing.T) {
		s := f(t)
		seedEntities(t, s)

		vals, err := s.PropertyValues(ctx(), "status", 1)
		require.NoError(t, err)
		assert.Len(t, vals, 1)
		assert.Equal(t, "open", vals[0])
	})

	t.Run("PropertyValuesTiebreakAlphabetical", func(t *testing.T) {
		s := f(t)
		e1 := entity.New("A", "t")
		e1.SetString("color", "red")
		e2 := entity.New("B", "t")
		e2.SetString("color", "blue")
		e3 := entity.New("C", "t")
		e3.SetString("color", "green")
		require.NoError(t, s.CreateEntity(ctx(), e1))
		require.NoError(t, s.CreateEntity(ctx(), e2))
		require.NoError(t, s.CreateEntity(ctx(), e3))

		vals, err := s.PropertyValues(ctx(), "color", 10)
		require.NoError(t, err)
		require.Len(t, vals, 3)
		assert.Equal(t, []string{"blue", "green", "red"}, vals)
	})

	t.Run("PropertyValuesFrequencyBeatsAlpha", func(t *testing.T) {
		s := f(t)
		e1 := entity.New("A", "t")
		e1.SetString("color", "zebra")
		e2 := entity.New("B", "t")
		e2.SetString("color", "zebra")
		e3 := entity.New("C", "t")
		e3.SetString("color", "alpha")
		require.NoError(t, s.CreateEntity(ctx(), e1))
		require.NoError(t, s.CreateEntity(ctx(), e2))
		require.NoError(t, s.CreateEntity(ctx(), e3))

		vals, err := s.PropertyValues(ctx(), "color", 10)
		require.NoError(t, err)
		require.Len(t, vals, 2)
		assert.Equal(t, "zebra", vals[0], "higher frequency should come first")
		assert.Equal(t, "alpha", vals[1])
	})

	t.Run("PropertyValuesUnknownProperty", func(t *testing.T) {
		s := f(t)
		seedEntities(t, s)

		vals, err := s.PropertyValues(ctx(), "nonexistent", 10)
		require.NoError(t, err)
		assert.Empty(t, vals)
	})
}
