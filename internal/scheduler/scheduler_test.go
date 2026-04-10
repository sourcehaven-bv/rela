package scheduler

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/robfig/cron/v3"

	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/model"
	"github.com/Sourcehaven-BV/rela/internal/project"
)

// --- test helpers ---

type mockWorkspace struct {
	mu         sync.Mutex
	cacheFiles map[string][]byte
	syncCount  atomic.Int32
	paths      *project.Context
	meta       *metamodel.Metamodel
}

func newMockWorkspace(t *testing.T) *mockWorkspace {
	t.Helper()
	root := t.TempDir()
	return &mockWorkspace{
		cacheFiles: make(map[string][]byte),
		paths:      &project.Context{Root: root},
		meta:       &metamodel.Metamodel{},
	}
}

func (m *mockWorkspace) Sync() (*model.SyncResult, error) {
	m.syncCount.Add(1)
	return &model.SyncResult{}, nil
}

func (m *mockWorkspace) Meta() *metamodel.Metamodel { return m.meta }
func (m *mockWorkspace) Paths() *project.Context    { return m.paths }

func (m *mockWorkspace) ReadCacheFile(name string) ([]byte, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	data, ok := m.cacheFiles[name]
	if !ok {
		return nil, &notFoundError{name}
	}
	return data, nil
}

func (m *mockWorkspace) WriteCacheFile(name string, data []byte) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.cacheFiles[name] = append([]byte(nil), data...) // defensive copy
	return nil
}

type notFoundError struct{ name string }

func (e *notFoundError) Error() string { return "not found: " + e.name }

// mockTracker records script paths executed.
type mockTracker struct {
	mu    sync.Mutex
	calls []string
}

func (m *mockTracker) record(path string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.calls = append(m.calls, path)
}

func (m *mockTracker) getCalls() []string {
	m.mu.Lock()
	defer m.mu.Unlock()
	result := make([]string, len(m.calls))
	copy(result, m.calls)
	return result
}

func discardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func newTestScheduler(t *testing.T, cfg *Config, now time.Time) (*Scheduler, *mockWorkspace, *mockTracker) {
	t.Helper()
	ws := newMockWorkspace(t)
	tracker := &mockTracker{}
	s := &Scheduler{
		config: cfg,
		ws:     ws,
		wsRaw:  ws,
		state:  newState(),
		logger: discardLogger(),
		now:    func() time.Time { return now },
	}
	s.executeTaskFunc = func(_ context.Context, task TaskConfig) {
		tracker.record(task.Script)
		s.stateMu.Lock()
		s.state.Tasks[task.Name] = s.now()
		s.stateMu.Unlock()
		s.saveState()
	}
	return s, ws, tracker
}

// --- tests ---

func TestPrevScheduleTime_daily(t *testing.T) {
	t.Parallel()

	parser := cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)
	sched, _ := parser.Parse("0 9 * * *")
	now := time.Date(2026, 4, 10, 14, 0, 0, 0, time.UTC)

	prev := prevScheduleTime(sched, now)
	expected := time.Date(2026, 4, 10, 9, 0, 0, 0, time.UTC)
	if !prev.Equal(expected) {
		t.Errorf("prevScheduleTime = %v, want %v", prev, expected)
	}
}

func TestPrevScheduleTime_beforeFirstRunToday(t *testing.T) {
	t.Parallel()

	parser := cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)
	sched, _ := parser.Parse("0 9 * * *")
	now := time.Date(2026, 4, 10, 8, 0, 0, 0, time.UTC)

	prev := prevScheduleTime(sched, now)
	expected := time.Date(2026, 4, 9, 9, 0, 0, 0, time.UTC)
	if !prev.Equal(expected) {
		t.Errorf("prevScheduleTime = %v, want %v", prev, expected)
	}
}

func TestPrevScheduleTime_weekly(t *testing.T) {
	t.Parallel()

	parser := cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)
	// Monday at 9am
	sched, _ := parser.Parse("0 9 * * 1")
	// Wednesday 2pm — last Monday was 2 days ago
	now := time.Date(2026, 4, 8, 14, 0, 0, 0, time.UTC) // Wednesday

	prev := prevScheduleTime(sched, now)
	expected := time.Date(2026, 4, 6, 9, 0, 0, 0, time.UTC) // Monday
	if !prev.Equal(expected) {
		t.Errorf("prevScheduleTime = %v, want %v", prev, expected)
	}
}

func TestScheduler_missedRun_firstEver(t *testing.T) {
	t.Parallel()

	cfg := &Config{
		Tasks: []TaskConfig{
			{Name: "check", Script: "check.lua", Schedule: "0 9 * * *"},
		},
	}

	now := time.Date(2026, 4, 10, 14, 0, 0, 0, time.UTC)
	s, _, tracker := newTestScheduler(t, cfg, now)

	s.executeMissedRuns(context.Background())

	calls := tracker.getCalls()
	if len(calls) != 1 || calls[0] != "check.lua" {
		t.Errorf("expected 1 call to check.lua, got %v", calls)
	}
}

func TestScheduler_missedRun_detected(t *testing.T) {
	t.Parallel()

	cfg := &Config{
		Tasks: []TaskConfig{
			{Name: "daily", Script: "daily.lua", Schedule: "0 9 * * *"},
		},
	}

	now := time.Date(2026, 4, 10, 14, 0, 0, 0, time.UTC)
	lastRun := time.Date(2026, 4, 9, 9, 0, 0, 0, time.UTC) // yesterday 9am

	s, _, tracker := newTestScheduler(t, cfg, now)
	s.state.Tasks["daily"] = lastRun

	s.executeMissedRuns(context.Background())

	calls := tracker.getCalls()
	if len(calls) != 1 || calls[0] != "daily.lua" {
		t.Errorf("expected 1 missed run call, got %v", calls)
	}
}

func TestScheduler_noMissedRun(t *testing.T) {
	t.Parallel()

	cfg := &Config{
		Tasks: []TaskConfig{
			{Name: "daily", Script: "daily.lua", Schedule: "0 9 * * *"},
		},
	}

	now := time.Date(2026, 4, 10, 9, 30, 0, 0, time.UTC)
	lastRun := time.Date(2026, 4, 10, 9, 5, 0, 0, time.UTC) // ran after today's window

	s, _, tracker := newTestScheduler(t, cfg, now)
	s.state.Tasks["daily"] = lastRun

	s.executeMissedRuns(context.Background())

	if calls := tracker.getCalls(); len(calls) != 0 {
		t.Errorf("expected no calls, got %v", calls)
	}
}

func TestScheduler_statePersistedAfterRun(t *testing.T) {
	t.Parallel()

	cfg := &Config{
		Tasks: []TaskConfig{
			{Name: "test", Script: "test.lua", Schedule: "0 9 * * *"},
		},
	}

	now := time.Date(2026, 4, 10, 14, 0, 0, 0, time.UTC)
	s, ws, _ := newTestScheduler(t, cfg, now)

	s.executeMissedRuns(context.Background())

	data, err := ws.ReadCacheFile(stateFile)
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
	s.loadState()

	if s.state == nil || s.state.Tasks == nil {
		t.Fatal("expected initialized state")
	}
	if len(s.state.Tasks) != 0 {
		t.Errorf("expected empty state, got %d entries", len(s.state.Tasks))
	}
}

func TestScheduler_loadState_existing(t *testing.T) {
	t.Parallel()

	ws := newMockWorkspace(t)
	ts := time.Date(2026, 4, 10, 9, 0, 0, 0, time.UTC)
	stateData, _ := json.Marshal(State{Tasks: map[string]time.Time{"daily": ts}})
	ws.cacheFiles[stateFile] = stateData

	s := &Scheduler{ws: ws, logger: discardLogger()}
	s.loadState()

	if got := s.state.Tasks["daily"]; !got.Equal(ts) {
		t.Errorf("loaded state: daily = %v, want %v", got, ts)
	}
}

func TestScheduler_syncCalledBeforeExecution(t *testing.T) {
	t.Parallel()

	cfg := &Config{
		Tasks: []TaskConfig{
			{Name: "test", Script: "test.lua", Schedule: "0 9 * * *"},
		},
	}

	now := time.Date(2026, 4, 10, 14, 0, 0, 0, time.UTC)
	s, ws, _ := newTestScheduler(t, cfg, now)

	// Use doExecuteTask to verify Sync is called.
	s.executeTaskFunc = func(ctx context.Context, task TaskConfig) {
		s.doExecuteTask(ctx, task)
	}

	s.executeMissedRuns(context.Background())

	if ws.syncCount.Load() < 1 {
		t.Error("expected Sync() to be called before task execution")
	}
}

func TestScheduler_Run_emptyConfig(t *testing.T) {
	t.Parallel()

	cfg := &Config{Tasks: nil}
	ws := newMockWorkspace(t)

	s := &Scheduler{
		config: cfg,
		ws:     ws,
		wsRaw:  ws,
		logger: discardLogger(),
		now:    time.Now,
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	err := s.Run(ctx)
	if err != nil {
		t.Errorf("Run with empty config should return nil, got %v", err)
	}
}

func TestScheduler_missedRun_cancelledContext(t *testing.T) {
	t.Parallel()

	cfg := &Config{
		Tasks: []TaskConfig{
			{Name: "a", Script: "a.lua", Schedule: "0 9 * * *"},
			{Name: "b", Script: "b.lua", Schedule: "0 9 * * *"},
		},
	}

	now := time.Date(2026, 4, 10, 14, 0, 0, 0, time.UTC)
	s, _, tracker := newTestScheduler(t, cfg, now)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel before executeMissedRuns

	s.executeMissedRuns(ctx)

	// With cancelled context, no tasks should execute.
	if calls := tracker.getCalls(); len(calls) != 0 {
		t.Errorf("expected no calls with cancelled context, got %v", calls)
	}
}

func TestScheduler_doExecuteTask_skipsOnCancelledContext(t *testing.T) {
	t.Parallel()

	ws := newMockWorkspace(t)
	s := &Scheduler{
		ws:     ws,
		wsRaw:  ws,
		state:  newState(),
		logger: discardLogger(),
		now:    time.Now,
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	task := TaskConfig{Name: "test", Script: "test.lua", Schedule: "* * * * *"}
	s.doExecuteTask(ctx, task)

	// Sync should not be called when context is cancelled.
	if ws.syncCount.Load() != 0 {
		t.Error("expected Sync not to be called with cancelled context")
	}
}
