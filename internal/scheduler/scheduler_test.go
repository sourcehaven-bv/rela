package scheduler

import (
	"context"
	"encoding/json"
	"log/slog"
	"sync"
	"testing"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/config"
	"github.com/Sourcehaven-BV/rela/internal/lua"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/project"
	"github.com/Sourcehaven-BV/rela/internal/state"
)

// --- test helpers ---

type mockWorkspace struct {
	mu         sync.Mutex
	cacheFiles map[string][]byte
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

func (m *mockWorkspace) Paths() *project.Context { return m.paths }

func (m *mockWorkspace) Config() config.Loader { return &mockConfig{m: m} }

func (m *mockWorkspace) State() state.KV { return &mockState{m: m} }

func (m *mockWorkspace) LuaWriteDeps() lua.WriteDeps { return lua.WriteDeps{} }

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
		metaFn: func() *metamodel.Metamodel { return ws.meta },

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
	s := &Scheduler{ws: ws, metaFn: func() *metamodel.Metamodel { return ws.meta }, logger: discardLogger()}
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

	s := &Scheduler{ws: ws, metaFn: func() *metamodel.Metamodel { return ws.meta }, logger: discardLogger()}
	s.loadState()

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
		metaFn: func() *metamodel.Metamodel { return ws.meta },

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
	metaFn := func() *metamodel.Metamodel { return ws.meta }

	// ws.Config().Load returns notFoundError for missing file.
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Should not panic, should not log errors.
	StartBackground(ctx, ws, metaFn, discardLogger())
}

func TestStartBackground_InvalidConfig(t *testing.T) {
	ws := newMockWorkspace(t)
	ws.cacheFiles["project:"+ConfigFile] = []byte("not: valid: yaml: at all:")
	metaFn := func() *metamodel.Metamodel { return ws.meta }

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Should log error and return without starting a goroutine.
	StartBackground(ctx, ws, metaFn, discardLogger())
}

func TestStartBackground_EmptyTasks(t *testing.T) {
	ws := newMockWorkspace(t)
	ws.cacheFiles["project:"+ConfigFile] = []byte("tasks: []\n")
	metaFn := func() *metamodel.Metamodel { return ws.meta }

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	StartBackground(ctx, ws, metaFn, discardLogger())
}

func TestNew(t *testing.T) {
	cfg := &Config{Tasks: []TaskConfig{{Name: "t", Script: "t.lua"}}}
	ws := newMockWorkspace(t)
	metaFn := func() *metamodel.Metamodel { return ws.meta }

	s := New(cfg, nil, ws, metaFn, discardLogger())
	if s == nil {
		t.Fatal("New returned nil")
	}
	if s.config != cfg {
		t.Error("config not wired")
	}
	if s.ws != ws {
		t.Error("ws not wired")
	}
	if s.metaFn == nil {
		t.Error("metaFn not wired")
	}
}

