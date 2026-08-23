package scheduler

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// TestSchedule_NextRun pins the cadence-to-deadline mapping.
//
// NextRun is what lets the job queue stay ignorant of schedules: the scheduler
// converts its own cadence into the queue's generic deadline primitive. If this
// drifts from IsDue, a task either retries past its next run (pointless work) or
// has its retries cancelled before they start.
func TestSchedule_NextRun(t *testing.T) {
	// A Wednesday, mid-afternoon, so weekday arithmetic has to wrap.
	base := time.Date(2026, 8, 26, 15, 30, 0, 0, time.UTC)

	tests := []struct {
		name     string
		schedule string
		lastRun  time.Time
		want     time.Time
	}{
		{
			name:     "interval adds the interval",
			schedule: "30m",
			lastRun:  base,
			want:     base.Add(30 * time.Minute),
		},
		{
			name:     "short interval stays short: this is the 60s case the queue must not outlive",
			schedule: "1m",
			lastRun:  base,
			want:     base.Add(time.Minute),
		},
		{
			name:     "day rolls to the next local midnight, not +24h from lastRun",
			schedule: "day",
			lastRun:  base,
			want:     time.Date(2026, 8, 27, 0, 0, 0, 0, time.UTC),
		},
		{
			name:     "weekday later this week",
			schedule: "friday",
			lastRun:  base, // Wednesday
			want:     time.Date(2026, 8, 28, 0, 0, 0, 0, time.UTC),
		},
		{
			name:     "same weekday wraps a full week rather than returning today",
			schedule: "wednesday",
			lastRun:  base, // Wednesday
			want:     time.Date(2026, 9, 2, 0, 0, 0, 0, time.UTC),
		},
		{
			name:     "weekday earlier in the week wraps forward",
			schedule: "monday",
			lastRun:  base, // Wednesday
			want:     time.Date(2026, 8, 31, 0, 0, 0, 0, time.UTC),
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			s, err := parseSchedule(tc.schedule)
			require.NoError(t, err)
			require.Equal(t, tc.want, s.NextRun(tc.lastRun))
		})
	}
}

// TestSchedule_NextRunIsAlwaysInTheFuture guards the property the deadline
// mapping depends on: a deadline at or before now would suppress every retry
// immediately, silently turning every policy into RetryNever.
func TestSchedule_NextRunIsAlwaysInTheFuture(t *testing.T) {
	lastRun := time.Date(2026, 8, 26, 15, 30, 0, 0, time.UTC)
	for _, raw := range []string{"1m", "30m", "2h", "day", "monday", "wednesday", "sunday"} {
		t.Run(raw, func(t *testing.T) {
			s, err := parseSchedule(raw)
			require.NoError(t, err)
			require.True(t, s.NextRun(lastRun).After(lastRun),
				"NextRun must be strictly after lastRun, got %v", s.NextRun(lastRun))
		})
	}
}

// TestSchedule_NextRunAgreesWithIsDue is the consistency check between the two
// halves. NextRun claims to be the earliest time IsDue holds, so IsDue must be
// false just before it and true at it.
func TestSchedule_NextRunAgreesWithIsDue(t *testing.T) {
	lastRun := time.Date(2026, 8, 26, 15, 30, 0, 0, time.UTC)
	for _, raw := range []string{"1m", "30m", "2h", "day", "monday", "friday"} {
		t.Run(raw, func(t *testing.T) {
			s, err := parseSchedule(raw)
			require.NoError(t, err)
			next := s.NextRun(lastRun)

			require.False(t, s.IsDue(lastRun, next.Add(-time.Second)),
				"must not be due just before NextRun")
			require.True(t, s.IsDue(lastRun, next),
				"must be due at NextRun")
		})
	}
}

// TestSchedule_NextRunUnparsedIsZero pins the fail-safe direction: an unset
// schedule yields no deadline rather than an epoch deadline, so a job keeps its
// own retry budget instead of being cancelled before its first attempt.
func TestSchedule_NextRunUnparsedIsZero(t *testing.T) {
	var s Schedule
	require.True(t, s.NextRun(time.Now()).IsZero())
}
