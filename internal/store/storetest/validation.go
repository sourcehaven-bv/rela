package storetest

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// RunValidationTests runs input validation conformance tests.
func RunValidationTests(t *testing.T, f Factory) {
	t.Run("CreateEntityRejectsInvalidIDs", func(t *testing.T) {
		s := f(t)

		invalid := []struct {
			id   string
			want string
		}{
			{"", "empty"},
			{"foo/bar", "path separator"},
			{"foo\\bar", "path separator"},
			{"foo\x00bar", "control character"},
			{"foo\nbar", "control character"},
			{"foo\tbar", "control character"},
			{"foo\x7fbar", "control character"},
			{"a--b", "consecutive dashes"},
		}
		for _, tc := range invalid {
			err := s.CreateEntity(ctx(), entity.New(tc.id, "t"))
			assert.Errorf(t, err, "id %q should be rejected", tc.id)
			if err != nil {
				assert.Containsf(t, err.Error(), tc.want,
					"error for id %q should mention %q", tc.id, tc.want)
			}
		}
	})

	t.Run("RelationKeyRejectsDoubleDash", func(t *testing.T) {
		s := f(t)

		err := s.CreateEntity(ctx(), entity.New("A--B", "t"))
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "consecutive dashes")

		require.NoError(t, s.CreateEntity(ctx(), entity.New("A-B", "t")))
		require.NoError(t, s.CreateEntity(ctx(), entity.New("C-D", "t")))

		_, err = s.CreateRelation(ctx(), "A-B", "req--ires", "C-D", nil)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "consecutive dashes")

		_, err = s.CreateRelation(ctx(), "A-B", "requires", "C-D", nil)
		require.NoError(t, err)
	})

	t.Run("RenameKeyCollapseDeterministic", func(t *testing.T) {
		s := f(t)

		require.NoError(t, s.CreateEntity(ctx(), entity.New("A", "t")))
		require.NoError(t, s.CreateEntity(ctx(), entity.New("C", "t")))
		require.NoError(t, s.CreateEntity(ctx(), entity.New("X", "t")))

		_, err := s.CreateRelation(ctx(), "A", "requires", "X",
			&store.RelationData{Content: "from-A"})
		require.NoError(t, err)
		_, err = s.CreateRelation(ctx(), "C", "requires", "X",
			&store.RelationData{Content: "from-C"})
		require.NoError(t, err)

		_, err = s.RenameEntity(ctx(), "A", "C")
		assert.ErrorIs(t, err, store.ErrConflict)
	})
}
