package worlds_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/worlds"
)

// parseSchema loads a schema fixture, failing the test on a load error.
func parseSchema(t *testing.T, yaml string) *metamodel.Metamodel {
	t.Helper()
	m, err := metamodel.Parse([]byte(yaml))
	require.NoError(t, err)
	return m
}

const pointeredSchema = `version: "1.0"
namespace: https://example.org/test#
entities:
  page:
    label: Page
    id_prefix: PAGE
    properties: {title: {type: string}}
    pointers:
      draft: {default: true}
      published: {}
  policy:
    label: Policy
    id_prefix: POL
    properties: {title: {type: string}}
    pointers:
      draft: {default: true}
      review: {}
      published: {}
  ticket:
    label: Ticket
    id_prefix: TKT
    properties: {title: {type: string}}
worlds:
  published:
    select: published
    otherwise: exclude
  editorial:
    select: [review, published]
    overrides:
      page: draft
    otherwise: default
`

func ptr(t *testing.T, s string) entity.Pointer {
	t.Helper()
	p, err := entity.ParsePointer(s)
	require.NoError(t, err)
	return p
}

// TestCompile_PointerlessProjectIsTheDefaultWorld pins AC1 and the whole
// compatibility story: a metamodel with no worlds and no pointers yields
// nothing to resolve, and the default world stays available and total.
func TestCompile_PointerlessProjectIsTheDefaultWorld(t *testing.T) {
	m := parseSchema(t, `version: "1.0"
namespace: https://example.org/test#
entities:
  ticket:
    label: Ticket
    id_prefix: TKT
    properties: {title: {type: string}}
`)
	c, err := worlds.Compile(m)
	require.NoError(t, err)
	assert.Empty(t, c.Names(), "no worlds declared")

	scope, ok := c.Lookup(metamodel.DefaultWorldName)
	require.True(t, ok, "the default world is always available")
	assert.True(t, scope.IsDefaultWorld())
	assert.Nil(t, scope.Types(), "the default world allocates no per-type map")

	// And an undeclared name fails closed rather than degrading.
	_, ok = c.Lookup("published")
	assert.False(t, ok, "an undeclared world must not resolve")
}

func TestCompile_NilMetamodel(t *testing.T) {
	c, err := worlds.Compile(nil)
	require.NoError(t, err)
	scope, ok := c.Lookup(metamodel.DefaultWorldName)
	require.True(t, ok)
	assert.True(t, scope.IsDefaultWorld())
}

// TestCompile_ChainsAndFallback covers the three resolution rules as they
// appear in the compiled form.
func TestCompile_ChainsAndFallback(t *testing.T) {
	c, err := worlds.Compile(parseSchema(t, pointeredSchema))
	require.NoError(t, err)
	assert.Equal(t, []string{"editorial", "published"}, c.Names())

	t.Run("rule 1: a pointerless type is ABSENT, not excluded", func(t *testing.T) {
		for _, name := range []string{"published", "editorial"} {
			scope, ok := c.Lookup(name)
			require.True(t, ok)
			_, declared := scope.For("ticket")
			assert.False(t, declared,
				"world %q must not carry a resolution for the pointerless type", name)
		}
	})

	t.Run("rule 2: the chain is ordered and type-filtered", func(t *testing.T) {
		scope, ok := c.Lookup("editorial")
		require.True(t, ok)

		// policy declares review AND published: both survive, in order.
		res, ok := scope.For("policy")
		require.True(t, ok)
		assert.Equal(t, []entity.Pointer{ptr(t, "review"), ptr(t, "published")}, res.Chain)

		// page declares published, which the world's global chain selects —
		// but the per-type override replaces that chain entirely.
		res, ok = scope.For("page")
		require.True(t, ok)
		assert.Equal(t, []entity.Pointer{ptr(t, "draft")}, res.Chain,
			"the override replaces the global chain, it does not extend it")
	})

	t.Run("rule 3: otherwise compiles to the fallback verdict", func(t *testing.T) {
		pub, ok := c.Lookup("published")
		require.True(t, ok)
		res, ok := pub.For("policy")
		require.True(t, ok)
		assert.Equal(t, store.FallbackExclude, res.Fallback)

		ed, ok := c.Lookup("editorial")
		require.True(t, ok)
		res, ok = ed.For("policy")
		require.True(t, ok)
		assert.Equal(t, store.FallbackDefaultState, res.Fallback)
	})

	t.Run("rule 3: a type the chain cannot satisfy compiles to an EMPTY chain", func(t *testing.T) {
		// The subtest above asserts the fallback FIELD is populated, but
		// `policy` declares review and published so its chain is non-empty
		// in both worlds — rule 3 is never the operative rule there. Rule 3
		// is the empty chain: a type declares pointers, the world selects
		// none of them, so `otherwise:` is the only answer left.
		m := parseSchema(t, `version: "1.0"
namespace: https://example.org/test#
entities:
  memo:
    label: Memo
    id_prefix: MEMO
    properties: {title: {type: string}}
    pointers:
      draft: {default: true}
      archived: {}
  note:
    label: Note
    id_prefix: NOTE
    properties: {title: {type: string}}
    pointers:
      draft: {default: true}
      published: {}
worlds:
  published:
    select: published
    otherwise: exclude
  lenient:
    select: published
    otherwise: default
`)
		// `note` declares `published` so the world is declarable, while
		// `memo` declares only draft/archived — so memo's chain is EMPTY
		// and `otherwise:` is the only thing that can resolve it.
		compiled, err := worlds.Compile(m)
		require.NoError(t, err)

		for _, tc := range []struct {
			world string
			want  store.Fallback
		}{
			{"published", store.FallbackExclude},
			{"lenient", store.FallbackDefaultState},
		} {
			scope, ok := compiled.Lookup(tc.world)
			require.True(t, ok)
			res, ok := scope.For("memo")
			require.True(t, ok, "memo declares pointers, so it must have a resolution")
			assert.Empty(t, res.Chain,
				"memo declares no coordinate this world selects: rule 3, empty chain")
			assert.Equal(t, tc.want, res.Fallback,
				"with an empty chain every memo resolves via otherwise: %s (world %q)", tc.want, tc.world)
		}
	})

	t.Run("a chain coordinate the type lacks is dropped, not an error", func(t *testing.T) {
		// `page` does not declare `review`; the editorial chain is
		// [review, published] but page is overridden, so use published's
		// world against a type that lacks the coordinate instead.
		scope, ok := c.Lookup("published")
		require.True(t, ok)
		res, ok := scope.For("page")
		require.True(t, ok)
		assert.Equal(t, []entity.Pointer{ptr(t, "published")}, res.Chain)
	})
}

// TestCompile_ChainDedup pins that a repeated coordinate collapses rather
// than being ranked twice (design doc §4.5's dedup rule, which applies
// even without templates).
func TestCompile_ChainDedup(t *testing.T) {
	c, err := worlds.Compile(parseSchema(t, `version: "1.0"
namespace: https://example.org/test#
entities:
  page:
    label: Page
    id_prefix: PAGE
    properties: {title: {type: string}}
    pointers:
      draft: {default: true}
      published: {}
worlds:
  repetitive:
    select: [published, draft, published]
    otherwise: exclude
`))
	require.NoError(t, err)
	scope, ok := c.Lookup("repetitive")
	require.True(t, ok)
	res, ok := scope.For("page")
	require.True(t, ok)
	assert.Equal(t, []entity.Pointer{ptr(t, "published"), ptr(t, "draft")}, res.Chain)
}

// TestCompile_RejectsBadPointerGrammar pins that the pointer grammar is
// enforced at compile time (it cannot live in the loader: metamodel may
// not import entity), and that the message is as diagnosable as a loader
// error — naming the type, the offending name, and the rule.
func TestCompile_RejectsBadPointerGrammar(t *testing.T) {
	tests := []struct {
		name string
		bad  string
	}{
		{"uppercase", "Draft"},
		{"leading digit", "1draft"},
		{"consecutive hyphens", "in--review"},
		{"trailing hyphen", "draft-"},
		{"underscore", "in_review"},
		{"reserved multi-axis separator", "nl+draft"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			m := parseSchema(t, `version: "1.0"
namespace: https://example.org/test#
entities:
  page:
    label: Page
    id_prefix: PAGE
    properties: {title: {type: string}}
    pointers:
      "`+tc.bad+`": {}
`)
			_, err := worlds.Compile(m)
			require.Error(t, err, "bad pointer name %q must not compile", tc.bad)
			assert.Contains(t, err.Error(), `entity "page"`)
			assert.Contains(t, err.Error(), tc.bad)
		})
	}
}

// TestCompile_ReportsEveryProblem pins the collect-then-report discipline
// the loader uses: an operator fixing a schema sees the whole list.
func TestCompile_ReportsEveryProblem(t *testing.T) {
	m := parseSchema(t, `version: "1.0"
namespace: https://example.org/test#
entities:
  page:
    label: Page
    id_prefix: PAGE
    properties: {title: {type: string}}
    pointers:
      Draft: {}
  policy:
    label: Policy
    id_prefix: POL
    properties: {title: {type: string}}
    pointers:
      Review: {}
`)
	_, err := worlds.Compile(m)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "2 problems")
	assert.Contains(t, err.Error(), "Draft")
	assert.Contains(t, err.Error(), "Review")
}

// TestCompile_GrammarCheckedWithoutWorlds pins that declaring states
// without declaring any world still validates their names — a project may
// legitimately do the two in either order.
func TestCompile_GrammarCheckedWithoutWorlds(t *testing.T) {
	m := parseSchema(t, `version: "1.0"
namespace: https://example.org/test#
entities:
  page:
    label: Page
    id_prefix: PAGE
    properties: {title: {type: string}}
    pointers:
      BAD: {}
`)
	_, err := worlds.Compile(m)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "BAD")
}

// TestDefault is the trivial-but-load-bearing accessor pin.
func TestDefault(t *testing.T) {
	assert.True(t, worlds.Default().IsDefaultWorld())
}
