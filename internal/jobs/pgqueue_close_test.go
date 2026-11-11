//go:build postgres

package jobs_test

import (
	"context"
	"os"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/jobs"
)

// TestPostgresQueue_CloseRequeuesOnlyRetryableJobs pins what happens to a job
// whose handler Close interrupts. A job with attempts left goes back to the
// queue without using one, so the next process runs it. A RetryNever job does
// not: its side effect may already have happened.
func TestPostgresQueue_CloseRequeuesOnlyRetryableJobs(t *testing.T) {
	admin := os.Getenv("RELA_TEST_DATABASE_URL")
	if admin == "" {
		t.Skip("RELA_TEST_DATABASE_URL not set; skipping postgres close test")
	}
	ctx := context.Background()
	dsn := schemaPinnedDSN(t, admin, "rela_jobs_close_test")

	var never, bounded atomic.Int32
	var block atomic.Bool
	block.Store(true)
	handler := func(counter *atomic.Int32) jobs.Handler {
		return func(ctx context.Context, _ jobs.Job) error {
			counter.Add(1)
			if block.Load() {
				<-ctx.Done()
				return ctx.Err()
			}
			return nil
		}
	}
	newQueue := func() jobs.Queue {
		q, err := jobs.NewPostgresQueue(ctx, discardLogger(), dsn)
		require.NoError(t, err)
		require.NoError(t, q.Register("close:never", handler(&never)))
		require.NoError(t, q.Register("close:bounded", handler(&bounded)))
		require.NoError(t, q.Start(ctx))
		return q
	}

	first := newQueue()
	require.NoError(t, first.Enqueue(ctx, jobs.Job{Kind: "close:never", Retry: jobs.RetryNever}))
	require.NoError(t, first.Enqueue(ctx, jobs.Job{Kind: "close:bounded", Retry: jobs.RetryBounded}))
	require.Eventually(t, func() bool { return never.Load() == 1 && bounded.Load() == 1 },
		10*time.Second, 50*time.Millisecond)
	require.NoError(t, first.Close(ctx))

	block.Store(false)
	second := newQueue()
	t.Cleanup(func() { _ = second.Close(context.Background()) })
	require.Eventually(t, func() bool { return bounded.Load() == 2 }, 10*time.Second, 50*time.Millisecond,
		"a job with attempts left must run again after close")
	require.Equal(t, int32(1), never.Load(), "a RetryNever job must not run again after close")
}
