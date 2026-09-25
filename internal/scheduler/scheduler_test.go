package scheduler

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/jobs"
	"github.com/Sourcehaven-BV/rela/internal/lua"
	"github.com/Sourcehaven-BV/rela/internal/schedulerstate"
	"github.com/Sourcehaven-BV/rela/internal/script"
)

var t0 = time.Date(2026, 4, 10, 14, 0, 0, 0, time.UTC)

// seed stores a task's state as if earlier runs had recorded it.
func seed(t *testing.T, ws WorkspaceProvider, task string, st schedulerstate.TaskState) {
	t.Helper()
	require.NoError(t, ws.SchedulerState().Seed(context.Background(), task, st))
}

// recorder is an engineRunner that records each run and returns err.
type recorder struct {
	mu   sync.Mutex
	runs []TaskConfig
	err  error
}

func (r *recorder) run(_ context.Context, task TaskConfig) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.runs = append(r.runs, task)
	return r.err
}

func (r *recorder) count() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.runs)
}

func TestTick_DueDecisions(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name  string
		every Schedule
		seed  *schedulerstate.TaskState
		want  bool
	}{
		{"first ever run", dailySchedule(), nil, true},
		{"missed day", dailySchedule(), &schedulerstate.TaskState{LastRun: t0.Add(-26 * time.Hour)}, true},
		{"already ran today", dailySchedule(), &schedulerstate.TaskState{LastRun: t0.Add(-time.Hour)}, false},
		{"interval elapsed", intervalSchedule(time.Hour), &schedulerstate.TaskState{LastRun: t0.Add(-61 * time.Minute)}, true},
		{"interval not elapsed", intervalSchedule(time.Hour), &schedulerstate.TaskState{LastRun: t0.Add(-59 * time.Minute)}, false},
		{"retry pending", intervalSchedule(time.Minute), &schedulerstate.TaskState{
			LastRun: t0.Add(-time.Hour), Failures: 1, NextRetry: t0.Add(time.Minute)}, false},
		{"retry due", dailySchedule(), &schedulerstate.TaskState{
			LastRun: t0.Add(-time.Hour), Failures: 1, NextRetry: t0}, true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			ws := newMockWorkspace(t)
			if tc.seed != nil {
				seed(t, ws, "task", *tc.seed)
			}
			s, q, _ := newTestScheduler(t, ws, t0, TaskConfig{Name: "task", Script: "t.lua", Every: tc.every})
			s.tick(context.Background())
			require.Equal(t, tc.want, len(q.jobs()) == 1)
		})
	}
}

// TestTick_NeverWaitsAndSkipsWhileActive pins the heart of BUG-TKL08E: a tick
// returns as soon as the run is queued, and a task whose run is still active is
// skipped rather than stacked.
func TestTick_NeverWaitsAndSkipsWhileActive(t *testing.T) {
	t.Parallel()
	ws := newMockWorkspace(t)
	s, q, c := newTestScheduler(t, ws, t0,
		TaskConfig{Name: "slow", Script: "slow.lua", Every: intervalSchedule(time.Minute)},
		TaskConfig{Name: "other", Script: "other.lua", Every: intervalSchedule(time.Minute)})

	s.tick(context.Background()) // nothing is ever delivered: the runs stay queued
	require.Len(t, q.jobs(), 2)

	c.Advance(5 * time.Minute)
	s.tick(context.Background())
	require.Len(t, q.jobs(), 2, "a task with an active run must be skipped, not stacked")

	ts, ok := taskState(t, ws, "slow")
	require.True(t, ok)
	require.NotNil(t, ts.Active)
	require.Equal(t, schedulerstate.RunQueued, ts.Active.Status)
}

// TestRun_SuccessStampsStartTime pins that success records when the run was
// decided, not when it finished: a run that starts at 23:59 and ends after
// midnight must not consume the next day's slot.
func TestRun_SuccessStampsStartTime(t *testing.T) {
	t.Parallel()
	ws := newMockWorkspace(t)
	start := time.Date(2026, 4, 10, 23, 59, 0, 0, time.UTC)
	s, q, c := newTestScheduler(t, ws, start, TaskConfig{Name: "daily", Script: "d.lua", Every: dailySchedule()})
	s.engineRunner = func(context.Context, TaskConfig) error {
		c.Advance(2 * time.Minute)
		return nil
	}

	s.tick(context.Background())
	q.drain(t)

	ts, ok := taskState(t, ws, "daily")
	require.True(t, ok)
	require.True(t, ts.LastRun.Equal(start), "LastRun = %v, want the start %v", ts.LastRun, start)
	require.Nil(t, ts.Active)
}

// TestRun_FailureAdvancesLadder pins BUG-ZKK2UL through the queue: a failed
// run arms a retry measured from when the failure was observed, and does not
// count as having run.
func TestRun_FailureAdvancesLadder(t *testing.T) {
	t.Parallel()
	ws := newMockWorkspace(t)
	s, q, c := newTestScheduler(t, ws, t0, TaskConfig{Name: "flaky", Script: "f.lua", Every: dailySchedule()})
	s.engineRunner = func(context.Context, TaskConfig) error {
		c.Advance(30 * time.Minute) // longer than the first rung
		return errBoom
	}

	s.tick(context.Background())
	q.drain(t)

	ts, ok := taskState(t, ws, "flaky")
	require.True(t, ok)
	require.Equal(t, 1, ts.Failures)
	require.True(t, ts.LastRun.IsZero(), "a failed run must not stamp a last successful run")
	require.True(t, ts.NextRetry.Equal(t0.Add(30*time.Minute+baseRetryDelay)),
		"a slow failure must retry in the future, not immediately")
}

// TestRun_RetryLadderReplacesSchedule drives a one-minute task that always
// fails: it fires on ladder rungs only, never on its cadence.
func TestRun_RetryLadderReplacesSchedule(t *testing.T) {
	t.Parallel()
	ws := newMockWorkspace(t)
	s, q, c := newTestScheduler(t, ws, t0, TaskConfig{Name: "broken", Script: "b.lua", Every: intervalSchedule(time.Minute)})
	var fired []time.Duration
	s.engineRunner = func(context.Context, TaskConfig) error {
		fired = append(fired, c.Now().Sub(t0))
		return errBoom
	}

	for range 60 { // one hour of ticks
		s.tick(context.Background())
		q.drain(t)
		c.Advance(time.Minute)
	}
	require.Equal(t, []time.Duration{0, 5 * time.Minute, 15 * time.Minute, 35 * time.Minute}, fired)
}

func TestRun_SuccessResetsLadder(t *testing.T) {
	t.Parallel()
	ws := newMockWorkspace(t)
	seed(t, ws, "task", schedulerstate.TaskState{Failures: 3, NextRetry: t0})
	s, q, _ := newTestScheduler(t, ws, t0, TaskConfig{Name: "task", Script: "t.lua", Every: dailySchedule()})
	s.engineRunner = (&recorder{}).run

	s.tick(context.Background())
	q.drain(t)

	ts, _ := taskState(t, ws, "task")
	require.Zero(t, ts.Failures)
	require.True(t, ts.NextRetry.IsZero())
	require.True(t, ts.LastRun.Equal(t0))
}

// TestRun_LostRunIsAbandonedAndRetried is the recovery path: a run whose job
// never reports back (a dead worker, a lost job) is abandoned when its lease
// expires, the ladder advances, and the task runs again. A late delivery of
// the lost job then does nothing.
func TestRun_LostRunIsAbandonedAndRetried(t *testing.T) {
	t.Parallel()
	ws := newMockWorkspace(t)
	s, q, c := newTestScheduler(t, ws, t0, TaskConfig{Name: "task", Script: "t.lua", Every: dailySchedule()})
	rec := &recorder{}
	s.engineRunner = rec.run

	s.tick(context.Background())
	lost := q.take()
	require.Len(t, lost, 1)

	c.Advance(queuedLease + time.Minute)
	s.tick(context.Background())
	ts, _ := taskState(t, ws, "task")
	require.Nil(t, ts.Active, "the expired run must be abandoned")
	require.Equal(t, 1, ts.Failures)

	c.Advance(baseRetryDelay)
	s.tick(context.Background())
	q.drain(t)
	require.Equal(t, 1, rec.count())

	require.NoError(t, q.deliver(t, lost[0]))
	require.Equal(t, 1, rec.count(), "a late delivery of an abandoned run must not execute")
	ts, _ = taskState(t, ws, "task")
	require.True(t, ts.LastRun.Equal(t0.Add(queuedLease+time.Minute+baseRetryDelay)))
}

// TestRun_DuplicateDeliveryRunsOnce pins at-least-once delivery: the queue may
// hand the same job over twice, and the script must still run once.
func TestRun_DuplicateDeliveryRunsOnce(t *testing.T) {
	t.Parallel()
	ws := newMockWorkspace(t)
	s, q, _ := newTestScheduler(t, ws, t0, TaskConfig{Name: "task", Script: "t.lua", Every: dailySchedule()})
	rec := &recorder{}
	s.engineRunner = rec.run

	s.tick(context.Background())
	job := q.take()[0]
	require.NoError(t, q.deliver(t, job))
	require.NoError(t, q.deliver(t, job))
	require.Equal(t, 1, rec.count())
}

// TestRun_EnqueueFailureFailsTheRun pins that a run whose job never reached the
// queue ends at once, so the task is not held until the lease expires.
func TestRun_EnqueueFailureFailsTheRun(t *testing.T) {
	t.Parallel()
	ws := newMockWorkspace(t)
	s, q, _ := newTestScheduler(t, ws, t0, TaskConfig{Name: "task", Script: "t.lua", Every: dailySchedule()})
	q.enqueErr = errors.New("queue down")

	s.tick(context.Background())

	ts, _ := taskState(t, ws, "task")
	require.Nil(t, ts.Active)
	require.Equal(t, 1, ts.Failures)
}

// TestTick_TwoSchedulersShareOneStore is the multi-process guarantee in
// miniature: two schedulers ticking over one store queue one run.
func TestTick_TwoSchedulersShareOneStore(t *testing.T) {
	t.Parallel()
	ws := newMockWorkspace(t)
	task := TaskConfig{Name: "task", Script: "t.lua", Every: dailySchedule()}
	a, qa, _ := newTestScheduler(t, ws, t0, task)
	b, qb, _ := newTestScheduler(t, ws, t0, task)
	rec := &recorder{}
	a.engineRunner, b.engineRunner = rec.run, rec.run

	a.tick(context.Background())
	b.tick(context.Background())
	require.Len(t, append(qa.jobs(), qb.jobs()...), 1)

	// The node that executes records the outcome, whichever one queued it.
	for _, job := range qa.take() {
		require.NoError(t, qb.deliver(t, job))
	}
	ts, _ := taskState(t, ws, "task")
	require.True(t, ts.LastRun.Equal(t0))
}

func TestTick_ImplausibleRetryTimeIsClamped(t *testing.T) {
	t.Parallel()
	ws := newMockWorkspace(t)
	seed(t, ws, "task", schedulerstate.TaskState{Failures: 1, NextRetry: t0.Add(30 * 24 * time.Hour)})
	s, q, _ := newTestScheduler(t, ws, t0, TaskConfig{Name: "task", Script: "t.lua", Every: dailySchedule()})

	s.tick(context.Background())
	require.Len(t, q.jobs(), 1, "a retry beyond the longest rung must be treated as due now")
}

func TestTick_PrunesHourly(t *testing.T) {
	t.Parallel()
	ws := newMockWorkspace(t)
	seed(t, ws, "gone", schedulerstate.TaskState{LastRun: t0.Add(-pruneAge - time.Hour)})
	s, _, c := newTestScheduler(t, ws, t0, TaskConfig{Name: "task", Script: "t.lua", Every: dailySchedule()})

	s.tick(context.Background())
	_, ok := taskState(t, ws, "gone")
	require.False(t, ok, "a task idle since before the cut-off is pruned")

	seed(t, ws, "gone", schedulerstate.TaskState{LastRun: t0.Add(-pruneAge - time.Hour)})
	c.Advance(time.Minute)
	s.tick(context.Background())
	_, ok = taskState(t, ws, "gone")
	require.True(t, ok, "prune runs at most once per interval")
}

func TestImportLegacyState(t *testing.T) {
	t.Parallel()
	ws := newMockWorkspace(t)
	last := t0.Add(-2 * time.Hour)
	retry := t0.Add(10 * time.Minute)
	ws.files[stateFile] = []byte(`{"tasks":{"daily":"` + last.Format(time.RFC3339Nano) + `"},` +
		`"failures":{"flaky":2},"next_retry":{"flaky":"` + retry.Format(time.RFC3339Nano) + `"}}`)
	s, _, _ := newTestScheduler(t, ws, t0, TaskConfig{Name: "daily", Script: "d.lua", Every: dailySchedule()})

	s.importLegacyState(context.Background())

	daily, ok := taskState(t, ws, "daily")
	require.True(t, ok)
	require.True(t, daily.LastRun.Equal(last))
	flaky, ok := taskState(t, ws, "flaky")
	require.True(t, ok)
	require.Equal(t, 2, flaky.Failures)
	require.True(t, flaky.NextRetry.Equal(retry))
	require.NotContains(t, ws.files, stateFile, "the legacy document is deleted once imported")
}

func TestParseState_CorruptIsEmpty(t *testing.T) {
	t.Parallel()
	st := parseState([]byte("not json"))
	require.Empty(t, st.Tasks)
	require.NotNil(t, st.Failures)
	require.NotNil(t, st.NextRetry)
}

func TestNew_RejectsNil(t *testing.T) {
	t.Parallel()
	ws := newMockWorkspace(t)
	cfg := &Config{}
	engine := script.NewEngine()
	for name, build := range map[string]func() (*Scheduler, error){
		"config":    func() (*Scheduler, error) { return New(nil, engine, ws, discardLogger()) },
		"engine":    func() (*Scheduler, error) { return New(cfg, nil, ws, discardLogger()) },
		"workspace": func() (*Scheduler, error) { return New(cfg, engine, nil, discardLogger()) },
		"logger":    func() (*Scheduler, error) { return New(cfg, engine, ws, nil) },
		"run-state": func() (*Scheduler, error) {
			return New(cfg, engine, &noRunState{mockWorkspace: ws}, discardLogger())
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			_, err := build()
			require.Error(t, err)
		})
	}
}

type noRunState struct{ *mockWorkspace }

func (*noRunState) SchedulerState() schedulerstate.Store { return nil }

func TestStartBackground_NoOps(t *testing.T) {
	t.Parallel()
	for name, file := range map[string][]byte{
		"missing": nil,
		"invalid": []byte("not: valid: yaml: at all:"),
		"empty":   []byte("tasks: []\n"),
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			ws := newMockWorkspace(t)
			if file != nil {
				ws.files["project:"+ConfigFile] = file
			}
			StartBackground(t.Context(), ws, discardLogger())
		})
	}
}

// queueWorkspace is a mockWorkspace that also carries a job queue, matching
// what appbuild.Services provides in production.
type queueWorkspace struct {
	*mockWorkspace
	q jobs.Client
}

func (w *queueWorkspace) Jobs() jobs.Client { return w.q }

// TestStartBackground_UsesQueue pins the production wiring: StartBackground
// attaches the provider's queue and runs a first tick through it.
func TestStartBackground_UsesQueue(t *testing.T) {
	t.Parallel()
	q := newFakeQueue()
	ws := &queueWorkspace{mockWorkspace: newMockWorkspace(t), q: q}
	ws.files["project:"+ConfigFile] = []byte("tasks:\n  - name: t\n    script: t.lua\n    every: 1h\n")

	StartBackground(t.Context(), ws, discardLogger())

	require.Eventually(t, func() bool { return len(q.jobs()) == 1 }, 5*time.Second, 20*time.Millisecond,
		"StartBackground must attach the job queue and queue the first-ever run")
}

// TestNewWithQueue_RejectsProviderWithoutQueue pins the constructor entry
// points must use: a scheduler without a queue would fail every task.
func TestNewWithQueue_RejectsProviderWithoutQueue(t *testing.T) {
	t.Parallel()
	_, err := NewWithQueue(&Config{}, script.NewEngine(), newMockWorkspace(t), discardLogger())
	require.Error(t, err)
}

func TestNewWithQueue_AttachesQueue(t *testing.T) {
	t.Parallel()
	q := newFakeQueue()
	s, err := NewWithQueue(&Config{}, script.NewEngine(),
		&queueWorkspace{mockWorkspace: newMockWorkspace(t), q: q}, discardLogger())
	require.NoError(t, err)
	require.NotNil(t, s)
	require.Contains(t, q.handlers, TaskKind)
	require.Contains(t, q.handlers, ExpandKind)
	require.Contains(t, q.handlers, ChildKind)
}

func TestUseQueue_RejectsNil(t *testing.T) {
	t.Parallel()
	s, _, _ := newTestScheduler(t, newMockWorkspace(t), t0)
	require.Error(t, s.UseQueue(nil))
}

// TestRun_RealEnginePullsLuaWriteDeps runs the real engine path: the script
// is missing, so the run fails, but the deps must have been pulled once.
func TestRun_RealEnginePullsLuaWriteDeps(t *testing.T) {
	t.Parallel()
	ws := newMockWorkspace(t)
	s, q, _ := newTestScheduler(t, ws, t0, TaskConfig{Name: "t", Script: "missing.lua", Every: dailySchedule()})

	s.tick(context.Background())
	q.drain(t)

	require.Equal(t, 1, ws.luaDepsCalls)
	ts, _ := taskState(t, ws, "t")
	require.Equal(t, 1, ts.Failures)
}

// TestRun_RealEngineRunsScript runs a real Lua script end to end.
func TestRun_RealEngineRunsScript(t *testing.T) {
	t.Parallel()
	ws := newMockWorkspace(t)
	root := ws.paths.Root
	require.NoError(t, os.MkdirAll(filepath.Join(root, "scripts"), 0o750))
	require.NoError(t, os.WriteFile(filepath.Join(root, "scripts", "task.lua"), []byte("local x = 1\n"), 0o600))
	ws.luaDepsProvider = func() lua.WriteDeps {
		return lua.WriteDeps{ReadDeps: lua.ReadDeps{ProjectRoot: root}, EntityManager: stubMutator{}}
	}
	s, q, _ := newTestScheduler(t, ws, t0, TaskConfig{Name: "t", Script: "task.lua", Every: dailySchedule()})

	s.tick(context.Background())
	q.drain(t)

	ts, _ := taskState(t, ws, "t")
	require.Zero(t, ts.Failures)
	require.True(t, ts.LastRun.Equal(t0))
}

func TestRetryDelay(t *testing.T) {
	t.Parallel()
	tests := []struct {
		failures int
		want     time.Duration
	}{
		{0, baseRetryDelay},
		{-5, baseRetryDelay},
		{1, 5 * time.Minute},
		{2, 10 * time.Minute},
		{3, 20 * time.Minute},
		{4, 40 * time.Minute},
		{5, 80 * time.Minute},
		{6, maxRetryDelay},
		{7, maxRetryDelay},
		{50, maxRetryDelay},
		{1 << 40, maxRetryDelay},
	}
	for _, tc := range tests {
		require.Equal(t, tc.want, retryDelay(tc.failures), "retryDelay(%d)", tc.failures)
	}
}
