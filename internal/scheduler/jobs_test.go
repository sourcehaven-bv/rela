package scheduler

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/jobs"
)

// These tests are the point of porting the scheduler onto the job seam: they
// exercise the contract through a REAL queue rather than asserting the seam is
// well designed. A seam nothing consumes is an untested hypothesis.

// fakeQueue records what the scheduler submits and runs handlers inline, so a
// test can assert on the Job the scheduler built without waiting on a worker
// pool. The real backend is exercised separately in TestScheduler_ThroughQueue.
type fakeQueue struct {
	mu         sync.Mutex
	handlers   map[jobs.Kind]jobs.Handler
	got        []jobs.Job
	enqueErr   error
	runResult  error
	onComplete func(name string, err error)
}

func newFakeQueue() *fakeQueue {
	return &fakeQueue{handlers: make(map[jobs.Kind]jobs.Handler)}
}

func (f *fakeQueue) Register(kind jobs.Kind, h jobs.Handler) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.handlers[kind] = h
	return nil
}

func (f *fakeQueue) Enqueue(_ context.Context, job jobs.Job) error {
	f.mu.Lock()
	f.got = append(f.got, job)
	err := f.enqueErr
	result := f.runResult
	f.mu.Unlock()

	if err != nil {
		return err
	}

	// Report completion from another goroutine, as a real worker would. The
	// submitter is blocked on its completion channel, so reporting inline would
	// deadlock — and would hide that ordering requirement rather than exercise
	// it.
	//
	// The scheduler's real handler is deliberately NOT invoked: these tests are
	// about the Job the scheduler builds and the state it records, not about
	// executing Lua. TestScheduler_ThroughRealQueue covers the handler.
	name, _ := job.Payload[payloadTaskName].(string)
	go f.report(name, result)
	return nil
}

// report is overridable so a test can drive completion itself.
func (f *fakeQueue) report(name string, err error) {
	if f.onComplete != nil {
		f.onComplete(name, err)
	}
}

func (f *fakeQueue) jobs() []jobs.Job {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]jobs.Job(nil), f.got...)
}

// TestUseQueue_RegistersHandler pins that the scheduler binds its own kind.
// A subsystem that owns a kind should be the thing that registers it, rather
// than leaving the wiring site to know the scheduler's internals.
func TestUseQueue_RegistersHandler(t *testing.T) {
	t.Parallel()

	q := newFakeQueue()
	s := &Scheduler{logger: discardLogger()}
	require.NoError(t, s.UseQueue(q))

	q.mu.Lock()
	defer q.mu.Unlock()
	require.Contains(t, q.handlers, TaskKind, "scheduler must register its own task kind")
}

// TestUseQueue_RejectsNil pins the constructor contract: a nil queue is a
// wiring bug, and silently falling back to inline execution would make a
// scheduler that stopped using the queue indistinguishable from one that never
// had it.
func TestUseQueue_RejectsNil(t *testing.T) {
	t.Parallel()

	s := &Scheduler{logger: discardLogger()}
	require.Error(t, s.UseQueue(nil))
}

// TestEnqueuedJob_UsesIdempotencyKey is the load-bearing assertion for how a
// schedule reaches the queue.
//
// The key is what says "one run of this task at a time is enough". An earlier
// version used a cadence-derived deadline instead, which made scheduled jobs
// vanish under load — precisely when the work matters most. The key degrades
// the other way: the run happens late rather than not at all.
func TestEnqueuedJob_UsesIdempotencyKey(t *testing.T) {
	t.Parallel()

	q := newFakeQueue()
	now := time.Now()
	s := newQueuedScheduler(t, q, now, TaskConfig{
		Name: "daily-report", Script: "report.lua", Every: intervalSchedule(time.Hour),
	})

	require.NoError(t, s.enqueueTask(context.Background(), s.config.Tasks[0]))

	submitted := q.jobs()
	require.Len(t, submitted, 1)
	require.Equal(t, "daily-report", submitted[0].IdempotencyKey,
		"the task name is the dedupe identity, so a pending run suppresses the next")
	require.True(t, submitted[0].Deadline.IsZero(),
		"a scheduled job must carry NO deadline: a deadline drops the job under load")
}

// TestEnqueuedJob_KeyIsPerTask pins that two tasks never suppress each other.
// A shared key would mean one slow task silently blocked every other one.
func TestEnqueuedJob_KeyIsPerTask(t *testing.T) {
	t.Parallel()

	q := newFakeQueue()
	now := time.Now()
	s := newQueuedScheduler(t, q, now,
		TaskConfig{Name: "a", Script: "a.lua", Every: intervalSchedule(time.Hour)},
		TaskConfig{Name: "b", Script: "b.lua", Every: intervalSchedule(time.Hour)},
	)

	require.NoError(t, s.enqueueTask(context.Background(), s.config.Tasks[0]))
	require.NoError(t, s.enqueueTask(context.Background(), s.config.Tasks[1]))

	submitted := q.jobs()
	require.Len(t, submitted, 2)
	require.NotEqual(t, submitted[0].IdempotencyKey, submitted[1].IdempotencyKey)
}

// TestEnqueuedJob_UsesRetryNever pins that the scheduler keeps ownership of
// retrying.
//
// Its ladder (5m→2h, suppressing the normal cadence) encodes hard-won behavior
// including the BUG-ZKK2UL clock-jump guard. Letting the queue retry as well
// would multiply the two policies rather than compose them — a failing task
// would be attempted queue-retries × ladder-rungs times.
func TestEnqueuedJob_UsesRetryNever(t *testing.T) {
	t.Parallel()

	q := newFakeQueue()
	now := time.Now()
	s := newQueuedScheduler(t, q, now, TaskConfig{
		Name: "task", Script: "s.lua", Every: intervalSchedule(time.Hour),
	})

	require.NoError(t, s.enqueueTask(context.Background(), s.config.Tasks[0]))

	submitted := q.jobs()
	require.Len(t, submitted, 1)
	require.Equal(t, jobs.RetryNever, submitted[0].Retry,
		"the scheduler owns retrying; the queue must not retry on top of the ladder")
}

// TestEnqueuedJob_CarriesTaskIdentity pins that run_as survives the hop onto a
// worker.
//
// This is a security-relevant property, not a convenience: reads inside a task
// are ACL-bound to the principal on the handler's context (DEC-O59WM4), and a
// worker's context is NOT the submitter's. Losing run_as here would silently run
// every task as the default scheduler identity.
func TestEnqueuedJob_CarriesTaskIdentity(t *testing.T) {
	t.Parallel()

	q := newFakeQueue()
	now := time.Now()
	s := newQueuedScheduler(t, q, now, TaskConfig{
		Name: "task", Script: "s.lua", RunAs: "system:reporting",
		Every: intervalSchedule(time.Hour),
	})

	require.NoError(t, s.enqueueTask(context.Background(), s.config.Tasks[0]))

	submitted := q.jobs()
	require.Len(t, submitted, 1)
	require.Equal(t, "task", submitted[0].Payload[payloadTaskName])
	require.Equal(t, "s.lua", submitted[0].Payload[payloadScript])
	require.Equal(t, "system:reporting", submitted[0].Payload[payloadRunAs])
}

// TestEnqueueTask_SkipsWhenInFlight pins non-overlap.
//
// The single-threaded loop gave this for free; moving execution onto a worker
// pool means it has to be stated, or a slow task on a short cadence queues
// behind itself and piles up. The skip must be distinguishable from a failure —
// a slow task has not gone wrong.
func TestEnqueueTask_SkipsWhenInFlight(t *testing.T) {
	t.Parallel()

	q := newFakeQueue()
	now := time.Now()
	s := newQueuedScheduler(t, q, now, TaskConfig{
		Name: "task", Script: "s.lua", Every: intervalSchedule(time.Hour),
	})

	// Simulate a run already in flight.
	_, err := s.claimInFlight("task")
	require.NoError(t, err)

	err = s.enqueueTask(context.Background(), s.config.Tasks[0])
	require.ErrorIs(t, err, errTaskInFlight)
	require.Empty(t, q.jobs(), "a task already running must not be submitted again")
}

// TestDoExecuteTask_InFlightSkipIsNotAFailure pins that a skip does not advance
// the retry ladder. Treating "still running" as a failure would suppress a
// healthy task's normal cadence and escalate it toward the ERROR threshold.
func TestDoExecuteTask_InFlightSkipIsNotAFailure(t *testing.T) {
	t.Parallel()

	q := newFakeQueue()
	now := time.Now()
	s := newQueuedScheduler(t, q, now, TaskConfig{
		Name: "task", Script: "s.lua", Every: intervalSchedule(time.Hour),
	})

	_, err := s.claimInFlight("task")
	require.NoError(t, err)

	s.doExecuteTask(context.Background(), s.config.Tasks[0])

	require.Empty(t, s.state.Failures, "a skipped task must not advance the retry ladder")
	require.Empty(t, s.state.NextRetry, "a skipped task must not schedule a retry")
}

// TestEnqueueTask_ReportsEnqueueFailure pins that a queue that refuses work
// surfaces as a task failure rather than a silent no-op — otherwise the task
// would look like it ran and its last-run stamp would advance.
func TestEnqueueTask_ReportsEnqueueFailure(t *testing.T) {
	t.Parallel()

	q := newFakeQueue()
	q.enqueErr = errors.New("queue is closed")
	now := time.Now()
	s := newQueuedScheduler(t, q, now, TaskConfig{
		Name: "task", Script: "s.lua", Every: intervalSchedule(time.Hour),
	})

	err := s.enqueueTask(context.Background(), s.config.Tasks[0])
	require.Error(t, err)
	require.NotErrorIs(t, err, errTaskInFlight)
}

// TestRunTaskJob_RejectsEmptyScript pins that an unrunnable job fails loudly
// rather than reporting success. A durable backend can hand a worker a job
// enqueued by an older build.
func TestRunTaskJob_RejectsEmptyScript(t *testing.T) {
	t.Parallel()

	s := &Scheduler{logger: discardLogger()}
	err := s.runTaskJob(context.Background(), jobs.Job{
		Kind:    TaskKind,
		Payload: map[string]any{payloadTaskName: "task"},
	})
	require.Error(t, err)
}

// TestReportInFlight_NoWaiterIsHarmless pins that a result with nobody waiting
// is dropped rather than blocking a worker. Reachable when the submitter was
// cancelled, or when a durable backend redelivers after a restart.
func TestReportInFlight_NoWaiterIsHarmless(t *testing.T) {
	t.Parallel()

	s := &Scheduler{logger: discardLogger()}
	require.NotPanics(t, func() { s.reportInFlight("nobody", nil) })
}

// newQueuedScheduler builds a scheduler wired to q with a fixed clock.
func newQueuedScheduler(
	t *testing.T, q jobs.Client, now time.Time, tasks ...TaskConfig,
) *Scheduler {
	t.Helper()

	cfg := &Config{Tasks: tasks}
	ws := newMockWorkspace(t)
	s := &Scheduler{
		config: cfg,
		ws:     ws,
		state:  newState(),
		logger: discardLogger(),
		now:    func() time.Time { return now },
	}
	require.NoError(t, s.UseQueue(q))

	// Completion goes back through the scheduler's own reporting path, which is
	// what unblocks enqueueTask. Wiring it here rather than inside fakeQueue
	// keeps the fake ignorant of the Scheduler type.
	if fq, ok := q.(*fakeQueue); ok {
		fq.onComplete = s.reportInFlight
	}
	return s
}

// TestScheduler_ThroughRealQueue is the reason for the port.
//
// Everything above uses a fake, which proves the scheduler builds the right Job
// but not that the seam does what it claims. This one runs a real memory-backed
// queue end to end: the scheduler registers its handler, submits a task, a
// worker picks it up on another goroutine, the script executes, and the
// scheduler records the outcome before returning.
//
// If the contract is wrong — a job that never runs, a result that never gets
// back, a deadline that expires the job before a worker sees it — this is where
// it shows, and nothing else in the tree would notice.
func TestScheduler_ThroughRealQueue(t *testing.T) {
	q, err := jobs.NewMemoryQueue(context.Background(), discardLogger())
	require.NoError(t, err)
	t.Cleanup(func() { _ = q.Close(context.Background()) })

	var (
		mu     sync.Mutex
		ranAs  string
		ranFor string
	)

	// Stand in for the Lua engine. The handler must still report completion
	// through the scheduler, exactly as runTaskJob does — a handler that skips
	// that leaves the submitter blocked forever, which is precisely what this
	// test caught the first time it was written.
	require.NoError(t, q.Start(context.Background()))

	now := time.Now()
	s := &Scheduler{
		config: &Config{Tasks: []TaskConfig{{
			Name: "nightly", Script: "reports.lua", RunAs: "system:reporting",
			Every: intervalSchedule(time.Hour),
		}}},
		ws:     newMockWorkspace(t),
		state:  newState(),
		logger: discardLogger(),
		now:    func() time.Time { return now },
	}
	// Wired exactly as production wires it, with the engine call replaced.
	s.engineRunner = func(_ context.Context, task TaskConfig) error {
		mu.Lock()
		ranFor, ranAs = task.Script, task.RunAs
		mu.Unlock()
		return nil
	}
	require.NoError(t, s.UseQueue(q))

	require.NoError(t, s.enqueueTask(context.Background(), s.config.Tasks[0]))

	mu.Lock()
	defer mu.Unlock()
	require.Equal(t, "reports.lua", ranFor, "the worker must receive the task's script")
	require.Equal(t, "system:reporting", ranAs, "run_as must survive the hop to a worker")
}

// TestScheduler_ThroughRealQueue_RecordsFailure pins that a failing script
// still advances the retry ladder when execution happens on a worker.
//
// This is the property most at risk from the port: the ladder is what closes
// BUG-ZKK2UL, and it only works if the outcome of an asynchronous run makes it
// back to the scheduler before the next tick evaluates the task.
func TestScheduler_ThroughRealQueue_RecordsFailure(t *testing.T) {
	q, err := jobs.NewMemoryQueue(context.Background(), discardLogger())
	require.NoError(t, err)
	t.Cleanup(func() { _ = q.Close(context.Background()) })

	require.NoError(t, q.Start(context.Background()))

	now := time.Now()
	s := &Scheduler{
		config: &Config{Tasks: []TaskConfig{{
			Name: "flaky", Script: "flaky.lua", Every: intervalSchedule(time.Minute),
		}}},
		ws:     newMockWorkspace(t),
		state:  newState(),
		logger: discardLogger(),
		now:    func() time.Time { return now },
	}
	s.engineRunner = func(context.Context, TaskConfig) error {
		return errors.New("script blew up")
	}
	require.NoError(t, s.UseQueue(q))

	s.doExecuteTask(context.Background(), s.config.Tasks[0])

	require.Equal(t, 1, s.state.Failures["flaky"],
		"a failure on a worker must advance the retry ladder")
	require.False(t, s.state.NextRetry["flaky"].IsZero(),
		"a failed task must have a pending retry, or it becomes perpetually due (BUG-ZKK2UL)")
	require.NotContains(t, s.state.Tasks, "flaky",
		"a failed run must not stamp a last-successful-run time")
}

// TestFirstEverRun_DoesNotHang is a regression test for a bug the port itself
// surfaced, and the clearest argument for porting a consumer onto a new seam
// rather than trusting its design.
//
// A task with no prior run has a zero last-run stamp. Feeding that to NextRun
// yields a deadline in year 1 — already long past — so the queue correctly
// dropped the job without running it, nothing ever reported completion, and
// enqueueTask blocked forever. A first-ever task would have hung the scheduler
// permanently, and every unit test still passed.
func TestFirstEverRun_DoesNotHang(t *testing.T) {
	t.Parallel()

	q, err := jobs.NewMemoryQueue(context.Background(), discardLogger())
	require.NoError(t, err)
	t.Cleanup(func() { _ = q.Close(context.Background()) })
	require.NoError(t, q.Start(context.Background()))

	ran := make(chan struct{}, 1)
	s := &Scheduler{
		config: &Config{Tasks: []TaskConfig{{
			Name: "brand-new", Script: "new.lua", Every: intervalSchedule(time.Minute),
		}}},
		ws:     newMockWorkspace(t),
		state:  newState(), // no recorded run: this is the first-ever case
		logger: discardLogger(),
		now:    time.Now,
	}
	s.engineRunner = func(context.Context, TaskConfig) error {
		ran <- struct{}{}
		return nil
	}
	require.NoError(t, s.UseQueue(q))

	done := make(chan struct{})
	go func() {
		defer close(done)
		s.doExecuteTask(context.Background(), s.config.Tasks[0])
	}()

	select {
	case <-done:
	case <-time.After(20 * time.Second):
		t.Fatal("first-ever run hung: the computed deadline was already past, " +
			"so the job was dropped and completion was never reported")
	}

	select {
	case <-ran:
	default:
		t.Fatal("first-ever task never executed")
	}
	require.Contains(t, s.state.Tasks, "brand-new",
		"a successful first run must stamp its last-run time")
}

// TestStartBackground_UsesQueueWhenAvailable pins the production wiring.
//
// The queue is attached through an optional interface on the workspace
// provider, so a mistake here fails silently — the scheduler would keep working
// inline and nobody would notice the queue was never attached. This asserts the
// handler actually gets registered.
func TestStartBackground_UsesQueueWhenAvailable(t *testing.T) {
	t.Parallel()

	q := newFakeQueue()
	ws := &queueWorkspace{mockWorkspace: newMockWorkspace(t), q: q}

	// schedules.yaml must exist for StartBackground to do anything.
	ws.cacheFiles["project:"+ConfigFile] = []byte("tasks:\n  - name: t\n    script: t.lua\n    every: 1h\n")

	StartBackground(t.Context(), ws, discardLogger())

	require.Eventually(t, func() bool {
		q.mu.Lock()
		defer q.mu.Unlock()
		_, ok := q.handlers[TaskKind]
		return ok
	}, 5*time.Second, 20*time.Millisecond,
		"StartBackground must attach the job queue when the provider carries one")
}

// queueWorkspace is a mockWorkspace that also carries a job queue, matching
// what appbuild.Services provides in production.
type queueWorkspace struct {
	*mockWorkspace
	q *fakeQueue
}

func (w *queueWorkspace) Jobs() jobs.Client { return w.q }

// TestOverloadedQueue_RunsLateRatherThanDropping is the regression test for the
// failure mode that motivated using an idempotency key instead of a deadline.
//
// With a cadence-derived deadline, a short-interval task queued behind a busy
// pool had its deadline lapse while it waited. The queue then correctly dropped
// it, nothing reported completion, and the scheduler blocked forever on a
// result that could not arrive — one overload episode stopped ALL scheduling
// until restart, silently, with the process idle rather than loaded.
//
// The right behavior under load is late, not never.
func TestOverloadedQueue_RunsLateRatherThanDropping(t *testing.T) {
	q, err := jobs.NewMemoryQueue(context.Background(), discardLogger())
	require.NoError(t, err)
	t.Cleanup(func() { _ = q.Close(context.Background()) })

	// Saturate every worker with jobs that will not finish until released.
	release := make(chan struct{})
	slow := jobs.NewKind("test", "slow")
	require.NoError(t, q.Register(slow, func(ctx context.Context, _ jobs.Job) error {
		select {
		case <-release:
		case <-ctx.Done():
		}
		return nil
	}))
	require.NoError(t, q.Start(context.Background()))
	for range 8 {
		require.NoError(t, q.Enqueue(context.Background(), jobs.Job{Kind: slow}))
	}
	time.Sleep(300 * time.Millisecond)

	ran := make(chan struct{}, 1)
	s := &Scheduler{
		config: &Config{Tasks: []TaskConfig{{
			// A one-second cadence: the shortest realistic interval, and the
			// case a deadline handled worst.
			Name: "frequent", Script: "f.lua", Every: intervalSchedule(time.Second),
		}}},
		ws:     newMockWorkspace(t),
		state:  newState(),
		logger: discardLogger(),
		now:    time.Now,
	}
	s.engineRunner = func(context.Context, TaskConfig) error {
		ran <- struct{}{}
		return nil
	}
	require.NoError(t, s.UseQueue(q))

	done := make(chan struct{})
	go func() {
		defer close(done)
		s.doExecuteTask(context.Background(), s.config.Tasks[0])
	}()

	// The task waits behind the backlog for longer than its own interval —
	// exactly the window in which a deadline would have expired it.
	time.Sleep(1500 * time.Millisecond)
	close(release)

	select {
	case <-done:
	case <-time.After(20 * time.Second):
		t.Fatal("scheduler hung waiting on a job the queue dropped under load")
	}

	select {
	case <-ran:
	default:
		t.Fatal("task was dropped under load; it should have run late instead")
	}
	require.Contains(t, s.state.Tasks, "frequent",
		"a run that completed late is still a successful run")
}

// attachTestQueue wires a real memory-backed queue onto a hand-built
// Scheduler.
//
// Needed because execution now goes exclusively through the queue: a scheduler
// without one cannot run anything. The tests that exercise the real execution
// path therefore need a real queue, and using the actual backend rather than a
// double means they keep testing what production does.
func attachTestQueue(t *testing.T, s *Scheduler) {
	t.Helper()

	q, err := jobs.NewMemoryQueue(context.Background(), discardLogger())
	require.NoError(t, err)
	t.Cleanup(func() { _ = q.Close(context.Background()) })
	require.NoError(t, s.UseQueue(q))
	require.NoError(t, q.Start(context.Background()))
}

// TestNewWithQueue_RejectsProviderWithoutQueue pins the constructor that entry
// points must use.
//
// This is the regression test for a real break: the `rela scheduler` command
// called New directly, never attached a queue, and after inline execution was
// removed it started cleanly and then failed EVERY task with "no job queue
// configured". No unit test caught it, because they all call UseQueue
// themselves — none exercised a wiring site. A local demo found it.
func TestNewWithQueue_RejectsProviderWithoutQueue(t *testing.T) {
	t.Parallel()

	// A provider with no Jobs() method — what every pre-queue caller looks like.
	_, err := NewWithQueue(&Config{}, nil, newMockWorkspace(t), discardLogger())
	require.Error(t, err,
		"a provider without a job queue must be refused, not silently accepted")
}

// TestNewWithQueue_AttachesQueue is the positive half: a provider that carries
// a queue produces a scheduler with the handler registered and ready to run.
func TestNewWithQueue_AttachesQueue(t *testing.T) {
	t.Parallel()

	q := newFakeQueue()
	ws := &queueWorkspace{mockWorkspace: newMockWorkspace(t), q: q}

	s, err := NewWithQueue(&Config{}, nil, ws, discardLogger())
	require.NoError(t, err)
	require.NotNil(t, s)

	q.mu.Lock()
	defer q.mu.Unlock()
	require.Contains(t, q.handlers, TaskKind)
}
