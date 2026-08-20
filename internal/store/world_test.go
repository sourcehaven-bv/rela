package store_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// TestWorldScope_ZeroValueIsTheDefaultWorld pins the compatibility
// guarantee: the zero value resolves every entity to its default state,
// costs no allocation, and is what every existing query construction site
// keeps meaning.
func TestWorldScope_ZeroValueIsTheDefaultWorld(t *testing.T) {
	var zero store.WorldScope
	assert.True(t, zero.IsDefaultWorld())
	assert.Nil(t, zero.Types())
	assert.True(t, store.DefaultWorld().IsDefaultWorld())

	// An empty or nil map compiles to the same thing — a caller must not
	// be able to build a non-default world that resolves nothing.
	assert.True(t, store.NewWorldScope(nil).IsDefaultWorld())
	assert.True(t, store.NewWorldScope(map[string]store.TypeResolution{}).IsDefaultWorld())
}

// TestWorldScope_AbsenceIsNotTheZeroValue is the RR-CZN30X pin, and the
// reason the map is unexported.
//
// A type ABSENT from the scope contributes its default state (rule 1).
// The zero TypeResolution EXCLUDES. Those are opposite verdicts, and a
// bare map index would silently return the second for the first. For
// reports the distinction; nothing else may.
func TestWorldScope_AbsenceIsNotTheZeroValue(t *testing.T) {
	scope := store.NewWorldScope(map[string]store.TypeResolution{
		"page": {Chain: []entity.Pointer{"published"}, Fallback: store.FallbackExclude},
	})

	res, ok := scope.For("page")
	require.True(t, ok, "a declared type reports ok=true")
	assert.Equal(t, []entity.Pointer{entity.Pointer("published")}, res.Chain)

	res, ok = scope.For("ticket")
	assert.False(t, ok, "an ABSENT type must report ok=false, meaning rule 1 (default state)")
	assert.Equal(t, store.TypeResolution{}, res,
		"the value on a miss is the zero TypeResolution, which is why callers MUST check ok: "+
			"read as a verdict it would mean 'exclude', the opposite of what absence means")
}

// TestFallback_ZeroValueExcludes pins the fail-closed direction: the zero
// Fallback hides content rather than leaking a draft into a published
// world. Note this reads as the OPPOSITE polarity from WorldScope's zero
// value, deliberately — see the type docs.
func TestFallback_ZeroValueExcludes(t *testing.T) {
	var zero store.Fallback
	assert.Equal(t, store.FallbackExclude, zero,
		"a default-constructed fallback must hide, never reveal")
	assert.NotEqual(t, store.FallbackExclude, store.FallbackDefaultState)
}

// TestNewWorldScope_CopiesInput pins that a WorldScope cannot change
// underfoot: it is handed to every backend, so sharing the caller's map
// would let one mutation re-scope live readers.
func TestNewWorldScope_CopiesInput(t *testing.T) {
	src := map[string]store.TypeResolution{
		"page": {Chain: []entity.Pointer{"published"}},
	}
	scope := store.NewWorldScope(src)

	src["page"] = store.TypeResolution{Chain: []entity.Pointer{"draft"}}
	src["policy"] = store.TypeResolution{Chain: []entity.Pointer{"draft"}}

	res, ok := scope.For("page")
	require.True(t, ok)
	assert.Equal(t, []entity.Pointer{entity.Pointer("published")}, res.Chain,
		"mutating the source map must not re-scope an existing WorldScope")
	_, ok = scope.For("policy")
	assert.False(t, ok, "a type added to the source map after construction must not appear")
}

func TestWorldScope_Types(t *testing.T) {
	scope := store.NewWorldScope(map[string]store.TypeResolution{
		"page":   {Chain: []entity.Pointer{"published"}},
		"policy": {Chain: []entity.Pointer{"review"}},
	})
	assert.ElementsMatch(t, []string{"page", "policy"}, scope.Types())
}
