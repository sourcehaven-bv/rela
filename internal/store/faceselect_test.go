package store_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

func TestFaceSelection_Modes(t *testing.T) {
	draft := entity.Face("draft")
	world := store.NewWorldScope(map[string]store.TypeResolution{
		"doc": {Chain: []entity.Face{draft}, Fallback: store.FallbackDefaultState},
	})

	var zero store.FaceSelection
	assert.True(t, zero.IsZero())
	require.ErrorIs(t, zero.Validate(), store.ErrInvalidQuery)
	assert.False(t, zero.Admits(""), "the zero selection admits nothing")
	assert.Equal(t, "unset", zero.String())

	def := store.InWorld(store.DefaultWorld())
	require.NoError(t, def.Validate())
	assert.True(t, def.IsDefaultWorld())
	assert.Equal(t, "in-world(default)", def.String())

	in := store.InWorld(world)
	w, ok := in.World()
	assert.True(t, ok)
	assert.False(t, w.IsDefaultWorld())
	assert.False(t, in.IsDefaultWorld())
	assert.True(t, in.Admits(draft), "ranking decides per family, after admission")
	assert.Equal(t, "in-world(doc)", in.String())
	_, ok = in.Faces()
	assert.False(t, ok)

	all := store.AllFaces()
	assert.True(t, all.IsAll())
	assert.True(t, all.Admits(draft))
	_, ok = all.World()
	assert.False(t, ok)
	assert.Equal(t, "all-faces", all.String())

	src := []entity.Face{"", draft}
	at := store.AtFaces(src...)
	src[1] = "mutated"
	faces, ok := at.Faces()
	assert.True(t, ok)
	assert.Equal(t, []entity.Face{"", draft}, faces, "AtFaces copies its input")
	faces[0] = "mutated"
	again, _ := at.Faces()
	assert.Equal(t, entity.Face(""), again[0], "Faces returns a copy")
	assert.True(t, at.Admits(draft))
	assert.False(t, at.Admits("published"))
	assert.False(t, at.IsAll())
	assert.Equal(t, `at-faces("","draft")`, at.String())

	none, ok := store.AtFaces().Faces()
	assert.True(t, ok)
	assert.NotNil(t, none)
	assert.Empty(t, none)
	assert.False(t, store.AtFaces().Admits(""), "an empty set matches nothing")
}
