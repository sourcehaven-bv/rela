package schedulerstate

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestTaskStateFailing(t *testing.T) {
	t.Parallel()

	require.False(t, (TaskState{}).Failing())
	require.True(t, (TaskState{NextRetry: time.Now()}).Failing())
}

func TestClampRetry(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, time.August, 25, 12, 0, 0, 0, time.UTC)
	maxDelay := 2 * time.Hour

	t.Run("plausible retry is unchanged", func(t *testing.T) {
		t.Parallel()

		rs := TaskState{NextRetry: now.Add(maxDelay)}
		got, clamped := ClampRetry(rs, now, maxDelay)
		require.False(t, clamped)
		require.Equal(t, rs, got)
	})

	t.Run("implausible retry is clamped to now", func(t *testing.T) {
		t.Parallel()

		rs := TaskState{NextRetry: now.Add(maxDelay + time.Nanosecond)}
		got, clamped := ClampRetry(rs, now, maxDelay)
		require.True(t, clamped)
		require.Equal(t, now, got.NextRetry)
	})
}

func TestApplyOutcome(t *testing.T) {
	t.Parallel()

	start := time.Date(2026, time.August, 25, 12, 0, 0, 0, time.UTC)
	observed := start.Add(3 * time.Minute)
	policy := func(failures int) time.Duration { return time.Duration(failures) * time.Minute }

	t.Run("success stamps the start and clears the ladder", func(t *testing.T) {
		t.Parallel()

		ts := TaskState{Failures: 3, NextRetry: observed, Version: 7}
		got, changed := ApplyOutcome(ts, start, Outcome{At: observed}, policy)
		require.True(t, changed)
		require.Equal(t, TaskState{LastRun: start, Version: 8}, got)
	})

	t.Run("failure advances the ladder from when it was observed", func(t *testing.T) {
		t.Parallel()

		ts := TaskState{Failures: 1, Version: 2}
		got, changed := ApplyOutcome(ts, start, Outcome{Error: "boom", At: observed}, policy)
		require.True(t, changed)
		require.Equal(t, 2, got.Failures)
		require.Equal(t, observed.Add(2*time.Minute), got.NextRetry)
		require.Equal(t, int64(3), got.Version)
	})

	t.Run("an older failure cannot resurrect the ladder after a newer success", func(t *testing.T) {
		t.Parallel()

		ts := TaskState{LastRun: start.Add(time.Hour), Version: 4}
		got, changed := ApplyOutcome(ts, start, Outcome{Error: "late", At: observed}, policy)
		require.False(t, changed)
		require.Equal(t, ts, got)
	})

	t.Run("an older success cannot regress a newer one", func(t *testing.T) {
		t.Parallel()

		ts := TaskState{LastRun: start.Add(time.Hour), Version: 4}
		got, changed := ApplyOutcome(ts, start, Outcome{At: observed}, policy)
		require.False(t, changed)
		require.Equal(t, ts, got)
	})
}

func TestChildOutcome(t *testing.T) {
	t.Parallel()

	at := time.Date(2026, time.August, 25, 12, 0, 0, 0, time.UTC)
	require.Equal(t, Outcome{At: at}, ChildOutcome(Run{Children: 3, ChildrenSettled: 3}, "", at))

	got := ChildOutcome(Run{Children: 3, ChildrenSettled: 3, ChildrenFailed: 2}, "smtp down", at)
	require.Equal(t, "2 of 3 for_each subjects failed; first: smtp down", got.Error)
}
