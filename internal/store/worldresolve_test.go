package store_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// The wire vocabulary must round-trip. ResolutionRule.String() is what a
// provenance field carries over HTTP and into logs, and ParseResolutionRule is
// how it comes back — a rule that stringifies to something Parse cannot read
// would silently degrade to "unscoped" on the way home, turning "this is a
// stand-in face" into "no world applied".
func TestResolutionRule_RoundTrips(t *testing.T) {
	t.Parallel()
	for _, rule := range []store.ResolutionRule{
		store.ResolutionUnscoped,
		store.ResolutionChain,
		store.ResolutionFallbackDefault,
		store.ResolutionExcluded,
	} {
		assert.Equal(t, rule, store.ParseResolutionRule(rule.String()),
			"rule %d did not survive String -> Parse", int(rule))
	}
}

// An unknown name reads as Unscoped — the rule that applies when no resolution
// was recorded. Documented as deliberate, and pinned because the alternative
// (an out-of-range rule) would render as "ResolutionRule(7)" on the wire.
func TestParseResolutionRule_UnknownIsUnscoped(t *testing.T) {
	t.Parallel()
	assert.Equal(t, store.ResolutionUnscoped, store.ParseResolutionRule("no-such-rule"))
	assert.Equal(t, store.ResolutionUnscoped, store.ParseResolutionRule(""))
}

// An out-of-range value must still name itself. These enums have a zero that
// means something specific, so a bare integer in a log line is exactly the
// confusion the String methods exist to prevent.
func TestRuleStringsNameOutOfRangeValues(t *testing.T) {
	t.Parallel()
	assert.Equal(t, "ResolutionRule(9)", store.ResolutionRule(9).String())
	assert.Equal(t, "Fallback(9)", store.Fallback(9).String())
	assert.Equal(t, "exclude", store.FallbackExclude.String())
	assert.Equal(t, "default-state", store.FallbackDefaultState.String())
}

// policyScope builds a world that resolves `policy` and says nothing about any
// other type, so a second type in the same test exercises rule 1.
func policyScope(fallback store.Fallback, chain ...entity.Face) store.WorldScope {
	return store.NewWorldScope(map[string]store.TypeResolution{
		"policy": {Chain: chain, Fallback: fallback},
	})
}

// The three resolution rules, each with the provenance that distinguishes it.
// "The published face" and "the default face, because no published face
// exists" are different facts about the same bytes; only Via and ChainPosition
// tell them apart, so every case asserts provenance, not just the face.
func TestResolveWorldPrimes(t *testing.T) {
	t.Parallel()
	const published, draft = entity.Face("published"), entity.Face("draft")

	t.Run("rule 2: the first chain coordinate that exists wins", func(t *testing.T) {
		t.Parallel()
		got := store.ResolveWorldPrimes(
			policyScope(store.FallbackExclude, published, draft),
			[]store.WorldCandidate{
				{ID: "POL-1", Type: "policy", Face: draft},
				{ID: "POL-1", Type: "policy", Face: published},
			})
		require.Contains(t, got, "POL-1")
		assert.Equal(t, published, got["POL-1"].Face)
		assert.Equal(t, store.ResolutionChain, got["POL-1"].Via)
		assert.Equal(t, 0, got["POL-1"].ChainPosition, "the world's first choice is position 0")
	})

	t.Run("a later chain coordinate reports its position", func(t *testing.T) {
		t.Parallel()
		// Only the draft exists, so the chain falls through to index 1. The
		// position is what lets a UI badge the row as a stand-in.
		got := store.ResolveWorldPrimes(
			policyScope(store.FallbackExclude, published, draft),
			[]store.WorldCandidate{{ID: "POL-2", Type: "policy", Face: draft}})
		require.Contains(t, got, "POL-2")
		assert.Equal(t, draft, got["POL-2"].Face)
		assert.Equal(t, store.ResolutionChain, got["POL-2"].Via)
		assert.Equal(t, 1, got["POL-2"].ChainPosition)
	})

	t.Run("rule 3 under exclude: the entity is absent entirely", func(t *testing.T) {
		t.Parallel()
		// The publication bit itself: an unpublished policy is not merely
		// stale in this world, it is not there.
		got := store.ResolveWorldPrimes(
			policyScope(store.FallbackExclude, published),
			[]store.WorldCandidate{{ID: "POL-3", Type: "policy", Face: entity.Face("")}})
		assert.NotContains(t, got, "POL-3")
	})

	t.Run("rule 3 under default-state: the default face stands in", func(t *testing.T) {
		t.Parallel()
		got := store.ResolveWorldPrimes(
			policyScope(store.FallbackDefaultState, published),
			[]store.WorldCandidate{{ID: "POL-4", Type: "policy", Face: entity.Face("")}})
		require.Contains(t, got, "POL-4")
		assert.True(t, got["POL-4"].Face.IsDefault())
		assert.Equal(t, store.ResolutionFallbackDefault, got["POL-4"].Via)
	})

	t.Run("rule 1: an unscoped type keeps its default face", func(t *testing.T) {
		t.Parallel()
		// A type the world says nothing about must behave exactly as it did
		// before worlds existed — absence is not the zero TypeResolution.
		got := store.ResolveWorldPrimes(
			policyScope(store.FallbackExclude, published),
			[]store.WorldCandidate{{ID: "CTL-1", Type: "control", Face: entity.Face("")}})
		require.Contains(t, got, "CTL-1")
		assert.True(t, got["CTL-1"].Face.IsDefault())
		assert.Equal(t, store.ResolutionUnscoped, got["CTL-1"].Via)
	})

	t.Run("no candidates resolves nothing", func(t *testing.T) {
		t.Parallel()
		assert.Nil(t, store.ResolveWorldPrimes(
			policyScope(store.FallbackExclude, published), nil))
	})

	t.Run("the verdict does not depend on candidate order", func(t *testing.T) {
		t.Parallel()
		// Decisions are made after the whole candidate set is seen. A
		// per-row streaming implementation would resolve to whichever face
		// arrived first, which is the bug this ordering rule prevents.
		scope := policyScope(store.FallbackExclude, published, draft)
		forward := store.ResolveWorldPrimes(scope, []store.WorldCandidate{
			{ID: "POL-5", Type: "policy", Face: draft},
			{ID: "POL-5", Type: "policy", Face: published},
		})
		reverse := store.ResolveWorldPrimes(scope, []store.WorldCandidate{
			{ID: "POL-5", Type: "policy", Face: published},
			{ID: "POL-5", Type: "policy", Face: draft},
		})
		assert.Equal(t, forward, reverse)
	})
}

// ResolutionAt answers for a face already chosen, where the family is not in
// hand. It is a total mapping rather than a walk, so every input must produce
// a rule.
func TestResolutionAt(t *testing.T) {
	t.Parallel()
	const published, draft, nl = entity.Face("published"), entity.Face("draft"), entity.Face("nl")
	scope := policyScope(store.FallbackDefaultState, published, draft)

	t.Run("a chain member reports its position", func(t *testing.T) {
		t.Parallel()
		rule, pos := store.ResolutionAt(scope, "policy", published)
		assert.Equal(t, store.ResolutionChain, rule)
		assert.Equal(t, 0, pos)

		rule, pos = store.ResolutionAt(scope, "policy", draft)
		assert.Equal(t, store.ResolutionChain, rule)
		assert.Equal(t, 1, pos)
	})

	t.Run("a face outside the chain is the fallback", func(t *testing.T) {
		t.Parallel()
		rule, pos := store.ResolutionAt(scope, "policy", nl)
		assert.Equal(t, store.ResolutionFallbackDefault, rule)
		assert.Equal(t, 0, pos)
	})

	t.Run("an unscoped type is rule 1", func(t *testing.T) {
		t.Parallel()
		// The doc requires a CANONICAL type name: an alias reads as unknown,
		// which is rule 1 rather than an error, so a caller passing an alias
		// silently gets the pre-worlds answer.
		rule, pos := store.ResolutionAt(scope, "control", published)
		assert.Equal(t, store.ResolutionUnscoped, rule)
		assert.Equal(t, 0, pos)
	})
}
