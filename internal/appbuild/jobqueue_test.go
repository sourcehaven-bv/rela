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

// TestJobQueueFor_NoDSNStillAssembles pins that a build WITHOUT a database URL
// still gets a working queue.
//
// The postgres build tag does not imply a postgres store: the composition-root
// tests assemble the tagged binary over an in-memory store with no DSN. An
// earlier version required a DSN from the tag alone, which failed those
// assemblies outright — on a queue they never enqueue to. Durability follows
// the STORE, not the tag.
//
// On the default build this is the only path, so the test is meaningful on
// both and needs no tag of its own.
func TestJobQueueFor_NoDSNStillAssembles(t *testing.T) {
	t.Parallel()

	svc, err := discover(t, writeMinimalProject(t))
	require.NoError(t, err, "assembly without a DSN must not fail")
	t.Cleanup(func() { _ = svc.Close() })

	require.NotNil(t, svc.Jobs(), "assembly without a DSN must still yield a usable queue")
}
