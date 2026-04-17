package storetest

import (
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// RunValidationTests runs input validation conformance tests.
func RunValidationTests(t *testing.T, f Factory) {
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

	t.Run("AttachmentKeyRejectsSlash", func(t *testing.T) {
		s := f(t)
		require.NoError(t, s.CreateEntity(ctx(), entity.New("E-1", "t")))

		err := s.AttachFile(ctx(), "E-1", "some/prop", "f.txt", strings.NewReader("data"))
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "slash")

		err = s.AttachFile(ctx(), "E-1", "screenshot", "f.png", strings.NewReader("data"))
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
