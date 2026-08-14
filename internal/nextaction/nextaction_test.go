package nextaction_test

import (
	"context"
	"errors"
	"maps"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/dataentryconfig"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/nextaction"
	"github.com/Sourcehaven-BV/rela/internal/userstate"
	"github.com/Sourcehaven-BV/rela/internal/userstate/memuserstate"
)

var base = time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC)

const testUser = "alice"

// ent builds a candidate entity with the given properties.
func ent(id, typ string, props map[string]any) *entity.Entity {
	e := entity.New(id, typ)
	maps.Copy(e.Properties, props)
	return e
}

// staticCandidates returns a CandidateFunc serving a fixed set per source id,
// plus a counter recording which sources were actually queried — that counter
// is how the short-circuit test proves lower bands were never touched.
func staticCandidates(
	bySource map[string][]*entity.Entity,
) (fn nextaction.CandidateFunc, queriedSuggests *[]string) {
	var queried []string
	fn = func(_ context.Context, src dataentryconfig.NextActionSource) ([]nextaction.Candidate, error) {
		// Identify the source by its Suggest, which the fixtures make unique.
		queried = append(queried, src.Suggest)
		var out []nextaction.Candidate
		for _, e := range bySource[src.Suggest] {
			out = append(out, nextaction.Candidate{Entity: e})
		}
		return out, nil
	}
	return fn, &queried
}

func newEngine(t *testing.T, cfg *dataentryconfig.Config, fn nextaction.CandidateFunc) (*nextaction.Engine, userstate.Store) {
	t.Helper()
	st := memuserstate.New()
	t.Cleanup(func() { _ = st.Close() })
	eng, err := nextaction.New(cfg, st, fn)
	require.NoError(t, err)
	return eng, st
}

func twoBandConfig() *dataentryconfig.Config {
	return &dataentryconfig.Config{
		NextActionBands: []dataentryconfig.NextActionBand{
			{ID: "blocking"},
			{ID: "ambient"},
		},
		NextActions: map[string]dataentryconfig.NextActionSource{
			"urgent": {Band: "blocking", Query: "type:task", Suggest: "urgent"},
			"quip":   {Band: "ambient", Query: "type:quip", Suggest: "quip"},
		},
	}
}

func TestNew_RejectsNilCollaborators(t *testing.T) {
	t.Parallel()
	cfg := twoBandConfig()
	fn, _ := staticCandidates(nil)
	st := memuserstate.New()
	t.Cleanup(func() { _ = st.Close() })

	_, err := nextaction.New(nil, st, fn)
	require.Error(t, err, "nil config must be rejected")

	_, err = nextaction.New(cfg, nil, fn)
	require.Error(t, err, "a nil userstate store would silently stop honoring snoozes")

	_, err = nextaction.New(cfg, st, nil)
	require.Error(t, err, "nil candidate func must be rejected")
}

// The core ordering guarantee: a higher band wins even when a lower one also
// has something to say.
func TestResolve_HigherBandWins(t *testing.T) {
	t.Parallel()
	fn, _ := staticCandidates(map[string][]*entity.Entity{
		"urgent": {ent("T-1", "task", nil)},
		"quip":   {ent("Q-1", "quip", nil)},
	})
	eng, _ := newEngine(t, twoBandConfig(), fn)

	got, ok, err := eng.Resolve(context.Background(), testUser, base)
	require.NoError(t, err)
	require.True(t, ok)
	require.Equal(t, "urgent", got.Source)
	require.Equal(t, "blocking", got.Band)
}

// Short-circuiting is what keeps a page to one or two queries. Asserting the
// lower band was never QUERIED (not merely that it lost) is the difference
// between the optimisation existing and not.
func TestResolve_ShortCircuitsLowerBands(t *testing.T) {
	t.Parallel()
	fn, queried := staticCandidates(map[string][]*entity.Entity{
		"urgent": {ent("T-1", "task", nil)},
		"quip":   {ent("Q-1", "quip", nil)},
	})
	eng, _ := newEngine(t, twoBandConfig(), fn)

	_, ok, err := eng.Resolve(context.Background(), testUser, base)
	require.NoError(t, err)
	require.True(t, ok)
	require.Equal(t, []string{"urgent"}, *queried,
		"the ambient source must not be queried once a higher band hits")
}

func TestResolve_FallsThroughToLowerBand(t *testing.T) {
	t.Parallel()
	fn, queried := staticCandidates(map[string][]*entity.Entity{
		"quip": {ent("Q-1", "quip", nil)},
	})
	eng, _ := newEngine(t, twoBandConfig(), fn)

	got, ok, err := eng.Resolve(context.Background(), testUser, base)
	require.NoError(t, err)
	require.True(t, ok)
	require.Equal(t, "quip", got.Source)
	require.Equal(t, []string{"urgent", "quip"}, *queried,
		"an empty high band must fall through, having queried both")
}

func TestResolve_NothingOwed(t *testing.T) {
	t.Parallel()
	fn, _ := staticCandidates(nil)
	eng, _ := newEngine(t, twoBandConfig(), fn)

	_, ok, err := eng.Resolve(context.Background(), testUser, base)
	require.NoError(t, err)
	require.False(t, ok, "no candidates anywhere means no suggestion, not an error")
}

func TestResolve_MutedSourceIsSkipped(t *testing.T) {
	t.Parallel()
	fn, _ := staticCandidates(map[string][]*entity.Entity{
		"urgent": {ent("T-1", "task", nil)},
		"quip":   {ent("Q-1", "quip", nil)},
	})
	eng, st := newEngine(t, twoBandConfig(), fn)
	require.NoError(t, st.SetMuted(context.Background(), testUser, "urgent", true))

	got, ok, err := eng.Resolve(context.Background(), testUser, base)
	require.NoError(t, err)
	require.True(t, ok)
	require.Equal(t, "quip", got.Source, "a muted source must not win its band")
}

func TestResolve_SnoozeSuppresses(t *testing.T) {
	t.Parallel()
	fn, _ := staticCandidates(map[string][]*entity.Entity{
		"urgent": {ent("T-1", "task", nil)},
		"quip":   {ent("Q-1", "quip", nil)},
	})
	eng, st := newEngine(t, twoBandConfig(), fn)
	require.NoError(t, st.SetSnooze(context.Background(),
		userstate.Key{User: testUser, Source: "urgent", EntityID: "T-1"}, base.Add(24*time.Hour)))

	got, ok, err := eng.Resolve(context.Background(), testUser, base)
	require.NoError(t, err)
	require.True(t, ok)
	require.Equal(t, "quip", got.Source)

	// Once the snooze lapses the higher band takes over again.
	got, ok, err = eng.Resolve(context.Background(), testUser, base.Add(25*time.Hour))
	require.NoError(t, err)
	require.True(t, ok)
	require.Equal(t, "urgent", got.Source, "an expired snooze must stop suppressing")
}

// Cooldown is what stops a high band re-showing the same hint on every page
// load — without it, short-circuiting would pin the top suggestion forever.
func TestResolve_CooldownSuppressesAfterShown(t *testing.T) {
	t.Parallel()
	cfg := twoBandConfig()
	src := cfg.NextActions["urgent"]
	src.Cooldown = "3d"
	cfg.NextActions["urgent"] = src

	fn, _ := staticCandidates(map[string][]*entity.Entity{
		"urgent": {ent("T-1", "task", nil)},
		"quip":   {ent("Q-1", "quip", nil)},
	})
	eng, _ := newEngine(t, cfg, fn)
	ctx := context.Background()

	got, ok, err := eng.Resolve(ctx, testUser, base)
	require.NoError(t, err)
	require.True(t, ok)
	require.Equal(t, "urgent", got.Source)
	require.NoError(t, eng.MarkShown(ctx, got, base))

	// Within the cooldown the lower band gets its turn.
	got, ok, err = eng.Resolve(ctx, testUser, base.Add(time.Hour))
	require.NoError(t, err)
	require.True(t, ok)
	require.Equal(t, "quip", got.Source, "a suggestion inside its cooldown must not re-show")

	// After it, the high band returns.
	got, ok, err = eng.Resolve(ctx, testUser, base.Add(4*24*time.Hour))
	require.NoError(t, err)
	require.True(t, ok)
	require.Equal(t, "urgent", got.Source)
}

// A source with no cooldown must still get one: DefaultCooldown exists so an
// operator omission cannot produce a nag.
func TestResolve_DefaultCooldownApplies(t *testing.T) {
	t.Parallel()
	fn, _ := staticCandidates(map[string][]*entity.Entity{
		"urgent": {ent("T-1", "task", nil)},
	})
	eng, _ := newEngine(t, twoBandConfig(), fn)
	ctx := context.Background()

	got, _, err := eng.Resolve(ctx, testUser, base)
	require.NoError(t, err)
	require.NoError(t, eng.MarkShown(ctx, got, base))

	_, ok, err := eng.Resolve(ctx, testUser, base.Add(time.Hour))
	require.NoError(t, err)
	require.False(t, ok, "an unconfigured cooldown must still suppress, not default to zero")
}

// Resolve must not start the clock: only MarkShown does.
func TestResolve_DoesNotMarkShown(t *testing.T) {
	t.Parallel()
	fn, _ := staticCandidates(map[string][]*entity.Entity{
		"urgent": {ent("T-1", "task", nil)},
	})
	eng, _ := newEngine(t, twoBandConfig(), fn)
	ctx := context.Background()

	_, _, err := eng.Resolve(ctx, testUser, base)
	require.NoError(t, err)

	got, ok, err := eng.Resolve(ctx, testUser, base.Add(time.Minute))
	require.NoError(t, err)
	require.True(t, ok, "resolving twice without MarkShown must still yield a suggestion")
	require.Equal(t, "urgent", got.Source)
}

func TestInterpolation(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		suggest string
		props   map[string]any
		want    string
	}{
		{
			name:    "substitutes a property",
			suggest: "{title} is waiting",
			props:   map[string]any{"title": "Draft the SOW"},
			want:    "Draft the SOW is waiting",
		},
		{
			name:    "substitutes several",
			suggest: "{title} since {started_on}",
			props:   map[string]any{"title": "SOW", "started_on": "2026-02-01"},
			want:    "SOW since 2026-02-01",
		},
		{
			name:    "id is available",
			suggest: "look at {id}",
			want:    "look at T-1",
		},
		{
			// A config typo must stay visibly a typo. Emptying it would
			// render a sentence with a hole and read as a product bug.
			name:    "unknown placeholder is left verbatim",
			suggest: "{nope} matters",
			want:    "{nope} matters",
		},
		{
			name:    "no placeholders",
			suggest: "just text",
			want:    "just text",
		},
		{
			name:    "unclosed brace is left alone",
			suggest: "{unclosed",
			want:    "{unclosed",
		},
		{
			name:    "non-string values render",
			suggest: "{count} left",
			props:   map[string]any{"count": 3},
			want:    "3 left",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			cfg := twoBandConfig()
			src := cfg.NextActions["urgent"]
			src.Suggest = tc.suggest
			cfg.NextActions["urgent"] = src

			fn := func(_ context.Context, s dataentryconfig.NextActionSource) ([]nextaction.Candidate, error) {
				if s.Suggest != tc.suggest {
					return nil, nil
				}
				return []nextaction.Candidate{{Entity: ent("T-1", "task", tc.props)}}, nil
			}
			eng, _ := newEngine(t, cfg, fn)

			got, ok, err := eng.Resolve(context.Background(), testUser, base)
			require.NoError(t, err)
			require.True(t, ok)
			require.Equal(t, tc.want, got.Message)
		})
	}
}

// Stability is what stops a refresh re-rolling the hint; per-day variation is
// what stops it being frozen forever.
func TestPick_StablePerUserPerDay(t *testing.T) {
	t.Parallel()
	many := []*entity.Entity{
		ent("T-1", "task", nil), ent("T-2", "task", nil), ent("T-3", "task", nil),
		ent("T-4", "task", nil), ent("T-5", "task", nil),
	}
	fn, _ := staticCandidates(map[string][]*entity.Entity{"urgent": many})
	eng, _ := newEngine(t, twoBandConfig(), fn)
	ctx := context.Background()

	first, _, err := eng.Resolve(ctx, testUser, base)
	require.NoError(t, err)
	for range 5 {
		again, _, rerr := eng.Resolve(ctx, testUser, base.Add(time.Minute))
		require.NoError(t, rerr)
		require.Equal(t, first.EntityID, again.EntityID,
			"a refresh on the same day must not re-roll the suggestion")
	}

	// Different users may differ; over five candidates at least one of a
	// handful of users should land elsewhere.
	differs := false
	for _, u := range []string{"bob", "carol", "dave", "erin"} {
		other, _, oerr := eng.Resolve(ctx, u, base)
		require.NoError(t, oerr)
		if other.EntityID != first.EntityID {
			differs = true
			break
		}
	}
	require.True(t, differs, "the pick should vary across users, not be a constant")
}

func TestResolve_CandidateCapIsApplied(t *testing.T) {
	t.Parallel()
	var many []*entity.Entity
	for i := range 100 {
		many = append(many, ent("T-"+string(rune('a'+i%26))+string(rune('a'+i/26)), "task", nil))
	}
	fn, _ := staticCandidates(map[string][]*entity.Entity{"urgent": many})
	eng, _ := newEngine(t, twoBandConfig(), fn)

	// The cap must not turn "many candidates" into "no suggestion".
	_, ok, err := eng.Resolve(context.Background(), testUser, base)
	require.NoError(t, err)
	require.True(t, ok)
}

// An entity-less (count) source keys on the source alone, so its snooze and
// cooldown still work.
func TestResolve_EntitylessSource(t *testing.T) {
	t.Parallel()
	cfg := &dataentryconfig.Config{
		NextActionBands: []dataentryconfig.NextActionBand{{ID: "blocking"}},
		NextActions: map[string]dataentryconfig.NextActionSource{
			"first-run": {Band: "blocking", Count: "client == 0", Suggest: "Start with a client?"},
		},
	}
	fn := func(_ context.Context, _ dataentryconfig.NextActionSource) ([]nextaction.Candidate, error) {
		return []nextaction.Candidate{{Entity: nil}}, nil
	}
	eng, st := newEngine(t, cfg, fn)
	ctx := context.Background()

	got, ok, err := eng.Resolve(ctx, testUser, base)
	require.NoError(t, err)
	require.True(t, ok)
	require.Empty(t, got.EntityID, "a count source has no entity")
	require.Equal(t, "Start with a client?", got.Message)

	require.NoError(t, st.SetSnooze(ctx, got.Key, base.Add(24*time.Hour)))
	_, ok, err = eng.Resolve(ctx, testUser, base)
	require.NoError(t, err)
	require.False(t, ok, "an entity-less suggestion must be snoozable")
}

// pick_one is the one affordance whose options cannot be written in config:
// they come from a query at render time.
func TestPickOne_ResolvesOptionsForTheWinner(t *testing.T) {
	t.Parallel()
	cfg := twoBandConfig()
	src := cfg.NextActions["urgent"]
	src.Actions = []dataentryconfig.NextActionOffer{
		{PickOne: &dataentryconfig.NextActionPickOne{
			Query: "type:task prop:effort=xs", Action: "start-task",
		}},
	}
	cfg.NextActions["urgent"] = src

	fn, _ := staticCandidates(map[string][]*entity.Entity{
		"urgent": {ent("T-1", "task", nil)},
	})
	eng, _ := newEngine(t, cfg, fn)
	eng.WithOptions(func(_ context.Context, _ string, limit int) ([]nextaction.PickOption, error) {
		require.Equal(t, dataentryconfig.DefaultPickOneLimit, limit,
			"an unset limit must arrive as the default, not zero")
		return []nextaction.PickOption{
			{EntityID: "T-9", Label: "Small one"},
			{EntityID: "T-8", Label: "Another"},
		}, nil
	})

	got, ok, err := eng.Resolve(context.Background(), testUser, base)
	require.NoError(t, err)
	require.True(t, ok)
	require.Len(t, got.PickOptions[0], 2)
	require.Equal(t, "Small one", got.PickOptions[0][0].Label)
}

// Options are resolved for the WINNER only: doing it during candidate
// collection would run an extra query per candidate for a list only one of
// them ever displays.
func TestPickOne_DoesNotResolveForLosingBands(t *testing.T) {
	t.Parallel()
	cfg := twoBandConfig()
	quip := cfg.NextActions["quip"]
	quip.Actions = []dataentryconfig.NextActionOffer{
		{PickOne: &dataentryconfig.NextActionPickOne{Query: "type:quip", Action: "ack"}},
	}
	cfg.NextActions["quip"] = quip

	fn, _ := staticCandidates(map[string][]*entity.Entity{
		"urgent": {ent("T-1", "task", nil)},
		"quip":   {ent("Q-1", "quip", nil)},
	})
	eng, _ := newEngine(t, cfg, fn)

	calls := 0
	eng.WithOptions(func(_ context.Context, _ string, _ int) ([]nextaction.PickOption, error) {
		calls++
		return nil, nil
	})

	got, _, err := eng.Resolve(context.Background(), testUser, base)
	require.NoError(t, err)
	require.Equal(t, "urgent", got.Source)
	require.Zero(t, calls, "the losing band's pick_one query must never run")
}

// An advisory surface must never break the page: a failed option query costs
// one affordance, not the suggestion.
func TestPickOne_FailureDoesNotBreakTheSuggestion(t *testing.T) {
	t.Parallel()
	cfg := twoBandConfig()
	src := cfg.NextActions["urgent"]
	src.Actions = []dataentryconfig.NextActionOffer{
		{PickOne: &dataentryconfig.NextActionPickOne{Query: "type:task", Action: "start-task"}},
	}
	cfg.NextActions["urgent"] = src

	fn, _ := staticCandidates(map[string][]*entity.Entity{"urgent": {ent("T-1", "task", nil)}})
	eng, _ := newEngine(t, cfg, fn)
	eng.WithOptions(func(_ context.Context, _ string, _ int) ([]nextaction.PickOption, error) {
		return nil, errors.New("query exploded")
	})

	got, ok, err := eng.Resolve(context.Background(), testUser, base)
	require.NoError(t, err, "an option failure must not fail the resolve")
	require.True(t, ok)
	require.Empty(t, got.PickOptions)
	require.Equal(t, "urgent", got.Source)
}

// Not wiring the resolver is a valid deployment state, not an error.
func TestPickOne_NoResolverIsHarmless(t *testing.T) {
	t.Parallel()
	cfg := twoBandConfig()
	src := cfg.NextActions["urgent"]
	src.Actions = []dataentryconfig.NextActionOffer{
		{PickOne: &dataentryconfig.NextActionPickOne{Query: "type:task", Action: "start-task"}},
	}
	cfg.NextActions["urgent"] = src

	fn, _ := staticCandidates(map[string][]*entity.Entity{"urgent": {ent("T-1", "task", nil)}})
	eng, _ := newEngine(t, cfg, fn)

	got, ok, err := eng.Resolve(context.Background(), testUser, base)
	require.NoError(t, err)
	require.True(t, ok)
	require.Empty(t, got.PickOptions)
}
