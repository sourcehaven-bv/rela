package schedulerstate

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestRunStateFailing(t *testing.T) {
	t.Parallel()

	require.False(t, (RunState{}).Failing())
	require.True(t, (RunState{NextRetry: time.Now()}).Failing())
}

func TestClampRetry(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, time.August, 25, 12, 0, 0, 0, time.UTC)
	maxDelay := 2 * time.Hour

	t.Run("plausible retry is unchanged", func(t *testing.T) {
		rs := RunState{NextRetry: now.Add(maxDelay)}
		got, clamped := ClampRetry(rs, now, maxDelay)
		require.False(t, clamped)
		require.Equal(t, rs, got)
	})

	t.Run("implausible retry is clamped to now", func(t *testing.T) {
		rs := RunState{NextRetry: now.Add(maxDelay + time.Nanosecond)}
		got, clamped := ClampRetry(rs, now, maxDelay)
		require.True(t, clamped)
		require.Equal(t, now, got.NextRetry)
	})
}
