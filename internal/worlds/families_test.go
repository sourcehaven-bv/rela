package worlds_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/worlds"
)

// TestCompiled_Families pins the scope that picks one row per entity with no
// world: faced types rank every declared face in declaration order and
// exclude an entity with none; faceless types stay absent (TKT-7IZHP0 §3.4).
func TestCompiled_Families(t *testing.T) {
	t.Run("zero value is trivial", func(t *testing.T) {
		assert.True(t, worlds.Compiled{}.Families().IsSet())
		assert.True(t, worlds.Compiled{}.Families().IsTrivial())
	})

	t.Run("faceless project is trivial", func(t *testing.T) {
		c, err := worlds.Compile(parseSchema(t, `version: "1.0"
namespace: https://example.org/test#
entities:
  ticket:
    label: Ticket
    id_prefix: TKT
    properties: {title: {type: string}}
`))
		require.NoError(t, err)
		assert.True(t, c.Families().IsTrivial())
	})

	withoutWorlds, _, found := strings.Cut(facedSchema, "worlds:")
	require.True(t, found, "facedSchema must declare worlds")
	for name, schema := range map[string]string{
		"with worlds":    facedSchema,
		"without worlds": withoutWorlds,
	} {
		t.Run(name, func(t *testing.T) {
			c, err := worlds.Compile(parseSchema(t, schema))
			require.NoError(t, err)
			fam := c.Families()

			res, ok := fam.For("policy")
			require.True(t, ok)
			assert.Equal(t, []entity.Face{ptr(t, "draft"), ptr(t, "review"), ptr(t, "published")}, res.Chain)
			assert.Equal(t, store.FallbackExclude, res.Fallback)

			_, ok = fam.For("ticket")
			assert.False(t, ok, "a faceless type reads its implicit face")
		})
	}
}
