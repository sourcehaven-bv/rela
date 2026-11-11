package jobs_test

import (
	"context"
	"errors"
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

// TestMemoryQueue_PanickingHandlerCostsOneAttempt pins that a panic is an
// attempt failure: the job is retried under its policy and the worker lives on.
func TestMemoryQueue_PanickingHandlerCostsOneAttempt(t *testing.T) {
	q, err := jobs.NewMemoryQueue(context.Background(), discardLogger())
	require.NoError(t, err)
	t.Cleanup(func() { _ = q.Close(context.Background()) })

	ran := make(chan struct{}, 16)
	require.NoError(t, q.Register("boom", func(context.Context, jobs.Job) error { panic("handler bug") }))
	require.NoError(t, q.Register("ok", func(context.Context, jobs.Job) error {
		ran <- struct{}{}
		return nil
	}))
	require.NoError(t, q.Start(context.Background()))

	// More panics than workers: had a panic killed its worker, nothing would
	// be left to run the healthy jobs.
	for range 8 {
		require.NoError(t, q.Enqueue(context.Background(), jobs.Job{Kind: "boom"}))
	}
	for range 4 {
		require.NoError(t, q.Enqueue(context.Background(), jobs.Job{Kind: "ok"}))
	}
	for range 4 {
		select {
		case <-ran:
		case <-time.After(10 * time.Second):
			t.Fatal("healthy jobs did not run after handlers panicked")
		}
	}
}

// TestMemoryQueue_KeyHeldWhileAwaitingRetry pins that a keyed job waiting
// for its next attempt is still pending: the key stays taken between attempts
// and is freed when Close drops the job.
func TestMemoryQueue_KeyHeldWhileAwaitingRetry(t *testing.T) {
	q, err := jobs.NewMemoryQueue(context.Background(), discardLogger())
	require.NoError(t, err)
	failed := make(chan struct{}, 1)
	require.NoError(t, q.Register("flaky", func(context.Context, jobs.Job) error {
		select {
		case failed <- struct{}{}:
		default:
		}
		return errors.New("not yet")
	}))
	require.NoError(t, q.Start(context.Background()))

	job := jobs.Job{Kind: "flaky", Retry: jobs.RetryBounded, IdempotencyKey: "k"}
	require.NoError(t, q.Enqueue(context.Background(), job))
	select {
	case <-failed:
	case <-time.After(10 * time.Second):
		t.Fatal("handler did not run")
	}
	// The first retry is at least retryDelayBase away, so the job is now
	// waiting on its timer.
	require.Eventually(t, func() bool {
		return errors.Is(q.Enqueue(context.Background(), job), jobs.ErrDuplicateJob)
	}, 10*time.Second, 20*time.Millisecond)

	require.NoError(t, q.Close(context.Background()))
}

// TestMemoryQueue_PayloadIsCopied pins that neither the caller nor a failed
// attempt can change what the next attempt receives.
func TestMemoryQueue_PayloadIsCopied(t *testing.T) {
	q, err := jobs.NewMemoryQueue(context.Background(), discardLogger())
	require.NoError(t, err)
	t.Cleanup(func() { _ = q.Close(context.Background()) })

	release := make(chan struct{})
	got := make(chan any, 1)
	require.NoError(t, q.Register("blocker", func(context.Context, jobs.Job) error {
		<-release
		return nil
	}))
	require.NoError(t, q.Register("read", func(_ context.Context, job jobs.Job) error {
		got <- []any{job.Payload["v"], job.Payload["list"].([]any)[0]}
		return nil
	}))
	require.NoError(t, q.Start(context.Background()))

	// Hold every worker so the job below stays queued while the caller
	// edits its map.
	for range 4 {
		require.NoError(t, q.Enqueue(context.Background(), jobs.Job{Kind: "blocker"}))
	}
	payload := map[string]any{"v": "original", "list": []any{"original"}}
	require.NoError(t, q.Enqueue(context.Background(), jobs.Job{Kind: "read", Payload: payload}))
	payload["v"] = "changed"
	payload["list"].([]any)[0] = "changed"
	close(release)

	select {
	case v := <-got:
		require.Equal(t, []any{"original", "original"}, v, "nested values must be copied too")
	case <-time.After(10 * time.Second):
		t.Fatal("handler did not run")
	}
}

// TestMemoryQueue_EnqueueBeforeStart pins ErrNotStarted on the direct path.
func TestMemoryQueue_EnqueueBeforeStart(t *testing.T) {
	q, err := jobs.NewMemoryQueue(context.Background(), discardLogger())
	require.NoError(t, err)
	t.Cleanup(func() { _ = q.Close(context.Background()) })
	require.NoError(t, q.Register("k", func(context.Context, jobs.Job) error { return nil }))
	require.ErrorIs(t, q.Enqueue(context.Background(), jobs.Job{Kind: "k"}), jobs.ErrNotStarted)
}

// TestCollectorFlush_DuplicateIsNotAnError pins that a flushed job collapsing
// into a pending one is reported as success, as Enqueue's contract says.
func TestCollectorFlush_DuplicateIsNotAnError(t *testing.T) {
	q, err := jobs.NewMemoryQueue(context.Background(), discardLogger())
	require.NoError(t, err)
	t.Cleanup(func() { _ = q.Close(context.Background()) })
	release := make(chan struct{})
	t.Cleanup(func() { close(release) })
	require.NoError(t, q.Register("keyed", func(ctx context.Context, _ jobs.Job) error {
		select {
		case <-release:
		case <-ctx.Done():
		}
		return nil
	}))
	require.NoError(t, q.Start(context.Background()))

	job := jobs.Job{Kind: "keyed", IdempotencyKey: "k"}
	require.NoError(t, q.Enqueue(context.Background(), job))
	ctx, collector := jobs.WithDeferral(context.Background())
	require.NoError(t, q.Enqueue(ctx, job))
	require.NoError(t, collector.Flush(context.Background(), q))
}
