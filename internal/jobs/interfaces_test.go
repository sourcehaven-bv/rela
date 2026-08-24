package jobs_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/jobs"
)

// producer is how an ordinary subsystem should depend on the queue: it can
// submit work and nothing else. It cannot register handlers, and — the point —
// it cannot start or stop the queue for everyone else.
type producer struct {
	q jobs.Enqueuer
}

func (p producer) notify(ctx context.Context, entityID string) error {
	return p.q.Enqueue(ctx, jobs.Job{
		Kind:    notifyKind,
		Payload: map[string]any{"entity": entityID},
		Retry:   jobs.RetryBounded,
	})
}

// consumer is how a subsystem that OWNS a kind should take the queue: it
// registers its handler once, at wiring time, with the registrar injected.
type consumer struct {
	seen chan string
}

func (c *consumer) register(r jobs.Registrar) error {
	return r.Register(notifyKind, func(_ context.Context, job jobs.Job) error {
		c.seen <- job.Payload["entity"].(string)
		return nil
	})
}

var notifyKind = jobs.NewKind("test", "notify")

// TestNarrowInterfaces_Compose is a compile-time-plus-runtime statement of the
// intended dependency shape.
//
// It exists because the split only pays off if consumers actually take the
// narrow interfaces. A future change that widens producer.q to jobs.Queue would
// still compile and still pass its own tests — this test is where that shows up
// as a deliberate choice rather than drift.
func TestNarrowInterfaces_Compose(t *testing.T) {
	q, err := jobs.NewMemoryQueue(context.Background(), discardLogger())
	require.NoError(t, err)
	t.Cleanup(func() { _ = q.Close(context.Background()) })

	// Wiring site: it alone holds the full Queue, and it alone starts it.
	var lifecycle jobs.Lifecycle = q

	c := &consumer{seen: make(chan string, 1)}
	require.NoError(t, c.register(q))
	require.NoError(t, lifecycle.Start(context.Background()))

	p := producer{q: q}
	require.NoError(t, p.notify(context.Background(), "TKT-1"))

	require.Equal(t, "TKT-1", <-c.seen)
}

// TestNewKind_Namespaces pins that a kind carries its owner.
//
// The collision this prevents is not hypothetical: every subsystem registers
// into ONE queue, so two packages that both chose "sync" would fail at
// Register — or, if the ordering ever changed, route one subsystem's jobs into
// another's handler.
func TestNewKind_Namespaces(t *testing.T) {
	require.Equal(t, jobs.Kind("scheduler:run-task"), jobs.NewKind("scheduler", "run-task"))
	require.NotEqual(t, jobs.NewKind("scheduler", "sync"), jobs.NewKind("mail", "sync"),
		"the same bare name under different owners must not collide")
	require.Equal(t, "scheduler:run-task", jobs.NewKind("scheduler", "run-task").String())
}
