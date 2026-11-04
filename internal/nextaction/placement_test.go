package nextaction_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/dataentryconfig"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/nextaction"
	"github.com/Sourcehaven-BV/rela/internal/userstate"
	"github.com/Sourcehaven-BV/rela/internal/userstate/memuserstate"
)

func newEngineWith(
	t *testing.T, cfg *dataentryconfig.Config, fn nextaction.CandidateFunc, opts ...nextaction.Option,
) *nextaction.Engine {
	t.Helper()
	st := memuserstate.New()
	t.Cleanup(func() { _ = st.Close() })
	eng, err := nextaction.New(cfg, st, fn, opts...)
	require.NoError(t, err)
	return eng
}

// The suggestion on screen must survive a navigation. Each page re-resolves,
// and the impression of the first page starts a cooldown; without the
// exemption the card would change on every click and use up the queue.
func TestResolve_CurrentIsExemptFromCooldown(t *testing.T) {
	t.Parallel()
	fn, _ := staticCandidates(map[string][]*entity.Entity{
		"urgent": {ent("T-1", "task", nil), ent("T-2", "task", nil)},
		"quip":   {ent("Q-1", "quip", nil)},
	})
	cfg := twoBandConfig()
	ctx := context.Background()

	first, st := newEngine(t, cfg, fn)
	got, ok, err := first.Resolve(ctx, testUser, base)
	require.NoError(t, err)
	require.True(t, ok)
	require.NoError(t, first.MarkShown(ctx, got, base))

	sticky, err := nextaction.New(cfg, st, fn, nextaction.WithCurrent(&userstate.Key{
		Source: got.Source, EntityID: got.EntityID, Variant: got.Key.Variant,
	}))
	require.NoError(t, err)
	again, ok, err := sticky.Resolve(ctx, testUser, base.Add(time.Hour))
	require.NoError(t, err)
	require.True(t, ok)
	require.Equal(t, got.Key, again.Key, "the current suggestion stays on screen inside its cooldown")

	plain, err := nextaction.New(cfg, st, fn)
	require.NoError(t, err)
	other, ok, err := plain.Resolve(ctx, testUser, base.Add(time.Hour))
	require.NoError(t, err)
	require.True(t, ok)
	require.NotEqual(t, got.Key, other.Key, "without current the cooldown still applies")
}

// The current suggestion wins its band over the stable-random pick, but a
// higher band still pre-empts it.
func TestResolve_CurrentWinsItsBandOnly(t *testing.T) {
	t.Parallel()
	fn, _ := staticCandidates(map[string][]*entity.Entity{
		"urgent": {ent("T-1", "task", nil), ent("T-2", "task", nil), ent("T-3", "task", nil)},
		"quip":   {ent("Q-1", "quip", nil)},
	})
	ctx := context.Background()
	for _, id := range []string{"T-1", "T-2", "T-3"} {
		eng := newEngineWith(t, twoBandConfig(), fn,
			nextaction.WithCurrent(&userstate.Key{Source: "urgent", EntityID: id}))
		got, ok, err := eng.Resolve(ctx, testUser, base)
		require.NoError(t, err)
		require.True(t, ok)
		require.Equal(t, id, got.EntityID)
	}

	eng := newEngineWith(t, twoBandConfig(), fn,
		nextaction.WithCurrent(&userstate.Key{Source: "quip", EntityID: "Q-1"}))
	got, _, err := eng.Resolve(ctx, testUser, base)
	require.NoError(t, err)
	require.Equal(t, "urgent", got.Source, "a higher band pre-empts the current suggestion")
}

// A filtered-out source is never queried, so a page that places nothing pays
// for nothing.
func TestResolve_SourceFilterSkipsBeforeQuerying(t *testing.T) {
	t.Parallel()
	fn, queried := staticCandidates(map[string][]*entity.Entity{
		"urgent": {ent("T-1", "task", nil)},
		"quip":   {ent("Q-1", "quip", nil)},
	})
	eng := newEngineWith(t, twoBandConfig(), fn,
		nextaction.WithSourceFilter(func(id string, _ dataentryconfig.NextActionSource) bool { return id != "urgent" }))
	got, ok, err := eng.Resolve(context.Background(), testUser, base)
	require.NoError(t, err)
	require.True(t, ok)
	require.Equal(t, "quip", got.Source)
	require.Equal(t, []string{"quip"}, *queried)
}

// A context source is about the page the reader is on. Having shown it on a
// visit yesterday is no reason to hide it on this visit, so the impression
// cooldown does not apply. Snooze still does.
func TestResolve_ContextSourceIgnoresImpressionCooldown(t *testing.T) {
	t.Parallel()
	cfg := &dataentryconfig.Config{
		NextActionBands: []dataentryconfig.NextActionBand{{ID: "b"}},
		NextActions: map[string]dataentryconfig.NextActionSource{
			"here": {Band: "b", Context: "task", Suggest: "here"},
		},
	}
	fn, _ := staticCandidates(map[string][]*entity.Entity{"here": {ent("T-1", "task", nil)}})
	eng, st := newEngine(t, cfg, fn)
	ctx := context.Background()

	got, ok, err := eng.Resolve(ctx, testUser, base)
	require.NoError(t, err)
	require.True(t, ok)
	require.NoError(t, eng.MarkShown(ctx, got, base))

	_, ok, err = eng.Resolve(ctx, testUser, base.Add(time.Hour))
	require.NoError(t, err)
	require.True(t, ok, "a shown context suggestion returns on the next visit")

	require.NoError(t, st.SetSnooze(ctx, got.Key, base.Add(24*time.Hour)))
	_, ok, err = eng.Resolve(ctx, testUser, base.Add(2*time.Hour))
	require.NoError(t, err)
	require.False(t, ok, "snooze still suppresses a context suggestion")
}

// The cooldown exemption is the ONLY thing current changes. A suggestion the
// reader snoozed or dismissed stays hidden even while it is on screen in
// another tab, and a muted source stays muted.
func TestResolve_CurrentDoesNotLiftSuppression(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	fn, _ := staticCandidates(map[string][]*entity.Entity{"urgent": {ent("T-1", "task", nil)}})
	cfg := &dataentryconfig.Config{
		NextActionBands: []dataentryconfig.NextActionBand{{ID: "blocking"}},
		NextActions: map[string]dataentryconfig.NextActionSource{
			"urgent": {Band: "blocking", Query: "type:task", Suggest: "urgent"},
		},
	}
	current := nextaction.WithCurrent(&userstate.Key{Source: "urgent", EntityID: "T-1"})

	t.Run("snoozed", func(t *testing.T) {
		t.Parallel()
		st := memuserstate.New()
		t.Cleanup(func() { _ = st.Close() })
		key := userstateKey("urgent", "T-1")
		require.NoError(t, st.SetSnooze(ctx, key, base.Add(24*time.Hour)))
		eng, err := nextaction.New(cfg, st, fn, current)
		require.NoError(t, err)
		_, ok, err := eng.Resolve(ctx, testUser, base)
		require.NoError(t, err)
		require.False(t, ok)
	})
	t.Run("muted", func(t *testing.T) {
		t.Parallel()
		st := memuserstate.New()
		t.Cleanup(func() { _ = st.Close() })
		require.NoError(t, st.SetMuted(ctx, testUser, "urgent", true))
		eng, err := nextaction.New(cfg, st, fn, current)
		require.NoError(t, err)
		_, ok, err := eng.Resolve(ctx, testUser, base)
		require.NoError(t, err)
		require.False(t, ok)
	})
}

// The candidate cap must not cut the suggestion on screen: a store order
// that moves would otherwise swap it for another one it still owes.
func TestResolve_CapKeepsCurrent(t *testing.T) {
	t.Parallel()
	var many []*entity.Entity
	for i := range nextaction.DefaultCandidateCap + 5 {
		many = append(many, ent(fmt.Sprintf("T-%03d", i), "task", nil))
	}
	last := many[len(many)-1].ID
	fn, _ := staticCandidates(map[string][]*entity.Entity{"urgent": many})
	cfg := &dataentryconfig.Config{
		NextActionBands: []dataentryconfig.NextActionBand{{ID: "blocking"}},
		NextActions: map[string]dataentryconfig.NextActionSource{
			"urgent": {Band: "blocking", Query: "type:task", Suggest: "urgent"},
		},
	}
	eng := newEngineWith(t, cfg, fn, nextaction.WithCurrent(&userstate.Key{Source: "urgent", EntityID: last}))
	got, ok, err := eng.Resolve(context.Background(), testUser, base)
	require.NoError(t, err)
	require.True(t, ok)
	require.Equal(t, last, got.EntityID)
}

func userstateKey(source, id string) userstate.Key {
	return userstate.Key{User: testUser, Source: source, EntityID: id}
}

// Cooldown falls back to the default for a source that sets none.
func TestCooldown(t *testing.T) {
	t.Parallel()
	require.Equal(t, nextaction.DefaultCooldown, nextaction.Cooldown(dataentryconfig.NextActionSource{}))
	require.Equal(t, 72*time.Hour, nextaction.Cooldown(dataentryconfig.NextActionSource{Cooldown: "3d"}))
}
