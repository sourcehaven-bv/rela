package nextaction_test

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/dataentryconfig"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/nextaction"
	"github.com/Sourcehaven-BV/rela/internal/userstate/memuserstate"
)

// matchIDs matches exactly the listed entity ids, standing in for a compiled
// predicate without dragging the metamodel into the engine's tests — which is
// the point of the Matcher seam.
type matchIDs struct {
	ids  map[string]bool
	err  error
	seen int
}

func (m *matchIDs) Match(_ context.Context, e *entity.Entity) (bool, error) {
	m.seen++
	if m.err != nil {
		return false, m.err
	}
	return m.ids[e.ID], nil
}

func conditionConfig() *dataentryconfig.Config {
	return &dataentryconfig.Config{
		NextActionBands: []dataentryconfig.NextActionBand{{ID: "stalled"}},
		NextActions: map[string]dataentryconfig.NextActionSource{
			"stale": {
				Band:      "stalled",
				Query:     "type:task",
				Condition: "days_between(entity.due, today()) <= 7",
				Suggest:   "Still on it?",
			},
		},
	}
}

func engineWithMatcher(t *testing.T, cfg *dataentryconfig.Config, fn nextaction.CandidateFunc,
	m nextaction.Matcher,
) *nextaction.Engine {
	t.Helper()
	st := memuserstate.New()
	t.Cleanup(func() { _ = st.Close() })
	eng, err := nextaction.New(cfg, st, fn,
		nextaction.WithMatchers(func(string) (nextaction.Matcher, bool) { return m, true }))
	require.NoError(t, err)
	return eng
}

// THE regression this feature turns on. The cap truncates to 20; a condition
// matching only a later candidate must still fire. Filtering after the cap
// would make the source look correctly quiet rather than broken — the
// silent-no-op this whole line of work exists to remove.
func TestCondition_MatchesBeyondTheCandidateCap(t *testing.T) {
	t.Parallel()

	var many []*entity.Entity
	for i := range 60 {
		many = append(many, ent(idFor(i), "task", nil))
	}
	fn, _ := staticCandidates(map[string][]*entity.Entity{"Still on it?": many})

	// Only the 50th candidate matches — well past DefaultCandidateCap (20).
	target := idFor(49)
	m := &matchIDs{ids: map[string]bool{target: true}}

	sug, ok, err := engineWithMatcher(t, conditionConfig(), fn, m).
		Resolve(context.Background(), testUser, base)

	require.NoError(t, err)
	require.True(t, ok, "a condition matching a beyond-cap candidate must still fire")
	require.Equal(t, target, sug.Key.EntityID)
	require.Equal(t, 60, m.seen, "every candidate must be tested before truncation")
}

// A condition that excludes everything yields no suggestion rather than an
// unconditioned one.
func TestCondition_NoMatchYieldsNothing(t *testing.T) {
	t.Parallel()
	fn, _ := staticCandidates(map[string][]*entity.Entity{
		"Still on it?": {ent("T-1", "task", nil), ent("T-2", "task", nil)},
	})
	m := &matchIDs{ids: map[string]bool{}}

	_, ok, err := engineWithMatcher(t, conditionConfig(), fn, m).
		Resolve(context.Background(), testUser, base)

	require.NoError(t, err)
	require.False(t, ok)
}

// An eval error is surfaced, not swallowed as "does not match". A broken
// condition must be distinguishable from a source with nothing to say.
func TestCondition_EvalErrorSurfaces(t *testing.T) {
	t.Parallel()
	fn, _ := staticCandidates(map[string][]*entity.Entity{
		"Still on it?": {ent("T-1", "task", nil)},
	})
	m := &matchIDs{err: errors.New("missing property")}

	_, _, err := engineWithMatcher(t, conditionConfig(), fn, m).
		Resolve(context.Background(), testUser, base)

	require.Error(t, err)
	require.Contains(t, err.Error(), "missing property")
}

// Declaring a condition without wiring matchers must fail construction. It
// would otherwise evaluate as "keep everything" — the suggestion still shows,
// but for entities the operator excluded, which looks like it works.
func TestNew_ConditionWithoutMatchersIsRejected(t *testing.T) {
	t.Parallel()
	fn, _ := staticCandidates(nil)
	st := memuserstate.New()
	t.Cleanup(func() { _ = st.Close() })

	_, err := nextaction.New(conditionConfig(), st, fn)

	require.Error(t, err)
	require.Contains(t, err.Error(), "WithMatchers")
}

// A source with no condition is unaffected when matchers are wired.
func TestCondition_AbsentConditionKeepsEveryCandidate(t *testing.T) {
	t.Parallel()
	cfg := conditionConfig()
	src := cfg.NextActions["stale"]
	src.Condition = ""
	cfg.NextActions["stale"] = src

	fn, _ := staticCandidates(map[string][]*entity.Entity{
		"Still on it?": {ent("T-1", "task", nil)},
	})
	m := &matchIDs{ids: map[string]bool{}} // would reject everything if consulted

	_, ok, err := engineWithMatcher(t, cfg, fn, m).Resolve(context.Background(), testUser, base)

	require.NoError(t, err)
	require.True(t, ok)
	require.Zero(t, m.seen, "a source without a condition must not consult a matcher")
}

// A wired lookup with no entry for this source must FAIL, not fall through to
// "keep everything" — that would show the suggestion for entities the operator
// excluded, which looks like the feature working. New() cannot catch this: the
// lookup is opaque to it.
func TestCondition_MissingMatcherFailsClosed(t *testing.T) {
	t.Parallel()
	fn, _ := staticCandidates(map[string][]*entity.Entity{
		"Still on it?": {ent("T-1", "task", nil)},
	})
	st := memuserstate.New()
	t.Cleanup(func() { _ = st.Close() })

	eng, err := nextaction.New(conditionConfig(), st, fn,
		nextaction.WithMatchers(func(string) (nextaction.Matcher, bool) { return nil, false }))
	require.NoError(t, err)

	_, _, err = eng.Resolve(context.Background(), testUser, base)
	require.Error(t, err)
	require.Contains(t, err.Error(), "no matcher was compiled")
}

func idFor(i int) string {
	return "T-" + string(rune('a'+i%26)) + string(rune('a'+i/26))
}
