package appbuild

import (
	"context"
	"errors"
	"io/fs"
	"iter"
	"log/slog"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/audit"
	"github.com/Sourcehaven-BV/rela/internal/autocascade"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/jobs"
	"github.com/Sourcehaven-BV/rela/internal/lua"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/principal"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// mapKV is an in-memory state.KV.
type mapKV struct {
	mu sync.Mutex
	m  map[string][]byte
}

func (k *mapKV) Get(_ context.Context, key string) ([]byte, error) {
	k.mu.Lock()
	defer k.mu.Unlock()
	v, ok := k.m[key]
	if !ok {
		return nil, fs.ErrNotExist
	}
	return v, nil
}

func (k *mapKV) Put(_ context.Context, key string, data []byte) error {
	k.mu.Lock()
	defer k.mu.Unlock()
	k.m[key] = data
	return nil
}

func (k *mapKV) Delete(_ context.Context, key string) error {
	k.mu.Lock()
	defer k.mu.Unlock()
	delete(k.m, key)
	return nil
}

// fakeReader serves one readable entity.
type fakeReader struct{ lua.EntityReader }

func (fakeReader) GetAddress(_ context.Context, addr string) (*entity.Entity, error) {
	if addr != "NOTE-1" {
		return nil, store.ErrNotFound
	}
	return &entity.Entity{ID: addr, Type: "note"}, nil
}

// fakeJobRunner records runs. The first run blocks on gate when one is
// set; during, when set, runs inside every run.
type fakeJobRunner struct {
	mu      sync.Mutex
	runs    []principal.Principal
	labels  []string
	files   []string
	hops    []int
	started chan struct{}
	gate    chan struct{}
	during  func(ctx context.Context, path string)
}

func (f *fakeJobRunner) ScheduledLuaWriteDeps() lua.WriteDeps {
	var d lua.WriteDeps
	d.VisibleReader = fakeReader{}
	return d
}

func (f *fakeJobRunner) BindIdentity(ctx context.Context) (context.Context, error) { return ctx, nil }

func (f *fakeJobRunner) ExecuteFile(ctx context.Context, path string, _ lua.WriteDeps, _ *entity.Entity) error {
	f.mu.Lock()
	f.runs = append(f.runs, principal.From(ctx))
	f.labels = append(f.labels, audit.TriggeredByFrom(ctx))
	f.files = append(f.files, path)
	hops, _ := ctx.Value(jobHopsKey{}).(int)
	f.hops = append(f.hops, hops)
	first := len(f.runs) == 1
	f.mu.Unlock()
	if first && f.started != nil {
		close(f.started)
	}
	if first && f.gate != nil {
		<-f.gate
	}
	if f.during != nil {
		f.during(ctx, path)
	}
	return nil
}

func (f *fakeJobRunner) count() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.runs)
}

func jobsMeta(runAs string, files ...string) *metamodel.Metamodel {
	if len(files) == 0 {
		files = []string{"push.lua"}
	}
	var do []metamodel.AutomationAction
	for _, f := range files {
		do = append(do, metamodel.AutomationAction{LuaFile: f, Background: true, RunAs: runAs})
	}
	return &metamodel.Metamodel{Automations: []metamodel.AutomationDef{{Name: "push", Do: do}}}
}

var note1 = autocascade.BackgroundScript{Automation: "push", LuaFile: "push.lua", Ref: entity.Ref{ID: "NOTE-1"}}

func scriptFor(file string) autocascade.BackgroundScript {
	s := note1
	s.LuaFile = file
	return s
}

func newTestJobs(t *testing.T, meta *metamodel.Metamodel, q jobs.Client, r *fakeJobRunner) *automationJobs {
	t.Helper()
	a, err := newAutomationJobs(meta, &mapKV{m: map[string][]byte{}}, noEntities{}, q)
	require.NoError(t, err)
	a.followUpDelay = time.Millisecond
	require.NoError(t, a.bind(r, nil))
	return a
}

// noEntities is a raw store with nothing in it.
type noEntities struct{}

func (noEntities) ListEntities(context.Context, store.EntityQuery) iter.Seq2[*entity.Entity, error] {
	return func(func(*entity.Entity, error) bool) {}
}

func startedQueue(t *testing.T) jobs.Queue {
	t.Helper()
	q, err := jobs.NewMemoryQueue(context.Background(), slog.Default())
	require.NoError(t, err)
	require.NoError(t, q.Start(context.Background()))
	t.Cleanup(func() { closeJobQueue(q) })
	return q
}

type probeKey struct{}

// TestAutomationJobs_ForegroundIdentity: the run carries the declared
// identity, not the saver's, records the saver in the audit label, and
// starts from a fresh context.
func TestAutomationJobs_ForegroundIdentity(t *testing.T) {
	for _, tc := range []struct{ runAs, want string }{
		{"", principal.UserAutomation},
		{"bot", "bot"},
	} {
		t.Run(tc.want, func(t *testing.T) {
			r := &fakeJobRunner{during: func(ctx context.Context, _ string) {
				require.Nil(t, ctx.Value(probeKey{}), "the saver's context reached the job")
			}}
			a := newTestJobs(t, jobsMeta(tc.runAs), nil, r)
			ctx := principal.With(context.Background(), principal.Principal{User: "alice"})
			ctx = context.WithValue(ctx, probeKey{}, "request state")
			require.NoError(t, a.EnqueueScript(ctx, note1))
			require.Equal(t, []principal.Principal{{User: tc.want, Tool: principal.ToolAutomationJob}}, r.runs)
			require.Equal(t, []string{"automation-job:push;by=alice"}, r.labels)
		})
	}
}

// TestAutomationJobs_ForegroundErrorIsGeneric: the saver learns that the
// action failed, not what the script said.
func TestAutomationJobs_ForegroundErrorIsGeneric(t *testing.T) {
	a := newTestJobs(t, jobsMeta(""), nil, &fakeJobRunner{})
	a.runner = failingRunner{&fakeJobRunner{}}
	err := a.EnqueueScript(context.Background(), note1)
	require.EqualError(t, err, `automation "push": background action failed`)
}

type failingRunner struct{ *fakeJobRunner }

func (failingRunner) ExecuteFile(context.Context, string, lua.WriteDeps, *entity.Entity) error {
	return errors.New("secret value 42")
}

// TestAutomationJobs_SaveDuringRunRunsAgain: a save while the script runs
// collapses into the running job and makes it run once more (C1).
func TestAutomationJobs_SaveDuringRunRunsAgain(t *testing.T) {
	r := &fakeJobRunner{started: make(chan struct{}), gate: make(chan struct{})}
	a := newTestJobs(t, jobsMeta(""), startedQueue(t), r)

	require.NoError(t, a.EnqueueScript(context.Background(), note1))
	<-r.started
	// Two saves while the job runs: both collapse into it.
	require.NoError(t, a.EnqueueScript(context.Background(), note1))
	require.NoError(t, a.EnqueueScript(context.Background(), note1))
	close(r.gate)
	ctx := context.Background()
	require.Eventually(t, func() bool {
		tok, _ := a.token(ctx, note1Key())
		done, _ := a.handled(ctx, note1Key())
		return tok == done
	}, 5*time.Second, 5*time.Millisecond, "the last save is handled")
	// Follow-ups stop once the token is handled; give them their full span.
	time.Sleep(time.Duration(followUpAttempts*(followUpAttempts+1)/2) * a.followUpDelay * 2)
	require.Equal(t, 2, r.count(), "one rerun covers every save made during the run")
}

func note1Key() string {
	return automationJobPayload{Automation: "push", LuaFile: "push.lua", ID: "NOTE-1"}.key()
}

// dupQueue reports the first enqueue as a duplicate, as the queue does while
// a job that has already returned waits for its completion to be recorded.
type dupQueue struct {
	mu   sync.Mutex
	dups int
	jobs []jobs.Job
}

func (q *dupQueue) Register(jobs.Kind, jobs.Handler) error { return nil }

func (q *dupQueue) Enqueue(_ context.Context, j jobs.Job) error {
	q.mu.Lock()
	defer q.mu.Unlock()
	if q.dups > 0 {
		q.dups--
		return jobs.ErrDuplicateJob
	}
	q.jobs = append(q.jobs, j)
	return nil
}

func (q *dupQueue) enqueued() int {
	q.mu.Lock()
	defer q.mu.Unlock()
	return len(q.jobs)
}

// TestAutomationJobs_CollapsedSaveIsFollowedUp: a save that collapses into a
// finishing job is enqueued again once the key is free (S1), unless a run
// has handled it by then.
func TestAutomationJobs_CollapsedSaveIsFollowedUp(t *testing.T) {
	t.Run("unhandled", func(t *testing.T) {
		q := &dupQueue{dups: 2}
		a := newTestJobs(t, jobsMeta(""), q, &fakeJobRunner{})
		require.NoError(t, a.EnqueueScript(context.Background(), note1))
		require.Eventually(t, func() bool { return q.enqueued() == 1 }, 5*time.Second, time.Millisecond)
	})
	t.Run("handled", func(t *testing.T) {
		q := &dupQueue{dups: 1}
		a := newTestJobs(t, jobsMeta(""), q, &fakeJobRunner{})
		a.followUpDelay = 20 * time.Millisecond
		require.NoError(t, a.EnqueueScript(context.Background(), note1))
		tok, err := a.token(context.Background(), note1Key())
		require.NoError(t, err)
		require.NoError(t, a.kv.Put(context.Background(), handledKey(note1Key()), []byte(tok)))
		time.Sleep(100 * time.Millisecond)
		require.Zero(t, q.enqueued())
	})
}

// TestAutomationJobs_OwnWriteDoesNotReschedule: a trigger from inside the
// running job's own write is ignored (no loop).
func TestAutomationJobs_OwnWriteDoesNotReschedule(t *testing.T) {
	var a *automationJobs
	r := &fakeJobRunner{during: func(ctx context.Context, _ string) {
		require.NoError(t, a.EnqueueScript(ctx, note1))
	}}
	a = newTestJobs(t, jobsMeta(""), nil, r)
	require.NoError(t, a.EnqueueScript(context.Background(), note1))
	require.Equal(t, 1, r.count())
}

// TestAutomationJobs_ForegroundJobsTriggeringEachOther: two actions whose
// writes trigger each other run once each in the foreground, without
// deadlocking (code review C1).
func TestAutomationJobs_ForegroundJobsTriggeringEachOther(t *testing.T) {
	var a *automationJobs
	r := &fakeJobRunner{during: func(ctx context.Context, path string) {
		other := map[string]string{"a.lua": "b.lua", "b.lua": "a.lua"}[path]
		require.NoError(t, a.EnqueueScript(ctx, scriptFor(other)))
	}}
	a = newTestJobs(t, jobsMeta("", "a.lua", "b.lua"), nil, r)
	done := make(chan struct{})
	go func() {
		defer close(done)
		assert.NoError(t, a.EnqueueScript(context.Background(), scriptFor("a.lua")))
	}()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("deadlock")
	}
	require.Equal(t, []string{"a.lua", "b.lua"}, r.files)
}

// TestAutomationJobs_QueuedChainStops: queued jobs triggering each other
// stop at the hop limit. A trigger that collapses into a running job must
// not restart the count (BUG-WKL0M2).
func TestAutomationJobs_QueuedChainStops(t *testing.T) {
	var a *automationJobs
	r := &fakeJobRunner{during: func(ctx context.Context, path string) {
		other := map[string]string{"a.lua": "b.lua", "b.lua": "a.lua"}[path]
		_ = a.EnqueueScript(ctx, scriptFor(other)) // refused at the limit
	}}
	a = newTestJobs(t, jobsMeta("", "a.lua", "b.lua"), startedQueue(t), r)
	require.NoError(t, a.EnqueueScript(context.Background(), scriptFor("a.lua")))
	require.Eventually(t, func() bool { return r.count() == maxAutomationJobHops }, 5*time.Second, 5*time.Millisecond)
	// Outlast every follow-up, which could still queue one more run.
	time.Sleep(50*time.Millisecond + time.Duration(followUpAttempts*(followUpAttempts+1)/2)*a.followUpDelay)
	r.mu.Lock()
	defer r.mu.Unlock()
	want := make([]int, maxAutomationJobHops)
	for i := range want {
		want[i] = i + 1
	}
	require.Equal(t, want, r.hops, "files run: %v", r.files)
}

// TestAutomationJobs_UnhandledTriggerKeepsHops: a later trigger cannot
// lower the hop count of a trigger no run has handled yet (BUG-WKL0M2).
func TestAutomationJobs_UnhandledTriggerKeepsHops(t *testing.T) {
	ctx := context.Background()
	a := newTestJobs(t, jobsMeta(""), startedQueue(t), &fakeJobRunner{})
	key := note1Key()
	_, err := a.recordTrigger(ctx, key, 7)
	require.NoError(t, err)
	tok, err := a.recordTrigger(ctx, key, 1)
	require.NoError(t, err)
	require.Equal(t, 7, tokenHops(string(tok)), "unhandled: the chain's count stays")

	require.NoError(t, a.kv.Put(ctx, handledKey(key), tok))
	tok, err = a.recordTrigger(ctx, key, 1)
	require.NoError(t, err)
	require.Equal(t, 1, tokenHops(string(tok)), "handled: a new trigger starts its own count")
}

func TestTokenHops(t *testing.T) {
	for _, tc := range []struct {
		token string
		want  int
	}{
		{string(newToken(3)), 3},
		{"0123abcd", 0}, // written before tokens carried a count
		{"0123abcd/x", 0},
		{"0123abcd/", 0},
	} {
		t.Run(tc.token, func(t *testing.T) {
			require.Equal(t, tc.want, tokenHops(tc.token))
		})
	}
}

// TestAutomationJobs_HopLimit: a chain of jobs triggering jobs stops.
func TestAutomationJobs_HopLimit(t *testing.T) {
	a := newTestJobs(t, jobsMeta(""), nil, &fakeJobRunner{})
	ctx := context.WithValue(context.Background(), jobHopsKey{}, maxAutomationJobHops)
	require.ErrorContains(t, a.EnqueueScript(ctx, note1), "triggered each other")
}

// TestAutomationJobs_LocksAreReleased: the per-key lock map does not grow.
func TestAutomationJobs_LocksAreReleased(t *testing.T) {
	a := newTestJobs(t, jobsMeta(""), nil, &fakeJobRunner{})
	unlock := a.lock("k")
	require.Len(t, a.locks, 1)
	unlock()
	require.Empty(t, a.locks)
}

// TestAutomationJobs_PayloadSelectsOnly: a job whose action is gone, whose
// entity cannot be read, or whose payload is incomplete ends without
// running and without an error to retry.
func TestAutomationJobs_PayloadSelectsOnly(t *testing.T) {
	r := &fakeJobRunner{}
	a := newTestJobs(t, jobsMeta(""), nil, r)
	for name, payload := range map[string]map[string]any{
		"unknown automation": {"automation": "other", "lua_file": "push.lua", "id": "NOTE-1"},
		"other file":         {"automation": "push", "lua_file": "evil.lua", "id": "NOTE-1"},
		"unreadable entity":  {"automation": "push", "lua_file": "push.lua", "id": "NOTE-2"},
		"incomplete":         {"automation": "push"},
	} {
		require.NoError(t, a.handle(context.Background(), jobs.Job{Payload: payload}), name)
	}
	require.Zero(t, r.count())
}

// TestAutomationJobs_PayloadRoundTrip: the payload survives a JSON trip,
// which turns the hop count into a float64.
func TestAutomationJobs_PayloadRoundTrip(t *testing.T) {
	p := automationJobPayload{Automation: "push", LuaFile: "push.lua", ID: "NOTE-1", Face: "draft", Hops: 3, By: "alice"}
	m := p.toMap()
	m["hops"] = float64(3)
	got, err := payloadFromMap(m)
	require.NoError(t, err)
	require.Equal(t, p, got)
}

func TestAutomationJobs_Retry(t *testing.T) {
	require.Equal(t, jobs.RetryBounded, jobRetry(metamodel.JobRetryDefault))
	require.Equal(t, jobs.RetryNever, jobRetry(metamodel.JobRetryNever))
	require.Equal(t, jobs.RetryPersistent, jobRetry(metamodel.JobRetryPersistent))
}

// TestAutomationJobs_UnboundRefuses: a trigger before bind fails loudly.
func TestAutomationJobs_UnboundRefuses(t *testing.T) {
	a, err := newAutomationJobs(jobsMeta(""), &mapKV{m: map[string][]byte{}}, noEntities{}, nil)
	require.NoError(t, err)
	require.Error(t, a.EnqueueScript(context.Background(), note1))
	_, err = newAutomationJobs(nil, nil, nil, nil)
	require.Error(t, err)
}
