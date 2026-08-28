package jobs

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// TestRetry_MaxAttempts pins the attempt budget each policy permits.
//
// These are TOTAL executions including the first, and this package enforces
// them itself in the dispatcher — the number is never handed to neoq, whose
// worker goroutine dies when it evaluates its own exhaustion. See
// backendRetryBudget.
func TestRetry_MaxAttempts(t *testing.T) {
	tests := []struct {
		name  string
		retry Retry
		want  int
	}{
		{"never runs exactly once", RetryNever, 1},
		{"bounded gets its attempt budget", RetryBounded, boundedAttempts},
		{"persistent gets its attempt budget", RetryPersistent, persistentAttempts},
		{"an out-of-range policy falls back to a single attempt", Retry(99), 1},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.want, tc.retry.maxAttempts())
		})
	}
}

// TestBackendRetryBudget_IsUnreachable guards the containment for the upstream
// worker-suicide bug.
//
// neoq returns from its worker goroutine when a job exceeds MaxRetries, so the
// budget handed to it must be one no job can reach. If someone "tidies" this
// back to the real per-policy budget, the pool starts dying under exactly the
// failure load the retry policy exists to absorb — silently.
func TestBackendRetryBudget_IsUnreachable(t *testing.T) {
	require.Greater(t, backendRetryBudget, persistentAttempts*1000,
		"the backend budget must be far beyond any policy's real budget")
}

// TestRetry_EffectiveDeadline covers the interaction between a caller's
// deadline and RetryPersistent's outer time bound.
func TestRetry_EffectiveDeadline(t *testing.T) {
	now := time.Date(2026, 8, 23, 12, 0, 0, 0, time.UTC)

	tests := []struct {
		name  string
		retry Retry
		given time.Time
		want  time.Time
	}{
		{
			name:  "no deadline stays absent for a bounded job",
			retry: RetryBounded,
			given: time.Time{},
			want:  time.Time{},
		},
		{
			name:  "caller deadline is preserved for a bounded job",
			retry: RetryBounded,
			given: now.Add(time.Hour),
			want:  now.Add(time.Hour),
		},
		{
			name:  "persistent without a deadline gets the outer window",
			retry: RetryPersistent,
			given: time.Time{},
			want:  now.Add(persistentWindow),
		},
		{
			name: "a sooner caller deadline wins over the window: the caller " +
				"knows something the queue does not",
			retry: RetryPersistent,
			given: now.Add(time.Minute),
			want:  now.Add(time.Minute),
		},
		{
			name:  "a later caller deadline is clamped to the window",
			retry: RetryPersistent,
			given: now.Add(30 * 24 * time.Hour),
			want:  now.Add(persistentWindow),
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := tc.retry.effectiveDeadline(Job{Deadline: tc.given}, now)
			require.Equal(t, tc.want, got)
		})
	}
}

// TestJob_ExpiredZeroDeadline pins the zero-value trap: an unset deadline must
// mean "no deadline", not the epoch — which would expire every job that left
// the field alone.
func TestJob_ExpiredZeroDeadline(t *testing.T) {
	now := time.Now()
	require.False(t, Job{}.expired(now), "zero deadline must mean no deadline")
	require.True(t, Job{Deadline: now.Add(-time.Second)}.expired(now))
	require.False(t, Job{Deadline: now.Add(time.Second)}.expired(now))
	require.True(t, Job{Deadline: now}.expired(now), "deadline exactly now has passed")
}

func TestJob_Validate(t *testing.T) {
	require.ErrorIs(t, Job{}.validate(), ErrNoKind)
	require.NoError(t, Job{Kind: "k"}.validate())
	require.Error(t, Job{Kind: "k", Retry: Retry(99)}.validate(),
		"an out-of-range retry must not be silently treated as RetryNever")
}

func TestRetry_String(t *testing.T) {
	require.Equal(t, "never", RetryNever.String())
	require.Equal(t, "bounded", RetryBounded.String())
	require.Equal(t, "persistent", RetryPersistent.String())
	require.Equal(t, "Retry(99)", Retry(99).String())
}
