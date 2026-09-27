package scheduler

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/jobs"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/script"
)

// TestQueuedJob_Shape pins how a run reaches the queue: keyed by its run id
// (never the bare task name, which let one stuck row block the task forever,
// BUG-TKL08E), RetryNever because the scheduler owns retrying, and carrying
// everything a worker in another process needs.
func TestQueuedJob_Shape(t *testing.T) {
	t.Parallel()
	ws := newMockWorkspace(t)
	s, q, _ := newTestScheduler(t, ws, t0, TaskConfig{
		Name: "nightly", Script: "reports.lua", RunAs: "system:reporting", Every: dailySchedule(),
		Capabilities: metamodel.Capabilities{HTTP: true, Secrets: []string{"token"}},
	})

	s.tick(context.Background())

	got := q.jobs()
	require.Len(t, got, 1)
	job := got[0]
	ts, _ := taskState(t, ws, "nightly")
	require.NotNil(t, ts.Active)
	require.Equal(t, TaskKind, job.Kind)
	require.Equal(t, ts.Active.ID, job.IdempotencyKey)
	require.Equal(t, ts.Active.ID, job.Payload[payloadRunID])
	require.Equal(t, jobs.RetryNever, job.Retry)
	require.True(t, job.Deadline.IsZero(), "a schedule must never expire its job; see the jobs package doc")
	require.Equal(t, "nightly", job.Payload[payloadTaskName])
	require.Equal(t, "reports.lua", job.Payload[payloadScript])
	require.Equal(t, "system:reporting", job.Payload[payloadRunAs])
	require.Equal(t, true, job.Payload[payloadHTTP])
	require.Equal(t, []string{"token"}, job.Payload[payloadSecrets])
}

// TestRunTaskJob_LegacyJobIsDropped pins the rollout path: a job queued by an
// older release carries no run id, and must not run unrecorded.
func TestRunTaskJob_LegacyJobIsDropped(t *testing.T) {
	t.Parallel()
	s, _, _ := newTestScheduler(t, newMockWorkspace(t), t0)
	rec := &recorder{}
	s.engineRunner = rec.run

	require.NoError(t, s.runTaskJob(context.Background(), jobs.Job{Payload: map[string]any{
		payloadTaskName: "old", payloadScript: "old.lua",
	}}))
	require.Zero(t, rec.count())
}

func TestRunTaskJob_EmptyScriptFailsTheRun(t *testing.T) {
	t.Parallel()
	ws := newMockWorkspace(t)
	s, q, _ := newTestScheduler(t, ws, t0, TaskConfig{Name: "t", Script: "t.lua", Every: dailySchedule()})
	s.tick(context.Background())
	job := q.take()[0]
	delete(job.Payload, payloadScript)

	require.Error(t, q.deliver(t, job))
	ts, _ := taskState(t, ws, "t")
	require.Equal(t, 1, ts.Failures)
}

func TestCapabilitiesFromPayload(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		payload map[string]any
		want    metamodel.Capabilities
	}{
		{"durable JSON shape", map[string]any{
			payloadHTTP: true, payloadAI: true, payloadMail: true, payloadWriteFile: true,
			payloadSecrets: []any{"reporting_token", "mail_password"},
		}, metamodel.Capabilities{
			HTTP: true, AI: true, Mail: true, WriteFile: true,
			Secrets: []string{"reporting_token", "mail_password"},
		}},
		{"empty grant stays closed", map[string]any{
			payloadHTTP: false, payloadSecrets: []any{},
		}, metamodel.Capabilities{}},
		{"malformed grant fails closed", map[string]any{
			payloadHTTP: "true", payloadAI: 1, payloadMail: "yes", payloadWriteFile: []any{true},
			payloadSecrets: []any{42, true, nil},
		}, metamodel.Capabilities{}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := capabilitiesFromPayload(tc.payload)
			require.Equal(t, tc.want.Any(), got.Any())
			require.Equal(t, tc.want.HTTP, got.HTTP)
			require.ElementsMatch(t, tc.want.Secrets, got.Secrets)
		})
	}
}

// newMemoryQueue starts a real in-memory queue for the test.
func newMemoryQueue(t *testing.T) jobs.Queue {
	t.Helper()
	q, err := jobs.NewMemoryQueue(context.Background(), discardLogger())
	require.NoError(t, err)
	t.Cleanup(func() { _ = q.Close(context.Background()) })
	return q
}

// realQueueScheduler builds a scheduler on a real memory queue and the wall
// clock.
func realQueueScheduler(t *testing.T, q jobs.Queue, task TaskConfig) (*Scheduler, *mockWorkspace) {
	t.Helper()
	ws := &queueWorkspace{mockWorkspace: newMockWorkspace(t), q: q}
	s, err := NewWithQueue(&Config{Tasks: []TaskConfig{task}}, script.NewEngine(), ws, discardLogger())
	require.NoError(t, err)
	return s, ws.mockWorkspace
}

// TestScheduler_ThroughRealQueue runs the whole path on the real memory
// backend: tick, a worker on another goroutine, the outcome recorded by that
// worker. The payload must survive the hop, including run_as and the
// capability snapshot.
func TestScheduler_ThroughRealQueue(t *testing.T) {
	t.Parallel()
	q := newMemoryQueue(t)
	caps := metamodel.Capabilities{HTTP: true, AI: true, WriteFile: true, Secrets: []string{"a", "b"}}
	s, ws := realQueueScheduler(t, q, TaskConfig{
		Name: "nightly", Script: "reports.lua", RunAs: "system:reporting", Capabilities: caps,
		Every: intervalSchedule(time.Hour),
	})
	var (
		mu  sync.Mutex
		ran TaskConfig
	)
	s.engineRunner = func(_ context.Context, task TaskConfig) error {
		mu.Lock()
		defer mu.Unlock()
		ran = task
		return nil
	}
	require.NoError(t, q.Start(context.Background()))

	s.tick(context.Background())

	require.Eventually(t, func() bool {
		ts, ok := taskState(t, ws, "nightly")
		return ok && !ts.LastRun.IsZero() && ts.Active == nil
	}, 10*time.Second, 20*time.Millisecond, "the worker must record the run's success")
	mu.Lock()
	defer mu.Unlock()
	require.Equal(t, "reports.lua", ran.Script)
	require.Equal(t, "system:reporting", ran.RunAs)
	require.Equal(t, caps, ran.Capabilities)
}

// TestOverloadedQueue_RunsLate pins that a run stuck behind a saturated worker
// pool runs late rather than being dropped, and that the tick is not blocked
// meanwhile.
func TestOverloadedQueue_RunsLate(t *testing.T) {
	t.Parallel()
	q := newMemoryQueue(t)
	release := make(chan struct{})
	slow := jobs.NewKind("test", "slow")
	require.NoError(t, q.Register(slow, func(ctx context.Context, _ jobs.Job) error {
		select {
		case <-release:
		case <-ctx.Done():
		}
		return nil
	}))
	s, ws := realQueueScheduler(t, q, TaskConfig{
		Name: "frequent", Script: "f.lua", Every: intervalSchedule(time.Second),
	})
	s.engineRunner = func(context.Context, TaskConfig) error { return nil }
	require.NoError(t, q.Start(context.Background()))
	for range 8 {
		require.NoError(t, q.Enqueue(context.Background(), jobs.Job{Kind: slow}))
	}
	time.Sleep(200 * time.Millisecond)

	ticked := make(chan struct{})
	go func() {
		s.tick(context.Background())
		close(ticked)
	}()
	select {
	case <-ticked:
	case <-time.After(5 * time.Second):
		t.Fatal("the tick blocked on a queued run")
	}

	time.Sleep(1500 * time.Millisecond) // longer than the task's own interval
	close(release)
	require.Eventually(t, func() bool {
		ts, ok := taskState(t, ws, "frequent")
		return ok && !ts.LastRun.IsZero()
	}, 20*time.Second, 20*time.Millisecond, "a run delayed by load must still run")
}
