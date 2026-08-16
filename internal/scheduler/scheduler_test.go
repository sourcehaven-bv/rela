package scheduler

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"os"
	"path/filepath"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/config"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/lua"
	"github.com/Sourcehaven-BV/rela/internal/project"
	"github.com/Sourcehaven-BV/rela/internal/script"
	"github.com/Sourcehaven-BV/rela/internal/state"
)

// --- test helpers ---

type mockWorkspace struct {
	mu              sync.Mutex
	cacheFiles      map[string][]byte
	paths           *project.Context
	luaDepsCalls    int
	luaDepsProvider func() lua.WriteDeps
}

func newMockWorkspace(t *testing.T) *mockWorkspace {
	t.Helper()
	return &mockWorkspace{
		cacheFiles: make(map[string][]byte),
		paths:      &project.Context{Root: t.TempDir()},
	}
}

func (m *mockWorkspace) Paths() *project.Context { return m.paths }

func (m *mockWorkspace) Config() config.Loader { return &mockConfig{m: m} }

func (m *mockWorkspace) State() state.KV { return &mockState{m: m} }

func (m *mockWorkspace) ScheduledLuaWriteDeps() lua.WriteDeps {
	m.mu.Lock()
	m.luaDepsCalls++
	provider := m.luaDepsProvider
	m.mu.Unlock()
	if provider != nil {
		return provider()
	}
	return lua.WriteDeps{}
}

type mockConfig struct{ m *mockWorkspace }

func (c *mockConfig) Load(_ context.Context, name string) ([]byte, error) {
	c.m.mu.Lock()
	defer c.m.mu.Unlock()
	data, ok := c.m.cacheFiles["project:"+name]
	if !ok {
		return nil, &notFoundError{name}
	}
	return data, nil
}

type mockState struct{ m *mockWorkspace }

func (s *mockState) Get(_ context.Context, key string) ([]byte, error) {
	s.m.mu.Lock()
	defer s.m.mu.Unlock()
	data, ok := s.m.cacheFiles[key]
	if !ok {
		return nil, &notFoundError{key}
	}
	return data, nil
}

func (s *mockState) Put(_ context.Context, key string, data []byte) error {
	s.m.mu.Lock()
	defer s.m.mu.Unlock()
	s.m.cacheFiles[key] = append([]byte(nil), data...)
	return nil
}

func (s *mockState) Delete(_ context.Context, key string) error {
	s.m.mu.Lock()
	defer s.m.mu.Unlock()
	delete(s.m.cacheFiles, key)
	return nil
}

type notFoundError struct{ name string }

func (e *notFoundError) Error() string { return "not found: " + e.name }

// errBoom is the injected failure used by the retry-ladder tests.
var errBoom = errors.New("boom")

// stubMutator satisfies lua.Mutator so a writer runtime can be constructed.
// The real-path tests run scripts that never mutate, so every method returning
// an error is correct: if one is ever called, the test should fail loudly
// rather than silently exercise a no-op write.
type stubMutator struct{}

var errStubMutator = errors.New("stubMutator: unexpected write from a test script")

func (stubMutator) CreateEntity(
	context.Context, *entity.Entity, entity.CreateOptions,
) (*entity.CreateResult, error) {
	return nil, errStubMutator
}

func (stubMutator) UpdateEntity(context.Context, *entity.Entity) (*entity.UpdateResult, error) {
	return nil, errStubMutator
}

func (stubMutator) PatchEntity(context.Context, string, entity.Patch) (*entity.UpdateResult, error) {
	return nil, errStubMutator
}

func (stubMutator) DeleteEntity(context.Context, string, bool) (*entity.DeleteResult, error) {
	return nil, errStubMutator
}

func (stubMutator) CreateRelation(
	context.Context, string, string, string, entity.RelationOptions,
) (*entity.Relation, error) {
	return nil, errStubMutator
}

func (stubMutator) DeleteRelation(context.Context, string, string, string) error {
	return errStubMutator
}

type mockTracker struct {
	mu    sync.Mutex
	calls []string
	times []time.Time
}

func (m *mockTracker) record(path string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.calls = append(m.calls, path)
}

// recordAt records a call together with the time the run started, so tests can
// assert WHEN a task fired rather than inferring it from the clock afterwards.
func (m *mockTracker) recordAt(path string, at time.Time) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.calls = append(m.calls, path)
	m.times = append(m.times, at)
}

func (m *mockTracker) getTimes() []time.Time {
	m.mu.Lock()
	defer m.mu.Unlock()
	return slices.Clone(m.times)
}

func (m *mockTracker) getCalls() []string {
	m.mu.Lock()
	defer m.mu.Unlock()
	result := make([]string, len(m.calls))
	copy(result, m.calls)
	return result
}

func discardLogger() *slog.Logger {
	return slog.New(slog.DiscardHandler)
}

func newTestScheduler(
	t *testing.T,
	cfg *Config,
	now time.Time,
) (*Scheduler, *mockWorkspace, *mockTracker) {
	t.Helper()
	ws := newMockWorkspace(t)
	tracker := &mockTracker{}
	s := &Scheduler{
		config: cfg,
		ws:     ws,
		state:  newState(),
		logger: discardLogger(),
		now:    func() time.Time { return now },
	}
	s.executeTaskFunc = func(ctx context.Context, task TaskConfig) {
		tracker.record(task.Script)
		s.state.Tasks[task.Name] = s.now()
		s.saveState(ctx)
	}
	return s, ws, tracker
}

func dailySchedule() Schedule {
	return Schedule{kind: dayKind, set: true}
}

func intervalSchedule(d time.Duration) Schedule {
	return Schedule{kind: intervalKind, interval: d, set: true}
}

// --- tests ---

func TestRunDueTasks_firstEver(t *testing.T) {
	t.Parallel()

	cfg := &Config{
		Tasks: []TaskConfig{
			{Name: "check", Script: "check.lua", Every: dailySchedule()},
		},
	}
	now := time.Date(2026, 4, 10, 14, 0, 0, 0, time.UTC)
	s, _, tracker := newTestScheduler(t, cfg, now)

	s.runDueTasks(context.Background())

	calls := tracker.getCalls()
	if len(calls) != 1 || calls[0] != "check.lua" {
		t.Errorf("expected 1 call to check.lua, got %v", calls)
	}
}

func TestRunDueTasks_missedDay(t *testing.T) {
	t.Parallel()

	cfg := &Config{
		Tasks: []TaskConfig{
			{Name: "daily", Script: "daily.lua", Every: dailySchedule()},
		},
	}
	now := time.Date(2026, 4, 10, 14, 0, 0, 0, time.Local)
	lastRun := time.Date(2026, 4, 9, 9, 0, 0, 0, time.Local) // yesterday

	s, _, tracker := newTestScheduler(t, cfg, now)
	s.state.Tasks["daily"] = lastRun

	s.runDueTasks(context.Background())

	calls := tracker.getCalls()
	if len(calls) != 1 || calls[0] != "daily.lua" {
		t.Errorf("expected 1 missed run call, got %v", calls)
	}
}

func TestRunDueTasks_notDue(t *testing.T) {
	t.Parallel()

	cfg := &Config{
		Tasks: []TaskConfig{
			{Name: "daily", Script: "daily.lua", Every: dailySchedule()},
		},
	}
	now := time.Date(2026, 4, 10, 9, 30, 0, 0, time.Local)
	lastRun := time.Date(2026, 4, 10, 9, 5, 0, 0, time.Local) // ran today

	s, _, tracker := newTestScheduler(t, cfg, now)
	s.state.Tasks["daily"] = lastRun

	s.runDueTasks(context.Background())

	if calls := tracker.getCalls(); len(calls) != 0 {
		t.Errorf("expected no calls, got %v", calls)
	}
}

func TestRunDueTasks_intervalDue(t *testing.T) {
	t.Parallel()

	cfg := &Config{
		Tasks: []TaskConfig{
			{Name: "check", Script: "check.lua", Every: intervalSchedule(30 * time.Minute)},
		},
	}
	now := time.Date(2026, 4, 10, 10, 0, 0, 0, time.UTC)
	lastRun := time.Date(2026, 4, 10, 9, 25, 0, 0, time.UTC) // 35min ago

	s, _, tracker := newTestScheduler(t, cfg, now)
	s.state.Tasks["check"] = lastRun

	s.runDueTasks(context.Background())

	calls := tracker.getCalls()
	if len(calls) != 1 {
		t.Errorf("expected 1 call, got %v", calls)
	}
}

func TestRunDueTasks_intervalNotDue(t *testing.T) {
	t.Parallel()

	cfg := &Config{
		Tasks: []TaskConfig{
			{Name: "check", Script: "check.lua", Every: intervalSchedule(30 * time.Minute)},
		},
	}
	now := time.Date(2026, 4, 10, 9, 20, 0, 0, time.UTC)
	lastRun := time.Date(2026, 4, 10, 9, 0, 0, 0, time.UTC) // 20min ago

	s, _, tracker := newTestScheduler(t, cfg, now)
	s.state.Tasks["check"] = lastRun

	s.runDueTasks(context.Background())

	if calls := tracker.getCalls(); len(calls) != 0 {
		t.Errorf("expected no calls, got %v", calls)
	}
}

func TestScheduler_statePersistedAfterRun(t *testing.T) {
	t.Parallel()

	cfg := &Config{
		Tasks: []TaskConfig{
			{Name: "test", Script: "test.lua", Every: dailySchedule()},
		},
	}
	now := time.Date(2026, 4, 10, 14, 0, 0, 0, time.UTC)
	s, ws, _ := newTestScheduler(t, cfg, now)

	s.runDueTasks(context.Background())

	data, err := ws.State().Get(context.Background(), stateFile)
	if err != nil {
		t.Fatalf("state file not written: %v", err)
	}
	var saved State
	if err := json.Unmarshal(data, &saved); err != nil {
		t.Fatalf("invalid state JSON: %v", err)
	}
	if _, ok := saved.Tasks["test"]; !ok {
		t.Error("expected 'test' task in saved state")
	}
}

func TestScheduler_loadState_noFile(t *testing.T) {
	t.Parallel()

	ws := newMockWorkspace(t)
	s := &Scheduler{ws: ws, logger: discardLogger()}
	s.loadState(context.Background())

	if s.state == nil || s.state.Tasks == nil {
		t.Fatal("expected initialized state")
	}
}

func TestScheduler_loadState_existing(t *testing.T) {
	t.Parallel()

	ws := newMockWorkspace(t)
	ts := time.Date(2026, 4, 10, 9, 0, 0, 0, time.UTC)
	stateData, _ := json.Marshal(State{Tasks: map[string]time.Time{"daily": ts}})
	ws.cacheFiles[stateFile] = stateData

	s := &Scheduler{ws: ws, logger: discardLogger()}
	s.loadState(context.Background())

	if got := s.state.Tasks["daily"]; !got.Equal(ts) {
		t.Errorf("loaded state: daily = %v, want %v", got, ts)
	}
}

func TestScheduler_Run_emptyConfig(t *testing.T) {
	t.Parallel()

	ws := newMockWorkspace(t)
	s := &Scheduler{
		config: &Config{Tasks: nil},
		ws:     ws,
		logger: discardLogger(),
		now:    time.Now,
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	if err := s.Run(ctx); err != nil {
		t.Errorf("Run with empty config should return nil, got %v", err)
	}
}

func TestRunDueTasks_cancelledContext(t *testing.T) {
	t.Parallel()

	cfg := &Config{
		Tasks: []TaskConfig{
			{Name: "a", Script: "a.lua", Every: dailySchedule()},
			{Name: "b", Script: "b.lua", Every: dailySchedule()},
		},
	}
	now := time.Date(2026, 4, 10, 14, 0, 0, 0, time.UTC)
	s, _, tracker := newTestScheduler(t, cfg, now)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	s.runDueTasks(ctx)

	if calls := tracker.getCalls(); len(calls) != 0 {
		t.Errorf("expected no calls with cancelled context, got %v", calls)
	}
}

func TestRunDueTasks_sequential(t *testing.T) {
	t.Parallel()

	cfg := &Config{
		Tasks: []TaskConfig{
			{Name: "a", Script: "a.lua", Every: dailySchedule()},
			{Name: "b", Script: "b.lua", Every: dailySchedule()},
			{Name: "c", Script: "c.lua", Every: dailySchedule()},
		},
	}
	now := time.Date(2026, 4, 10, 14, 0, 0, 0, time.UTC)
	s, _, tracker := newTestScheduler(t, cfg, now)

	s.runDueTasks(context.Background())

	calls := tracker.getCalls()
	if len(calls) != 3 {
		t.Fatalf("expected 3 calls, got %d", len(calls))
	}
	// Verify execution order matches config order.
	if calls[0] != "a.lua" || calls[1] != "b.lua" || calls[2] != "c.lua" {
		t.Errorf("expected [a.lua b.lua c.lua], got %v", calls)
	}
}

func TestStartBackground_NoConfig(t *testing.T) {
	// When schedules.yaml is missing, StartBackground should silently
	// no-op without starting a goroutine.
	ws := newMockWorkspace(t)

	// ws.Config().Load returns notFoundError for missing file.
	ctx := t.Context()

	// Should not panic, should not log errors.
	StartBackground(ctx, ws, discardLogger())
}

func TestStartBackground_InvalidConfig(t *testing.T) {
	ws := newMockWorkspace(t)
	ws.cacheFiles["project:"+ConfigFile] = []byte("not: valid: yaml: at all:")

	ctx := t.Context()

	// Should log error and return without starting a goroutine.
	StartBackground(ctx, ws, discardLogger())
}

func TestStartBackground_EmptyTasks(t *testing.T) {
	ws := newMockWorkspace(t)
	ws.cacheFiles["project:"+ConfigFile] = []byte("tasks: []\n")

	ctx := t.Context()

	StartBackground(ctx, ws, discardLogger())
}

func TestNew(t *testing.T) {
	cfg := &Config{Tasks: []TaskConfig{{Name: "t", Script: "t.lua"}}}
	ws := newMockWorkspace(t)

	s := New(cfg, nil, ws, discardLogger())
	if s == nil {
		t.Fatal("New returned nil")
	}
	if s.config != cfg {
		t.Error("config not wired")
	}
	if s.ws != ws {
		t.Error("ws not wired")
	}
}

// TestDoExecuteTask_PullsLuaWriteDeps exercises the real doExecuteTask path
// (no executeTaskFunc override) to verify the scheduler pulls lua.WriteDeps
// from its WorkspaceProvider before invoking the engine. The Lua script is
// intentionally absent, so ExecuteFile returns an error — what we're
// verifying is that LuaWriteDeps() was called regardless.
func TestDoExecuteTask_PullsLuaWriteDeps(t *testing.T) {
	t.Parallel()

	cfg := &Config{
		Tasks: []TaskConfig{
			{Name: "t", Script: "missing.lua", Every: dailySchedule()},
		},
	}
	ws := newMockWorkspace(t)
	s := New(cfg, script.NewEngine(), ws, discardLogger())
	s.now = func() time.Time { return time.Date(2026, 4, 10, 14, 0, 0, 0, time.UTC) }
	s.state = newState()

	s.doExecuteTask(context.Background(), cfg.Tasks[0])

	ws.mu.Lock()
	calls := ws.luaDepsCalls
	ws.mu.Unlock()
	if calls != 1 {
		t.Errorf("expected LuaWriteDeps called once, got %d", calls)
	}
}

// --- BUG-ZKK2UL: failed-task retry ladder ---
//
// Pins AM-scheduler-failed-task-not-rescheduled-immediately. Before the fix a
// failing task left no state behind, so it stayed perpetually due and ran on
// every 60s tick — a `day` task executed ~1440x/day.

// newRetryTestScheduler builds a scheduler with a MUTABLE clock and a
// failure-injecting executeTaskFunc. The shared newTestScheduler helper is
// unusable here: its clock is fixed and its override always simulates success,
// so no failure scheduling could be observed through it.
//
// The override delegates to the real recordFailure/success bookkeeping so the
// tests exercise the actual state transitions rather than a reimplementation.
func newRetryTestScheduler(
	t *testing.T,
	cfg *Config,
	clock *time.Time,
	fail func() bool,
) (*Scheduler, *mockTracker) {
	t.Helper()
	ws := newMockWorkspace(t)
	tracker := &mockTracker{}
	s := &Scheduler{
		config: cfg,
		ws:     ws,
		state:  newState(),
		logger: discardLogger(),
		now:    func() time.Time { return *clock },
	}
	s.executeTaskFunc = func(ctx context.Context, task TaskConfig) {
		start := s.now()
		// Record the START time, not the clock after the run: a real
		// doExecuteTask advances the clock while the script executes, so
		// attributing a post-run timestamp would misreport when the task
		// actually fired (RR-14NHU6).
		tracker.recordAt(task.Script, start)
		if fail() {
			s.recordFailure(ctx, task, start, 0, errBoom)
			return
		}
		// Call the SAME bookkeeping the production path uses. Hand-copying
		// it here is what previously let a reverted start-time fix pass
		// green (RR-F6182G / RR-3BCWQ4).
		s.recordSuccess(ctx, task, start)
	}
	return s, tracker
}

// tickFor advances the clock in `step` increments for `total`, calling
// runDueTasks at each tick, and returns the times at which the task started.
// Times come from the tracker (recorded inside the run) rather than from the
// clock after runDueTasks returns, so one tick firing N tasks is not
// indistinguishable from one task firing N times.
func tickFor(
	s *Scheduler,
	clock *time.Time,
	tracker *mockTracker,
	step, total time.Duration,
) []time.Time {
	base := *clock
	for elapsed := time.Duration(0); elapsed <= total; elapsed += step {
		*clock = base.Add(elapsed)
		s.runDueTasks(context.Background())
	}
	return tracker.getTimes()
}

// TestRunDueTasks_failingTaskDoesNotHotLoop is the direct regression for the
// reported symptom: a daily task whose script fails must not run on every tick.
func TestRunDueTasks_failingTaskDoesNotHotLoop(t *testing.T) {
	t.Parallel()

	cfg := &Config{Tasks: []TaskConfig{
		{Name: "daily", Script: "daily.lua", Every: dailySchedule()},
	}}
	clock := time.Date(2026, 4, 10, 9, 0, 0, 0, time.Local)
	s, tracker := newRetryTestScheduler(t, cfg, &clock, func() bool { return true })

	// 10 ticks a minute apart, all inside the same calendar day. Pre-fix this
	// produced 10 executions.
	fired := tickFor(s, &clock, tracker, time.Minute, 9*time.Minute)

	// Only the initial run plus the 5m ladder step fall in this window.
	if len(fired) != 2 {
		t.Fatalf("expected 2 executions (initial + 5m retry) in 9m, got %d at %v", len(fired), fired)
	}
	if got := fired[1].Sub(fired[0]); got != baseRetryDelay {
		t.Errorf("first retry after %v, want %v", got, baseRetryDelay)
	}
}

// TestRunDueTasks_retryLadder pins the exact backoff steps.
func TestRunDueTasks_retryLadder(t *testing.T) {
	t.Parallel()

	cfg := &Config{Tasks: []TaskConfig{
		{Name: "daily", Script: "daily.lua", Every: dailySchedule()},
	}}
	base := time.Date(2026, 4, 10, 0, 0, 0, 0, time.Local)
	clock := base
	s, tracker := newRetryTestScheduler(t, cfg, &clock, func() bool { return true })

	// Tick every minute for 8h: initial run at t=0, then 5m, 10m, 20m, 40m,
	// 80m, then capped at 2h.
	fired := tickFor(s, &clock, tracker, time.Minute, 8*time.Hour)

	want := []time.Duration{
		0,
		5 * time.Minute,   // +5m
		15 * time.Minute,  // +10m
		35 * time.Minute,  // +20m
		75 * time.Minute,  // +40m
		155 * time.Minute, // +80m
		275 * time.Minute, // +2h (capped)
		395 * time.Minute, // +2h
	}
	if len(fired) != len(want) {
		t.Fatalf("got %d executions, want %d (offsets %v)", len(fired), len(want), offsets(fired, base))
	}
	for i, w := range want {
		if got := fired[i].Sub(base); got != w {
			t.Errorf("execution %d at +%v, want +%v", i, got, w)
		}
	}
}

// TestRunDueTasks_shortIntervalTaskBacksOff covers the user's explicit
// requirement: a failing 5m task must climb the ladder, not keep firing at its
// normal 5m cadence.
func TestRunDueTasks_shortIntervalTaskBacksOff(t *testing.T) {
	t.Parallel()

	cfg := &Config{Tasks: []TaskConfig{
		{Name: "freq", Script: "freq.lua", Every: intervalSchedule(5 * time.Minute)},
	}}
	base := time.Date(2026, 4, 10, 0, 0, 0, 0, time.UTC)
	clock := base
	s, tracker := newRetryTestScheduler(t, cfg, &clock, func() bool { return true })

	fired := tickFor(s, &clock, tracker, time.Minute, 6*time.Hour)

	// On its normal 5m cadence a 6h window would yield ~73 runs. The ladder
	// must cut that to the same handful of steps a daily task gets.
	if len(fired) > 8 {
		t.Fatalf("failing 5m task ran %d times in 6h — not backing off (offsets %v)",
			len(fired), offsets(fired, base))
	}
	// The gaps must widen rather than hold at the 5m schedule.
	if got := fired[1].Sub(fired[0]); got != baseRetryDelay {
		t.Errorf("first retry after %v, want %v", got, baseRetryDelay)
	}
	if got := fired[2].Sub(fired[1]); got != 2*baseRetryDelay {
		t.Errorf("second retry after %v, want %v", got, 2*baseRetryDelay)
	}
	// By 6h the ladder has climbed past its rungs and holds at the cap.
	last := fired[len(fired)-1].Sub(fired[len(fired)-2])
	if last != maxRetryDelay {
		t.Errorf("final gap %v, want the %v cap", last, maxRetryDelay)
	}
}

// TestRunDueTasks_scheduleSuppressedWhileFailing verifies reading A: while a
// retry is pending the normal schedule must not fire the task.
func TestRunDueTasks_scheduleSuppressedWhileFailing(t *testing.T) {
	t.Parallel()

	cfg := &Config{Tasks: []TaskConfig{
		{Name: "freq", Script: "freq.lua", Every: intervalSchedule(time.Minute)},
	}}
	base := time.Date(2026, 4, 10, 0, 0, 0, 0, time.UTC)
	clock := base
	s, tracker := newRetryTestScheduler(t, cfg, &clock, func() bool { return true })

	// A 1m schedule would fire on nearly every tick if the schedule were still
	// consulted while failing.
	fired := tickFor(s, &clock, tracker, 30*time.Second, 4*time.Minute)

	if len(fired) != 1 {
		t.Fatalf("expected only the initial run within 4m (5m ladder step not yet due), got %d at %v",
			len(fired), offsets(fired, base))
	}
}

// TestRunDueTasks_successResetsLadder verifies the ladder clears on success and
// the task returns to its normal schedule.
func TestRunDueTasks_successResetsLadder(t *testing.T) {
	t.Parallel()

	cfg := &Config{Tasks: []TaskConfig{
		{Name: "daily", Script: "daily.lua", Every: dailySchedule()},
	}}
	base := time.Date(2026, 4, 10, 0, 0, 0, 0, time.Local)
	clock := base
	failing := true
	s, _ := newRetryTestScheduler(t, cfg, &clock, func() bool { return failing })

	// Fail once to arm the ladder.
	s.runDueTasks(context.Background())
	if s.state.Failures["daily"] != 1 {
		t.Fatalf("expected 1 failure recorded, got %d", s.state.Failures["daily"])
	}
	if _, pending := s.state.NextRetry["daily"]; !pending {
		t.Fatal("expected a pending retry after failure")
	}

	// Succeed on the 5m retry.
	failing = false
	clock = base.Add(baseRetryDelay)
	s.runDueTasks(context.Background())

	if n, ok := s.state.Failures["daily"]; ok {
		t.Errorf("failure count not cleared after success: %d", n)
	}
	if _, pending := s.state.NextRetry["daily"]; pending {
		t.Error("pending retry not cleared after success")
	}
	if got := s.state.Tasks["daily"]; !got.Equal(clock) {
		t.Errorf("last-run stamped %v, want the run start %v", got, clock)
	}
}

// newRealPathScheduler builds a scheduler that runs the PRODUCTION
// doExecuteTask against a real script engine and a real script on disk.
//
// The retry tests drive executeTaskFunc, which bypasses doExecuteTask
// entirely; the state bookkeeping inside it therefore needs its own coverage,
// or reverting it passes green (RR-F6182G).
//
// The clock advances by runDuration on every call after the first, simulating
// a script that takes time to run — which is exactly what distinguishes the
// start timestamp from the completion timestamp.
func newRealPathScheduler(
	t *testing.T,
	cfg *Config,
	luaSource string,
	start time.Time,
	runDuration time.Duration,
) *Scheduler {
	t.Helper()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "scripts"), 0o750); err != nil {
		t.Fatalf("mkdir scripts: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "scripts", "task.lua"), []byte(luaSource), 0o600); err != nil {
		t.Fatalf("write script: %v", err)
	}

	ws := newMockWorkspace(t)
	ws.luaDepsProvider = func() lua.WriteDeps {
		return lua.WriteDeps{
			ReadDeps: lua.ReadDeps{ProjectRoot: root},
			// A writer runtime panics without one. These scripts never
			// mutate, so a stub that fails loudly if called is enough.
			EntityManager: stubMutator{},
		}
	}

	s := New(cfg, script.NewEngine(), ws, discardLogger())
	calls := 0
	s.now = func() time.Time {
		defer func() { calls++ }()
		if calls == 0 {
			return start
		}
		return start.Add(runDuration)
	}
	return s
}

// TestDoExecuteTask_recordsStartTimeNotCompletion pins the secondary defect
// through the REAL doExecuteTask: a task starting at 23:59 that runs past
// midnight must stamp its start time, or it consumes the next day's slot.
func TestDoExecuteTask_recordsStartTimeNotCompletion(t *testing.T) {
	t.Parallel()

	every := dailySchedule()
	cfg := &Config{Tasks: []TaskConfig{
		{Name: "daily", Script: "task.lua", Every: every},
	}}
	start := time.Date(2026, 4, 10, 23, 59, 0, 0, time.Local)
	// The script runs for two minutes, crossing midnight.
	s := newRealPathScheduler(t, cfg, "return true\n", start, 2*time.Minute)

	s.doExecuteTask(context.Background(), cfg.Tasks[0])

	got, ok := s.state.Tasks["daily"]
	if !ok {
		t.Fatal("successful run recorded no timestamp")
	}
	if !got.Equal(start) {
		t.Fatalf("recorded %v, want the start time %v", got, start)
	}
	// The next day's slot must still be due.
	if !every.IsDue(got, start.Add(24*time.Hour)) {
		t.Error("next day's run was skipped — completion time was recorded instead of start")
	}
}

// TestDoExecuteTask_failureDoesNotCountAsRun pins recordFailure's core
// invariant through the real path: a failed attempt must not stamp
// state.Tasks, which is the "last SUCCESSFUL run" the schedule is evaluated
// against. Without this, injecting that write passes green (RR-QOSJZ5).
func TestDoExecuteTask_failureDoesNotCountAsRun(t *testing.T) {
	t.Parallel()

	cfg := &Config{Tasks: []TaskConfig{
		{Name: "daily", Script: "task.lua", Every: dailySchedule()},
	}}
	start := time.Date(2026, 4, 10, 9, 0, 0, 0, time.Local)
	s := newRealPathScheduler(t, cfg, `error("boom")`, start, time.Second)

	s.doExecuteTask(context.Background(), cfg.Tasks[0])

	if _, ok := s.state.Tasks["daily"]; ok {
		t.Error("a failed run stamped state.Tasks — the schedule would treat it as having succeeded")
	}
	if got := s.state.Failures["daily"]; got != 1 {
		t.Errorf("Failures = %d, want 1", got)
	}
	if retryAt, ok := s.state.NextRetry["daily"]; !ok {
		t.Error("no retry scheduled after failure")
	} else if want := start.Add(baseRetryDelay); !retryAt.Equal(want) {
		// Based on START, not completion, so a slow failure doesn't drift.
		t.Errorf("retry at %v, want %v", retryAt, want)
	}
}

// TestRunDueTasks_implausibleRetryTimeIsClamped covers a clock jump or a
// hand-edited state file: an unbounded future retry would wedge the task
// forever, silently, since the not-yet-due branch logs nothing (RR-R6YXKM).
func TestRunDueTasks_implausibleRetryTimeIsClamped(t *testing.T) {
	t.Parallel()

	cfg := &Config{Tasks: []TaskConfig{
		{Name: "daily", Script: "daily.lua", Every: dailySchedule()},
	}}
	base := time.Date(2026, 4, 10, 0, 0, 0, 0, time.UTC)
	clock := base
	s, tracker := newRetryTestScheduler(t, cfg, &clock, func() bool { return false })
	s.state.NextRetry["daily"] = base.AddDate(100, 0, 0)

	s.runDueTasks(context.Background())

	if got := len(tracker.getCalls()); got != 1 {
		t.Fatalf("task ran %d times, want 1 — an implausible retry time wedged it", got)
	}
}

// TestPruneOrphanedState covers state left behind by tasks removed from
// schedules.yaml, which nothing else cleans up (RR-7GYJ60).
func TestPruneOrphanedState(t *testing.T) {
	t.Parallel()

	cfg := &Config{Tasks: []TaskConfig{
		{Name: "live", Script: "live.lua", Every: dailySchedule()},
	}}
	ts := time.Date(2026, 4, 10, 9, 0, 0, 0, time.UTC)
	seeded := &State{
		Tasks:     map[string]time.Time{"live": ts, "removed": ts},
		Failures:  map[string]int{"removed": 3},
		NextRetry: map[string]time.Time{"removed": ts},
	}
	data, err := json.Marshal(seeded)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	ws := newMockWorkspace(t)
	ws.cacheFiles[stateFile] = data
	s := New(cfg, nil, ws, discardLogger())

	s.loadState(context.Background())

	if _, ok := s.state.Tasks["live"]; !ok {
		t.Error("pruned a task that is still configured")
	}
	for _, m := range []string{"Tasks", "Failures", "NextRetry"} {
		var present bool
		switch m {
		case "Tasks":
			_, present = s.state.Tasks["removed"]
		case "Failures":
			_, present = s.state.Failures["removed"]
		case "NextRetry":
			_, present = s.state.NextRetry["removed"]
		}
		if present {
			t.Errorf("%s still holds an entry for a task no longer in the config", m)
		}
	}
}

func TestRetryDelay(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		failures int
		want     time.Duration
	}{
		{"corrupt zero count treated as first failure", 0, baseRetryDelay},
		{"negative count treated as first failure", -5, baseRetryDelay},
		{"first failure", 1, 5 * time.Minute},
		{"second doubles", 2, 10 * time.Minute},
		{"third doubles", 3, 20 * time.Minute},
		{"fourth doubles", 4, 40 * time.Minute},
		{"fifth doubles", 5, 80 * time.Minute},
		{"sixth reaches the cap", 6, maxRetryDelay},
		{"holds at the cap", 7, maxRetryDelay},
		{"large count never exceeds the cap", 50, maxRetryDelay},
		{"overflow-sized count never exceeds the cap", 1 << 40, maxRetryDelay},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := retryDelay(tc.failures); got != tc.want {
				t.Errorf("retryDelay(%d) = %v, want %v", tc.failures, got, tc.want)
			}
		})
	}
}

func offsets(times []time.Time, base time.Time) []time.Duration {
	out := make([]time.Duration, len(times))
	for i, tm := range times {
		out[i] = tm.Sub(base)
	}
	return out
}
