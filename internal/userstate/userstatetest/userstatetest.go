// Package userstatetest is the conformance harness every
// [userstate.Store] implementation must pass.
//
// It exists for the same reason internal/store/storetest does: three
// backends without a shared contract test are three subtly different
// behaviors, and the one that diverges is the one nobody runs locally. The
// expiry and replace-don't-stack rules in particular are easy to implement
// plausibly-but-differently in Go maps, on disk, and in SQL.
//
// Every assertion here is deterministic — no wall clock, no sleeps — because
// [userstate.Store] takes `now` as a parameter. A suite that slept would be
// slow and flaky and would still not pin the boundary conditions.
package userstatetest

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/userstate"
)

// Factory returns a fresh, empty store for one subtest. Cleanup (including
// Close) is the factory's responsibility, normally via t.Cleanup.
type Factory func(t *testing.T) userstate.Store

// base is an arbitrary fixed instant. Fixed rather than time.Now() so a
// failure reproduces exactly and no test depends on the day it runs.
var base = time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC)

// Named spans, so an assertion reads as the scenario it models rather than
// as arithmetic.
const (
	anHour    = time.Hour
	oneDay    = 24 * time.Hour
	twoDays   = 2 * oneDay
	oneWeek   = 7 * oneDay
	oneMonth  = 30 * oneDay
	oneYear   = 365 * oneDay
	nearMonth = 29 * oneDay
)

func key(user, source, entity string) userstate.Key {
	return userstate.Key{User: user, Source: source, EntityID: entity}
}

// RunAll runs every conformance suite against f.
func RunAll(t *testing.T, f Factory) {
	t.Helper()
	t.Run("Snooze", func(t *testing.T) { RunSnoozeTests(t, f) })
	t.Run("Mute", func(t *testing.T) { RunMuteTests(t, f) })
	t.Run("Shown", func(t *testing.T) { RunShownTests(t, f) })
	t.Run("Prune", func(t *testing.T) { RunPruneTests(t, f) })
	t.Run("Isolation", func(t *testing.T) { RunIsolationTests(t, f) })
	t.Run("Closed", func(t *testing.T) { RunClosedTests(t, f) })
}

// RunSnoozeTests pins snooze semantics, including the boundary condition and
// the replace-don't-stack rule.
func RunSnoozeTests(t *testing.T, f Factory) {
	t.Helper()
	ctx := context.Background()

	t.Run("absent key is not snoozed", func(t *testing.T) {
		s := f(t)
		_, ok, err := s.SnoozedUntil(ctx, key("alice", "stale", "T-1"), base)
		require.NoError(t, err)
		require.False(t, ok)
	})

	t.Run("live snooze reports its deadline", func(t *testing.T) {
		s := f(t)
		k := key("alice", "stale", "T-1")
		until := base.Add(oneDay)
		require.NoError(t, s.SetSnooze(ctx, k, until))

		got, ok, err := s.SnoozedUntil(ctx, k, base)
		require.NoError(t, err)
		require.True(t, ok)
		require.True(t, got.Equal(until), "got %v, want %v", got, until)
	})

	// Expiry is judged at read time against `now`, so a backend that never
	// prunes still answers correctly.
	t.Run("expired snooze reads as absent", func(t *testing.T) {
		s := f(t)
		k := key("alice", "stale", "T-1")
		require.NoError(t, s.SetSnooze(ctx, k, base.Add(anHour)))

		_, ok, err := s.SnoozedUntil(ctx, k, base.Add(2*anHour))
		require.NoError(t, err)
		require.False(t, ok)
	})

	// The boundary: "until T" is over AT T. Left to each backend this is
	// exactly where < and <= diverge silently.
	t.Run("snooze expires at the deadline, not after it", func(t *testing.T) {
		s := f(t)
		k := key("alice", "stale", "T-1")
		until := base.Add(anHour)
		require.NoError(t, s.SetSnooze(ctx, k, until))

		_, ok, err := s.SnoozedUntil(ctx, k, until)
		require.NoError(t, err)
		require.False(t, ok, "a snooze until T must not be live at exactly T")

		_, ok, err = s.SnoozedUntil(ctx, k, until.Add(-time.Nanosecond))
		require.NoError(t, err)
		require.True(t, ok, "a snooze until T must still be live just before T")
	})

	t.Run("re-snoozing replaces rather than stacks", func(t *testing.T) {
		s := f(t)
		k := key("alice", "stale", "T-1")
		require.NoError(t, s.SetSnooze(ctx, k, base.Add(anHour)))
		require.NoError(t, s.SetSnooze(ctx, k, base.Add(oneWeek)))

		got, ok, err := s.SnoozedUntil(ctx, k, base)
		require.NoError(t, err)
		require.True(t, ok)
		require.True(t, got.Equal(base.Add(oneWeek)), "later snooze must win, got %v", got)
	})

	// A shorter re-snooze must also win: the user said "actually, remind me
	// sooner", and silently keeping the longer one would ignore them.
	t.Run("a shorter re-snooze also replaces", func(t *testing.T) {
		s := f(t)
		k := key("alice", "stale", "T-1")
		require.NoError(t, s.SetSnooze(ctx, k, base.Add(oneWeek)))
		require.NoError(t, s.SetSnooze(ctx, k, base.Add(anHour)))

		got, _, err := s.SnoozedUntil(ctx, k, base)
		require.NoError(t, err)
		require.True(t, got.Equal(base.Add(anHour)), "shorter snooze must win, got %v", got)
	})

	// Variant is what makes a reset condition surface again.
	t.Run("variant distinguishes keys", func(t *testing.T) {
		s := f(t)
		draft := userstate.Key{User: "alice", Source: "stale", EntityID: "P-1", Variant: "draft"}
		sent := userstate.Key{User: "alice", Source: "stale", EntityID: "P-1", Variant: "sent"}
		require.NoError(t, s.SetSnooze(ctx, draft, base.Add(oneDay)))

		_, ok, err := s.SnoozedUntil(ctx, sent, base)
		require.NoError(t, err)
		require.False(t, ok, "a different variant must not inherit the snooze")
	})

	// Entity-less (count) sources key on the source alone.
	t.Run("empty entity id is a usable key", func(t *testing.T) {
		s := f(t)
		k := key("alice", "first-run", "")
		require.NoError(t, s.SetSnooze(ctx, k, base.Add(oneDay)))

		_, ok, err := s.SnoozedUntil(ctx, k, base)
		require.NoError(t, err)
		require.True(t, ok)
	})
}

// RunMuteTests pins per-source muting and its reversibility.
func RunMuteTests(t *testing.T, f Factory) {
	t.Helper()
	ctx := context.Background()

	t.Run("unmuted by default", func(t *testing.T) {
		s := f(t)
		muted, err := s.Muted(ctx, "alice", "stale")
		require.NoError(t, err)
		require.False(t, muted)
	})

	t.Run("mute then unmute", func(t *testing.T) {
		s := f(t)
		require.NoError(t, s.SetMuted(ctx, "alice", "stale", true))
		muted, err := s.Muted(ctx, "alice", "stale")
		require.NoError(t, err)
		require.True(t, muted)

		require.NoError(t, s.SetMuted(ctx, "alice", "stale", false))
		muted, err = s.Muted(ctx, "alice", "stale")
		require.NoError(t, err)
		require.False(t, muted, "unmute must be reversible — that is the point of per-source mute")
	})

	t.Run("muting is idempotent", func(t *testing.T) {
		s := f(t)
		require.NoError(t, s.SetMuted(ctx, "alice", "stale", true))
		require.NoError(t, s.SetMuted(ctx, "alice", "stale", true))
		got, err := s.MutedSources(ctx, "alice")
		require.NoError(t, err)
		require.Equal(t, []string{"stale"}, got)
	})

	t.Run("unmuting an unmuted source is a no-op", func(t *testing.T) {
		s := f(t)
		require.NoError(t, s.SetMuted(ctx, "alice", "never-muted", false))
		got, err := s.MutedSources(ctx, "alice")
		require.NoError(t, err)
		require.Empty(t, got)
	})

	// The settings screen this backs is what makes muting discoverable.
	t.Run("MutedSources lists sorted ids", func(t *testing.T) {
		s := f(t)
		for _, src := range []string{"zzz", "aaa", "mmm"} {
			require.NoError(t, s.SetMuted(ctx, "alice", src, true))
		}
		got, err := s.MutedSources(ctx, "alice")
		require.NoError(t, err)
		require.Equal(t, []string{"aaa", "mmm", "zzz"}, got)
	})

	t.Run("MutedSources is empty for an unknown user", func(t *testing.T) {
		s := f(t)
		got, err := s.MutedSources(ctx, "nobody")
		require.NoError(t, err)
		require.Empty(t, got)
	})
}

// RunShownTests pins the last-shown record backing cooldown.
func RunShownTests(t *testing.T, f Factory) {
	t.Helper()
	ctx := context.Background()

	t.Run("never shown", func(t *testing.T) {
		s := f(t)
		_, ok, err := s.LastShown(ctx, key("alice", "stale", "T-1"))
		require.NoError(t, err)
		require.False(t, ok)
	})

	t.Run("records and overwrites", func(t *testing.T) {
		s := f(t)
		k := key("alice", "stale", "T-1")
		require.NoError(t, s.MarkShown(ctx, k, base))
		at, ok, err := s.LastShown(ctx, k)
		require.NoError(t, err)
		require.True(t, ok)
		require.True(t, at.Equal(base))

		later := base.Add(anHour)
		require.NoError(t, s.MarkShown(ctx, k, later))
		at, _, err = s.LastShown(ctx, k)
		require.NoError(t, err)
		require.True(t, at.Equal(later), "MarkShown records the LAST time shown")
	})
}

// RunPruneTests pins housekeeping. Pruning must never change an answer —
// only reclaim space — so correctness cannot depend on it having run.
func RunPruneTests(t *testing.T, f Factory) {
	t.Helper()
	ctx := context.Background()

	t.Run("removes expired snoozes and keeps live ones", func(t *testing.T) {
		s := f(t)
		dead := key("alice", "stale", "T-dead")
		live := key("alice", "stale", "T-live")
		require.NoError(t, s.SetSnooze(ctx, dead, base.Add(anHour)))
		require.NoError(t, s.SetSnooze(ctx, live, base.Add(twoDays)))

		removed, err := s.Prune(ctx, base.Add(oneDay), oneMonth)
		require.NoError(t, err)
		require.GreaterOrEqual(t, removed, 1)

		_, ok, err := s.SnoozedUntil(ctx, live, base.Add(oneDay))
		require.NoError(t, err)
		require.True(t, ok, "pruning must not drop a live snooze")
	})

	t.Run("removes stale shown records", func(t *testing.T) {
		s := f(t)
		old := key("alice", "stale", "T-old")
		recent := key("alice", "stale", "T-recent")
		require.NoError(t, s.MarkShown(ctx, old, base))
		require.NoError(t, s.MarkShown(ctx, recent, base.Add(nearMonth)))

		_, err := s.Prune(ctx, base.Add(oneMonth), oneWeek)
		require.NoError(t, err)

		_, ok, err := s.LastShown(ctx, old)
		require.NoError(t, err)
		require.False(t, ok, "shown record older than keepShown should be pruned")

		_, ok, err = s.LastShown(ctx, recent)
		require.NoError(t, err)
		require.True(t, ok, "recent shown record must survive")
	})

	t.Run("pruning an empty store is fine", func(t *testing.T) {
		s := f(t)
		removed, err := s.Prune(ctx, base, time.Hour)
		require.NoError(t, err)
		require.Zero(t, removed)
	})

	// Mutes are an explicit standing choice with no expiry; a housekeeping
	// pass that silently un-muted sources would be a bug the user notices as
	// suggestions they switched off coming back.
	t.Run("pruning never drops mutes", func(t *testing.T) {
		s := f(t)
		require.NoError(t, s.SetMuted(ctx, "alice", "stale", true))
		_, err := s.Prune(ctx, base.Add(oneYear), time.Nanosecond)
		require.NoError(t, err)

		muted, err := s.Muted(ctx, "alice", "stale")
		require.NoError(t, err)
		require.True(t, muted, "a mute has no expiry and must survive pruning")
	})
}

// RunIsolationTests pins that state is scoped per user. A leak here shows a
// user someone else's snoozes — mild in itself, but it would also mean the
// keying is wrong in a way that corrupts cooldown for everyone.
func RunIsolationTests(t *testing.T, f Factory) {
	t.Helper()
	ctx := context.Background()

	t.Run("snoozes do not leak between users", func(t *testing.T) {
		s := f(t)
		require.NoError(t, s.SetSnooze(ctx, key("alice", "stale", "T-1"), base.Add(oneDay)))

		_, ok, err := s.SnoozedUntil(ctx, key("bob", "stale", "T-1"), base)
		require.NoError(t, err)
		require.False(t, ok)
	})

	t.Run("mutes do not leak between users", func(t *testing.T) {
		s := f(t)
		require.NoError(t, s.SetMuted(ctx, "alice", "stale", true))

		muted, err := s.Muted(ctx, "bob", "stale")
		require.NoError(t, err)
		require.False(t, muted)

		got, err := s.MutedSources(ctx, "bob")
		require.NoError(t, err)
		require.Empty(t, got)
	})

	t.Run("shown records do not leak between users", func(t *testing.T) {
		s := f(t)
		require.NoError(t, s.MarkShown(ctx, key("alice", "stale", "T-1"), base))

		_, ok, err := s.LastShown(ctx, key("bob", "stale", "T-1"))
		require.NoError(t, err)
		require.False(t, ok)
	})

	t.Run("sources are distinct within a user", func(t *testing.T) {
		s := f(t)
		require.NoError(t, s.SetSnooze(ctx, key("alice", "src-a", "T-1"), base.Add(oneDay)))

		_, ok, err := s.SnoozedUntil(ctx, key("alice", "src-b", "T-1"), base)
		require.NoError(t, err)
		require.False(t, ok)
	})
}

// RunClosedTests pins that a closed store fails loudly rather than silently
// accepting writes that go nowhere.
func RunClosedTests(t *testing.T, f Factory) {
	t.Helper()
	ctx := context.Background()

	t.Run("methods report ErrClosed after Close", func(t *testing.T) {
		s := f(t)
		require.NoError(t, s.Close())

		_, _, err := s.SnoozedUntil(ctx, key("alice", "stale", "T-1"), base)
		require.ErrorIs(t, err, userstate.ErrClosed)

		err = s.SetSnooze(ctx, key("alice", "stale", "T-1"), base)
		require.ErrorIs(t, err, userstate.ErrClosed)

		_, err = s.Muted(ctx, "alice", "stale")
		require.ErrorIs(t, err, userstate.ErrClosed)

		err = s.MarkShown(ctx, key("alice", "stale", "T-1"), base)
		require.ErrorIs(t, err, userstate.ErrClosed)
	})

	t.Run("Close is idempotent", func(t *testing.T) {
		s := f(t)
		require.NoError(t, s.Close())
		require.NoError(t, s.Close())
	})
}
