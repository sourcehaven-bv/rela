package scheduler

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/config"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/jobs"
	"github.com/Sourcehaven-BV/rela/internal/lua"
	"github.com/Sourcehaven-BV/rela/internal/project"
	"github.com/Sourcehaven-BV/rela/internal/schedulerstate"
	"github.com/Sourcehaven-BV/rela/internal/schedulerstate/kvstate"
	"github.com/Sourcehaven-BV/rela/internal/script"
	"github.com/Sourcehaven-BV/rela/internal/state"
)

// mockWorkspace is a WorkspaceProvider over in-memory files and a real
// kvstate run-state store, so the scheduler is tested against the same store
// contract production uses.
type mockWorkspace struct {
	mu              sync.Mutex
	files           map[string][]byte
	paths           *project.Context
	runs            schedulerstate.Store
	luaDepsCalls    int
	luaDepsProvider func() lua.WriteDeps
}

func newMockWorkspace(t *testing.T) *mockWorkspace {
	t.Helper()
	m := &mockWorkspace{
		files: make(map[string][]byte),
		paths: &project.Context{Root: t.TempDir()},
	}
	runs, err := kvstate.New(&mockState{m: m})
	require.NoError(t, err)
	m.runs = runs
	return m
}

func (m *mockWorkspace) Paths() *project.Context { return m.paths }

func (m *mockWorkspace) Config() config.Loader { return &mockConfig{m: m} }

func (m *mockWorkspace) State() state.KV { return &mockState{m: m} }

func (m *mockWorkspace) SchedulerState() schedulerstate.Store { return m.runs }

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
	data, ok := c.m.files["project:"+name]
	if !ok {
		return nil, notFound(name)
	}
	return data, nil
}

// List reports no directory-shaped config: the scheduler reads
// schedules.yaml by name and nothing else.
func (c *mockConfig) List(_ context.Context, _ string) ([]string, error) { return nil, nil }

type mockState struct{ m *mockWorkspace }

func (s *mockState) Get(_ context.Context, key string) ([]byte, error) {
	s.m.mu.Lock()
	defer s.m.mu.Unlock()
	data, ok := s.m.files[key]
	if !ok {
		return nil, notFound(key)
	}
	return data, nil
}

func (s *mockState) Put(_ context.Context, key string, data []byte) error {
	s.m.mu.Lock()
	defer s.m.mu.Unlock()
	s.m.files[key] = append([]byte(nil), data...)
	return nil
}

func (s *mockState) Delete(_ context.Context, key string) error {
	s.m.mu.Lock()
	defer s.m.mu.Unlock()
	delete(s.m.files, key)
	return nil
}

// notFound is how the real config and state backends report a missing key.
func notFound(name string) error {
	return &os.PathError{Op: "get", Path: name, Err: os.ErrNotExist}
}

// errBoom is the injected failure used by the retry-ladder tests.
var errBoom = errors.New("boom")

// stubMutator satisfies lua.Mutator so a writer runtime can be constructed.
// Every method fails, so a test script that writes fails loudly.
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

func discardLogger() *slog.Logger {
	return slog.New(slog.DiscardHandler)
}

func dailySchedule() Schedule {
	return Schedule{kind: dayKind, set: true}
}

func intervalSchedule(d time.Duration) Schedule {
	return Schedule{kind: intervalKind, interval: d, set: true}
}

// clock is a settable time source shared by a scheduler and its test.
type clock struct {
	mu  sync.Mutex
	now time.Time
}

func (c *clock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

func (c *clock) Set(t time.Time) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.now = t
}

func (c *clock) Advance(d time.Duration) { c.Set(c.Now().Add(d)) }

// fakeQueue records jobs and runs them only when the test calls drain, so a
// test controls exactly when a worker "picks up" a run.
type fakeQueue struct {
	mu       sync.Mutex
	handlers map[jobs.Kind]jobs.Handler
	all      []jobs.Job
	pending  []jobs.Job
	enqueErr error
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
	defer f.mu.Unlock()
	if f.enqueErr != nil {
		return f.enqueErr
	}
	job.Attempt = 1
	f.all = append(f.all, job)
	f.pending = append(f.pending, job)
	return nil
}

// jobs returns every job accepted so far.
func (f *fakeQueue) jobs() []jobs.Job {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]jobs.Job(nil), f.all...)
}

// take removes and returns the pending jobs.
func (f *fakeQueue) take() []jobs.Job {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := f.pending
	f.pending = nil
	return out
}

// deliver runs one job through its registered handler.
func (f *fakeQueue) deliver(t *testing.T, job jobs.Job) error {
	t.Helper()
	f.mu.Lock()
	h, ok := f.handlers[job.Kind]
	f.mu.Unlock()
	require.True(t, ok, "no handler for %s", job.Kind)
	return h(context.Background(), job)
}

// drain delivers pending jobs, including any they enqueue, until none remain.
func (f *fakeQueue) drain(t *testing.T) {
	t.Helper()
	for {
		batch := f.take()
		if len(batch) == 0 {
			return
		}
		for _, job := range batch {
			_ = f.deliver(t, job)
		}
	}
}

// newTestScheduler builds a scheduler over ws with a fake queue and a
// settable clock.
func newTestScheduler(
	t *testing.T, ws WorkspaceProvider, now time.Time, tasks ...TaskConfig,
) (*Scheduler, *fakeQueue, *clock) {
	t.Helper()
	s, err := New(&Config{Tasks: tasks}, script.NewEngine(), ws, discardLogger())
	require.NoError(t, err)
	c := &clock{now: now}
	s.now = c.Now
	q := newFakeQueue()
	require.NoError(t, s.UseQueue(q))
	return s, q, c
}

// taskState loads one task's run-state.
func taskState(t *testing.T, ws WorkspaceProvider, task string) (schedulerstate.TaskState, bool) {
	t.Helper()
	got, err := ws.SchedulerState().Load(context.Background(), []string{task})
	require.NoError(t, err)
	ts, ok := got[task]
	return ts, ok
}
