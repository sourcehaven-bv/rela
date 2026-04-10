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
	return &mockWorkspace{
		cacheFiles: make(map[string][]byte),
		paths:      &project.Context{Root: t.TempDir()},
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
	m.cacheFiles[name] = append([]byte(nil), data...)
	return nil
}

type notFoundError struct{ name string }

func (e *notFoundError) Error() string { return "not found: " + e.name }

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
		wsRaw:  ws,
		state:  newState(),
		logger: discardLogger(),
		now:    func() time.Time { return now },
	}
	s.executeTaskFunc = func(_ context.Context, task TaskConfig) {
		tracker.record(task.Script)
		s.state.Tasks[task.Name] = s.now()
		s.saveState()
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
			{Name: "test", Script: "test.lua", Every: dailySchedule()},
		},
	}
	now := time.Date(2026, 4, 10, 14, 0, 0, 0, time.UTC)
	s, ws, _ := newTestScheduler(t, cfg, now)

	s.executeTaskFunc = func(ctx context.Context, task TaskConfig) {
		s.doExecuteTask(ctx, task)
	}

	s.runDueTasks(context.Background())

	if ws.syncCount.Load() < 1 {
		t.Error("expected Sync() to be called before task execution")
	}
}

func TestScheduler_Run_emptyConfig(t *testing.T) {
	t.Parallel()

	ws := newMockWorkspace(t)
	s := &Scheduler{
		config: &Config{Tasks: nil},
		ws:     ws,
		wsRaw:  ws,
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

	task := TaskConfig{Name: "test", Script: "test.lua", Every: dailySchedule()}
	s.doExecuteTask(ctx, task)

	if ws.syncCount.Load() != 0 {
		t.Error("expected Sync not to be called with cancelled context")
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
