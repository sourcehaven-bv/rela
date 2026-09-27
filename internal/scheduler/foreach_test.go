package scheduler

import (
	"context"
	"errors"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/jobs"
	"github.com/Sourcehaven-BV/rela/internal/schedulerstate"
)

type forEachWorkspace struct {
	*mockWorkspace
	mu        sync.Mutex
	ids       []string
	dropped   int
	principal map[string]string
	templates []string
}

func (w *forEachWorkspace) RunScheduledTemplate(_ context.Context, template, subject string) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.templates = append(w.templates, template+":"+subject)
	return nil
}

func (w *forEachWorkspace) ScheduledForEachEntities(
	context.Context, string, []string, int,
) (ids []string, dropped int, err error) {
	return append([]string(nil), w.ids...), w.dropped, nil
}

func (w *forEachWorkspace) ScheduledForEachPrincipal(_ context.Context, id string) (string, error) {
	return w.principal[id], nil
}

func newForEachWorkspace(t *testing.T, ids ...string) *forEachWorkspace {
	t.Helper()
	principal := make(map[string]string, len(ids))
	for _, id := range ids {
		principal[id] = "user:" + id
	}
	return &forEachWorkspace{mockWorkspace: newMockWorkspace(t), ids: ids, principal: principal}
}

func digestTask() TaskConfig {
	return TaskConfig{
		Name: "digest", Script: "digest.lua", Every: dailySchedule(),
		ForEach: &ForEachConfig{EntityType: "person"},
	}
}

// failFor is an engineRunner that fails for the listed principals.
func failFor(rec *recorder, users ...string) func(context.Context, TaskConfig) error {
	return func(ctx context.Context, task TaskConfig) error {
		_ = rec.run(ctx, task)
		if slices.Contains(users, task.RunAs) {
			return errBoom
		}
		return nil
	}
}

func TestForEach_TickQueuesExpansionForTheOccurrence(t *testing.T) {
	t.Parallel()
	ws := newForEachWorkspace(t)
	now := time.Date(2026, time.August, 25, 12, 0, 0, 0, time.Local)
	s, q, _ := newTestScheduler(t, ws, now, digestTask())

	s.tick(context.Background())

	got := q.jobs()
	require.Len(t, got, 1)
	require.Equal(t, ExpandKind, got[0].Kind)
	require.Equal(t, "2026-08-25", got[0].Payload[payloadOccurrence])
	ts, _ := taskState(t, ws, "digest")
	require.Equal(t, ts.Active.ID, got[0].IdempotencyKey)
	require.Equal(t, "2026-08-25", ts.Active.Occurrence)
}

// TestForEach_RunEndsWhenChildrenSettle pins that expansion is not the task's
// outcome: the run stays active until every child settled.
func TestForEach_RunEndsWhenChildrenSettle(t *testing.T) {
	t.Parallel()
	ws := newForEachWorkspace(t, "P-2", "P-1")
	s, q, _ := newTestScheduler(t, ws, t0, digestTask())
	rec := &recorder{}
	s.engineRunner = rec.run

	s.tick(context.Background())
	require.NoError(t, q.deliver(t, q.take()[0]))

	children := q.take()
	require.Len(t, children, 2)
	require.Equal(t, "P-1", children[0].Payload[payloadSubject], "children are deterministic")
	require.Equal(t, jobs.RetryBounded, children[0].Retry)
	require.Equal(t, children[0].Payload[payloadRunID].(string)+"/P-1", children[0].IdempotencyKey)
	require.NotContains(t, children[0].Payload, payloadScript)
	require.NotContains(t, children[0].Payload, payloadRunAs)

	ts, _ := taskState(t, ws, "digest")
	require.NotNil(t, ts.Active, "the run is active until its children settle")

	for _, child := range children {
		require.NoError(t, q.deliver(t, child))
	}
	ts, _ = taskState(t, ws, "digest")
	require.Nil(t, ts.Active)
	require.True(t, ts.LastRun.Equal(t0))
	require.Equal(t, 2, rec.count())
	require.Equal(t, "user:P-1", rec.runs[0].RunAs, "each child runs as its subject's principal")
}

// TestForEach_ChildOfAbandonedRunDoesNotExecute pins that a child still queued
// when its run is reaped does not run: the retry run owns the subject, and
// running both would deliver twice.
func TestForEach_ChildOfAbandonedRunDoesNotExecute(t *testing.T) {
	t.Parallel()
	ws := newForEachWorkspace(t, "P-1")
	s, q, c := newTestScheduler(t, ws, t0, digestTask())
	rec := &recorder{}
	s.engineRunner = rec.run

	s.tick(context.Background())
	require.NoError(t, q.deliver(t, q.take()[0]))
	stale := q.take()
	require.Len(t, stale, 1)

	c.Advance(runningLease + time.Minute)
	s.tick(context.Background())
	ts, _ := taskState(t, ws, "digest")
	require.Equal(t, 1, ts.Failures, "the run was abandoned")

	require.NoError(t, q.deliver(t, stale[0]))
	require.Zero(t, rec.count(), "a child of an abandoned run must not execute")
}

// TestForEach_RedeliveredChildDoesNotExecuteAgain pins that a child delivered
// again after it settled (the queue could not record its completion) does not
// repeat its side effect.
func TestForEach_RedeliveredChildDoesNotExecuteAgain(t *testing.T) {
	t.Parallel()
	ws := newForEachWorkspace(t, "P-1", "P-2")
	s, q, _ := newTestScheduler(t, ws, t0, digestTask())
	rec := &recorder{}
	s.engineRunner = rec.run

	s.tick(context.Background())
	require.NoError(t, q.deliver(t, q.take()[0]))
	children := q.take()
	require.NoError(t, q.deliver(t, children[0]))
	require.NoError(t, q.deliver(t, children[0]))

	require.Equal(t, 1, rec.count(), "the redelivered child is skipped")
	ts, _ := taskState(t, ws, "digest")
	require.NotNil(t, ts.Active, "the run still waits for its other subject")
}

// TestForEach_ChildFailureFailsTheRun pins BUG-1YMHIS
// (AM-foreach-child-failure-records-failure): a fan-out where a child failed
// records a failure, so the ladder advances.
func TestForEach_ChildFailureFailsTheRun(t *testing.T) {
	t.Parallel()
	ws := newForEachWorkspace(t, "P-1", "P-2")
	s, q, _ := newTestScheduler(t, ws, t0, digestTask())
	rec := &recorder{}
	s.engineRunner = failFor(rec, "user:P-2")

	s.tick(context.Background())
	require.NoError(t, q.deliver(t, q.take()[0]))
	for _, child := range q.take() {
		child.Attempt = finalAttempt
		_ = q.deliver(t, child)
	}

	ts, _ := taskState(t, ws, "digest")
	require.Nil(t, ts.Active)
	require.Equal(t, 1, ts.Failures)
	require.True(t, ts.LastRun.IsZero())
}

// TestForEach_NonFinalChildFailureDoesNotSettle pins that a child with
// retries left hands its error back to the queue rather than settling.
func TestForEach_NonFinalChildFailureDoesNotSettle(t *testing.T) {
	t.Parallel()
	ws := newForEachWorkspace(t, "P-1")
	s, q, _ := newTestScheduler(t, ws, t0, digestTask())
	s.engineRunner = failFor(&recorder{}, "user:P-1")

	s.tick(context.Background())
	require.NoError(t, q.deliver(t, q.take()[0]))
	child := q.take()[0]
	require.ErrorIs(t, q.deliver(t, child), errBoom)

	ts, _ := taskState(t, ws, "digest")
	require.NotNil(t, ts.Active)
	require.Zero(t, ts.Active.ChildrenSettled)
}

// TestForEach_RetrySkipsDeliveredSubjects pins that retrying a partly failed
// fan-out repeats only the subjects that failed, not the ones delivered.
func TestForEach_RetrySkipsDeliveredSubjects(t *testing.T) {
	t.Parallel()
	ws := newForEachWorkspace(t, "P-1", "P-2")
	s, q, c := newTestScheduler(t, ws, t0, digestTask())
	rec := &recorder{}
	s.engineRunner = failFor(rec, "user:P-2")
	runAll := func() {
		require.NoError(t, q.deliver(t, q.take()[0]))
		for _, child := range q.take() {
			child.Attempt = finalAttempt
			_ = q.deliver(t, child)
		}
	}

	s.tick(context.Background())
	runAll()
	c.Advance(baseRetryDelay + time.Minute)
	s.engineRunner = failFor(rec) // P-2 recovers
	s.tick(context.Background())
	runAll()

	var ranFor []string
	for _, r := range rec.runs {
		ranFor = append(ranFor, r.RunAs)
	}
	require.Equal(t, []string{"user:P-1", "user:P-2", "user:P-2"}, ranFor)
	ts, _ := taskState(t, ws, "digest")
	require.Zero(t, ts.Failures)
	require.False(t, ts.LastRun.IsZero())
}

func TestForEach_NoSubjectsSucceeds(t *testing.T) {
	t.Parallel()
	ws := newForEachWorkspace(t)
	s, q, _ := newTestScheduler(t, ws, t0, digestTask())

	s.tick(context.Background())
	q.drain(t)

	ts, _ := taskState(t, ws, "digest")
	require.Nil(t, ts.Active)
	require.True(t, ts.LastRun.Equal(t0))
}

func TestForEach_ChildRunsTemplate(t *testing.T) {
	t.Parallel()
	ws := newForEachWorkspace(t, "P-1")
	task := digestTask()
	task.Script, task.Template = "", "overdue"
	s, q, _ := newTestScheduler(t, ws, t0, task)

	s.tick(context.Background())
	q.drain(t)

	require.Equal(t, []string{"overdue:P-1"}, ws.templates)
	ts, _ := taskState(t, ws, "digest")
	require.True(t, ts.LastRun.Equal(t0))
}

func TestForEach_EnqueueFailureSettlesChildAsFailed(t *testing.T) {
	t.Parallel()
	ws := newForEachWorkspace(t, "P-1")
	s, q, _ := newTestScheduler(t, ws, t0, digestTask())

	s.tick(context.Background())
	expand := q.take()[0]
	q.enqueErr = errors.New("queue down")
	require.NoError(t, q.deliver(t, expand))

	ts, _ := taskState(t, ws, "digest")
	require.Nil(t, ts.Active)
	require.Equal(t, 1, ts.Failures)
}

func TestForEach_ProgressExtendsLease(t *testing.T) {
	t.Parallel()
	ws := newForEachWorkspace(t, "P-1", "P-2")
	s, q, c := newTestScheduler(t, ws, t0, digestTask())
	s.engineRunner = (&recorder{}).run

	s.tick(context.Background())
	require.NoError(t, q.deliver(t, q.take()[0]))
	children := q.take()

	c.Advance(runningLease - time.Minute)
	require.NoError(t, q.deliver(t, children[0]))
	c.Advance(runningLease - time.Minute)
	s.tick(context.Background()) // past the original lease, within the extended one
	ts, _ := taskState(t, ws, "digest")
	require.NotNil(t, ts.Active, "a child's progress must keep the run alive")
	require.Equal(t, schedulerstate.RunRunning, ts.Active.Status)
}

// finalAttempt is beyond every retry budget, so a job carrying it is on its
// final attempt whatever its Retry.
const finalAttempt = 1 << 20
