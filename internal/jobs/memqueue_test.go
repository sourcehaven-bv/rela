package jobs_test

import (
	"context"
	"log/slog"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/jobs"
	"github.com/Sourcehaven-BV/rela/internal/jobs/jobstest"
)

// discardLogger keeps conformance output readable — these tests deliberately
// provoke handler failures, and the resulting warn lines are expected.
func discardLogger() *slog.Logger {
	return slog.New(slog.DiscardHandler)
}

func TestMemoryQueue_Conformance(t *testing.T) {
	jobstest.RunAll(t, func(t *testing.T) jobs.Queue {
		t.Helper()
		q, err := jobs.NewMemoryQueue(context.Background(), discardLogger())
		require.NoError(t, err)
		t.Cleanup(func() { _ = q.Close(context.Background()) })
		return q
	})
}

func TestNewMemoryQueue_RejectsNilLogger(t *testing.T) {
	// Constructors reject nil required collaborators rather than substituting
	// a no-op: a silent no-op logger would hide every job failure.
	_, err := jobs.NewMemoryQueue(context.Background(), nil)
	require.Error(t, err)
}

// TestMemoryQueue_CloseGivesUpOnAStuckHandler: Close waits for running
// handlers only until its context is done, so a handler that ignores
// cancellation cannot wedge shutdown.
func TestMemoryQueue_CloseGivesUpOnAStuckHandler(t *testing.T) {
	q, err := jobs.NewMemoryQueue(context.Background(), discardLogger())
	require.NoError(t, err)
	started, release := make(chan struct{}), make(chan struct{})
	t.Cleanup(func() { close(release) })
	require.NoError(t, q.Register("stuck", func(context.Context, jobs.Job) error {
		close(started)
		<-release
		return nil
	}))
	require.NoError(t, q.Start(context.Background()))
	require.NoError(t, q.Enqueue(context.Background(), jobs.Job{Kind: "stuck", Retry: jobs.RetryNever}))
	select {
	case <-started:
	case <-time.After(10 * time.Second):
		t.Fatal("timed out waiting for handler to run")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	require.ErrorIs(t, q.Close(ctx), context.DeadlineExceeded)
}
