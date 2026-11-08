package jobs_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/goleak"

	"github.com/Sourcehaven-BV/rela/internal/jobs"
)

// openCloseCycles is how many queues the goroutine tests open and close. A
// leak of one goroutine per queue then shows as twenty leaked stacks.
const openCloseCycles = 20

// requireCloseReleasesGoroutines opens a queue with open, starts it and closes
// it, openCloseCycles times, and fails t if any goroutine started during those
// cycles is still running afterwards. One warm-up cycle runs first, so
// goroutines a dependency starts once per process are not counted.
//
// Tests using it must not call t.Parallel: a goroutine from a test running
// alongside would count as a leak.
func requireCloseReleasesGoroutines(t *testing.T, open func(t *testing.T) jobs.Queue) {
	t.Helper()
	ctx := context.Background()
	cycle := func() {
		q := open(t)
		require.NoError(t, q.Register("noop", func(context.Context, jobs.Job) error { return nil }))
		require.NoError(t, q.Start(ctx))
		require.NoError(t, q.Close(ctx))
	}

	cycle()
	ignore := goleak.IgnoreCurrent()
	for range openCloseCycles {
		cycle()
	}
	goleak.VerifyNone(t, ignore)
}

// TestMemoryQueue_CloseReleasesGoroutines pins TKT-736J0CZ: neoq's memory
// backend started a cron goroutine that Shutdown never stopped, so each
// closed queue leaked one. It fails if go.mod's neoq replace is dropped.
func TestMemoryQueue_CloseReleasesGoroutines(t *testing.T) {
	requireCloseReleasesGoroutines(t, func(t *testing.T) jobs.Queue {
		t.Helper()
		q, err := jobs.NewMemoryQueue(context.Background(), discardLogger())
		require.NoError(t, err)
		return q
	})
}
