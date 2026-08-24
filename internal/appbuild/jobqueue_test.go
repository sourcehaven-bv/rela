package appbuild_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/jobs"
)

// TestServices_JobsIsWired asserts every assembled Services carries a usable
// job queue.
//
// The failure this guards against is a nil queue reaching a producer: enqueue
// sites call svc.Jobs().Enqueue(...) directly, so a nil would surface as a
// panic deep in a handler rather than as a wiring error at startup.
func TestServices_JobsIsWired(t *testing.T) {
	svc, err := discover(t, writeMinimalProject(t))
	require.NoError(t, err)
	t.Cleanup(func() { _ = svc.Close() })

	q := svc.Jobs()
	require.NotNil(t, q, "assembled Services must carry a job queue")

	// The returned type is jobs.Client — Enqueue and Register, no lifecycle.
	// A consumer must not be able to Close the queue out from under every
	// other subsystem, so the assertion is that Lifecycle is NOT reachable
	// through the accessor.
	_, hasLifecycle := q.(jobs.Lifecycle)
	require.False(t, hasLifecycle,
		"Services.Jobs must not expose lifecycle control to consumers")

	// Usable, not merely non-nil: register and round-trip one job. Handlers
	// may be registered after start; the dispatcher resolves per job.
	done := make(chan jobs.Job, 1)
	require.NoError(t, q.Register("wiring-probe", func(_ context.Context, job jobs.Job) error {
		done <- job
		return nil
	}))
	require.NoError(t, q.Enqueue(context.Background(), jobs.Job{
		Kind:    "wiring-probe",
		Payload: map[string]any{"ok": "yes"},
	}))

	select {
	case got := <-done:
		require.Equal(t, "yes", got.Payload["ok"])
	case <-time.After(10 * time.Second):
		t.Fatal("timed out waiting for the wired queue to run a job")
	}
}
