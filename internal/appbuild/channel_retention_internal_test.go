package appbuild

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/audit"
	"github.com/Sourcehaven-BV/rela/internal/channels"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/entitymanager"
	"github.com/Sourcehaven-BV/rela/internal/realtime"
	"github.com/Sourcehaven-BV/rela/internal/realtimelimits"
	"github.com/Sourcehaven-BV/rela/internal/statemachine"
	"github.com/Sourcehaven-BV/rela/internal/storage"
	"github.com/Sourcehaven-BV/rela/internal/store/memstore"
)

// batchesOf returns the tombstone batches of id filed so far.
func batchesOf(t *testing.T, cs channels.Store, id string) []channels.TombstoneBatch {
	t.Helper()
	all, err := cs.Tombstones(context.Background(), time.Now().Add(time.Hour))
	require.NoError(t, err)
	var out []channels.TombstoneBatch
	for _, b := range all {
		if b.Handle.Anchor().ID == id {
			out = append(out, b)
		}
	}
	return out
}

func pendingCount(c *tombstoneConfirmer) int {
	c.mu.Lock()
	defer c.mu.Unlock()
	n := 0
	for _, hs := range c.pending {
		n += len(hs)
	}
	return n
}

func woke(c *tombstoneConfirmer) bool {
	select {
	case <-c.wake:
		return true
	default:
		return false
	}
}

// The guard files its batch as pending with the policy and purge time of
// the config; an undo forgets it.
func TestChannelGuard_FilesPendingBatch(t *testing.T) {
	cases := []struct {
		name   string
		typ    string
		policy channels.TombstonePolicy
		grace  time.Duration
	}{
		{"delete type erases at once", "purge-me", channels.PolicyDelete, 0},
		{"detach type waits", "chat", channels.PolicyDetach, realtimelimits.DetachedRetention},
		{"undeclared type waits", "gone-type", channels.PolicyDetach, realtimelimits.DetachedRetention},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ctx := t.Context()
			f := newGuardFixture(t)
			appendTo(t, f.cs, chanKey("INC-1", "", tc.typ))
			undo, err := guardDelete(ctx, f.guard, entitymanager.AnchorDelete{ID: "INC-1", Block: true, Detach: true})
			require.NoError(t, err)
			require.NotNil(t, undo)

			got := batchesOf(t, f.cs, "INC-1")
			require.Len(t, got, 1)
			assert.True(t, got[0].Filing.Pending)
			assert.Equal(t, tc.policy, got[0].Filing.Policy)
			assert.Equal(t, got[0].At.Add(tc.grace), got[0].Filing.PurgeAfter)
			assert.Equal(t, 1, pendingCount(f.guard.tombs))

			require.NoError(t, undo.Restore(ctx))
			assert.Zero(t, pendingCount(f.guard.tombs), "a restored batch is forgotten")
			assert.Empty(t, batchesOf(t, f.cs, "INC-1"))
		})
	}
}

// A detach channel created after the guard listed the entity is filed
// with its own retention, not the delete type's immediate purge.
func TestChannelGuard_LateChannelKeepsItsRetention(t *testing.T) {
	ctx := t.Context()
	f := newGuardFixture(t)
	appendTo(t, f.cs, chanKey("INC-1", "", "purge-me"))
	d := entitymanager.AnchorDelete{ID: "INC-1", Block: true, Detach: true}
	s, err := f.guard.Scope(ctx, []string{d.ID})
	require.NoError(t, err)

	// Created between the guard's list and its tombstone.
	appendTo(t, f.cs, chanKey("INC-1", "", "chat"))
	undo, err := s.BeforeDelete(ctx, d)
	require.NoError(t, err)
	require.NotNil(t, undo)

	got := batchesOf(t, f.cs, "INC-1")
	require.Len(t, got, 1)
	assert.Len(t, got[0].Channels, 2)
	assert.Equal(t, got[0].At.Add(realtimelimits.DetachedRetention), got[0].Filing.PurgeAfter,
		"the late chat channel keeps the batch for the detached retention")
}

// A delete through the manager confirms the guard's batch after commit and
// wakes the worker. A failed delete restores the channels and leaves
// nothing pending.
func TestChannelGuard_ManagerDeleteConfirms(t *testing.T) {
	ctx := context.Background()
	f := newGuardFixture(t)
	mgr := f.manager(t, f.guard)
	f.seed(t, "inc-a", "draft")
	appendTo(t, f.cs, chanKey("inc-a", "", "purge-me"))
	appendTo(t, f.cs, chanKey("inc-a", "", "chat"))

	_, err := mgr.DeleteEntity(ctx, "inc-a", true)
	require.NoError(t, err)
	got := batchesOf(t, f.cs, "inc-a")
	require.Len(t, got, 1)
	assert.False(t, got[0].Filing.Pending, "the post-commit hook confirmed it")
	assert.Equal(t, channels.PolicyDelete, got[0].Filing.Policy)
	assert.Equal(t, got[0].At.Add(realtimelimits.DetachedRetention), got[0].Filing.PurgeAfter,
		"the detach channel in the batch keeps it for the detached retention")
	assert.Zero(t, pendingCount(f.guard.tombs))
	assert.True(t, woke(f.guard.tombs))

	f.seed(t, "inc-b", "draft")
	appendTo(t, f.cs, chanKey("inc-b", "", "purge-me"))
	failing, err := entitymanager.New(entitymanager.Deps{
		Store: failingTxStore{f.st}, Meta: f.meta, Templater: gcNopTemplater{}, Audit: audit.Nop{},
		ACL: acl.NopACL{}, Transitions: statemachine.EmptySet(), FieldGate: entitymanager.AllowAllFieldGate{},
		AnchorGuard: f.guard,
	})
	require.NoError(t, err)
	_, err = failing.DeleteEntity(ctx, "inc-b", true)
	require.ErrorIs(t, err, errDiskFull)
	assert.Equal(t, []string{"inc-b/purge-me"}, entityKeys(t, f.cs, "inc-b"))
	assert.Zero(t, pendingCount(f.guard.tombs))
}

// A face delete confirms only that face's batch.
func TestChannelGuard_FaceDeleteConfirmsFace(t *testing.T) {
	ctx := context.Background()
	f := newGuardFixture(t)
	mgr := f.manager(t, f.guard)
	f.seed(t, "inc-a", "draft", "final")
	appendTo(t, f.cs, chanKey("inc-a", "draft", "chat"))
	// A delete of the final face is in flight: its batch is pending.
	appendTo(t, f.cs, chanKey("inc-a", "final", "chat"))
	other, err := f.cs.Tombstone(ctx, "inc-a", "final", time.Now(),
		realtime.Filer(f.cfg, channels.PolicyDelete, time.Now(), true))
	require.NoError(t, err)
	require.False(t, other.IsZero())
	f.guard.tombs.add(other)

	_, err = mgr.DeleteEntityFace(ctx, "inc-a", "draft", true)
	require.NoError(t, err)
	byFace := map[entity.Face]channels.TombstoneBatch{}
	for _, b := range batchesOf(t, f.cs, "inc-a") {
		byFace[b.Handle.Anchor().Face] = b
	}
	require.Len(t, byFace, 2)
	assert.False(t, byFace["draft"].Filing.Pending, "the deleted face's batch is confirmed")
	assert.True(t, byFace["final"].Filing.Pending, "the other face's batch stays pending")
	assert.Equal(t, 1, pendingCount(f.guard.tombs), "the other face's batch is still remembered")
}

// A channel of a blocking type created after Scope listed the entity
// refuses the delete, and nothing is moved.
func TestChannelGuard_LateBlockingChannelRefuses(t *testing.T) {
	ctx := t.Context()
	f := newGuardFixture(t)
	appendTo(t, f.cs, chanKey("INC-1", "", "chat"))
	d := entitymanager.AnchorDelete{ID: "INC-1", Block: true, Detach: true}
	s, err := f.guard.Scope(ctx, []string{d.ID})
	require.NoError(t, err)

	appendTo(t, f.cs, chanKey("INC-1", "", "war-room"))
	_, err = s.BeforeDelete(ctx, d)
	var blocked *entitymanager.AnchorBlockedError
	require.ErrorAs(t, err, &blocked)
	assert.Equal(t, "war-room", blocked.ChannelType)
	assert.Empty(t, batchesOf(t, f.cs, "INC-1"))
	assert.Zero(t, pendingCount(f.guard.tombs))
	assert.Len(t, entityKeys(t, f.cs, "INC-1"), 2)
}

// A soft delete tombstones nothing: a restore must find the channels where
// they were.
func TestChannelGuard_SoftDeleteKeepsChannels(t *testing.T) {
	ctx := context.Background()
	f := newGuardFixture(t)
	mgr := f.manager(t, f.guard)
	require.True(t, entitymanager.SupportsSoftDelete(mgr))
	f.seed(t, "inc-a", "draft")
	appendTo(t, f.cs, chanKey("inc-a", "", "purge-me"))

	_, err := entitymanager.SoftDeleteEntity(ctx, mgr, "inc-a")
	require.NoError(t, err)
	assert.Equal(t, []string{"inc-a/purge-me"}, entityKeys(t, f.cs, "inc-a"))
	assert.Empty(t, batchesOf(t, f.cs, "inc-a"))

	// The orphan check counts the marked family as existing.
	faces, err := anchorExistence{st: f.st}.Faces(ctx, []string{"inc-a", "inc-z"})
	require.NoError(t, err)
	assert.Equal(t, map[string][]entity.Face{"inc-a": {"draft"}}, faces)
}

func TestAnchorExistence(t *testing.T) {
	ctx := context.Background()
	st := memstore.New()
	for _, e := range []*entity.Entity{
		{ID: "T-1", Type: "task"},
		{ID: "P-1", Type: "page", Face: "draft"},
		{ID: "P-1", Type: "page", Face: "final"},
	} {
		require.NoError(t, st.CreateEntity(ctx, e))
	}
	got, err := anchorExistence{st: st}.Faces(ctx, []string{"T-1", "P-1", "GONE-1"})
	require.NoError(t, err)
	assert.Equal(t, []entity.Face{""}, got["T-1"])
	assert.ElementsMatch(t, []entity.Face{"draft", "final"}, got["P-1"])
	assert.NotContains(t, got, "GONE-1")
}

func TestTombstoneConfirmer(t *testing.T) {
	_, err := newTombstoneConfirmer(nil)
	require.Error(t, err)

	ctx := context.Background()
	_, cs, _ := newChannelAliases(t)
	c, err := newTombstoneConfirmer(cs)
	require.NoError(t, err)
	appendTo(t, cs, chanKey("A-1", "draft", "room"))
	appendTo(t, cs, chanKey("A-1", "final", "room"))
	draft, err := cs.Tombstone(ctx, "A-1", "draft", time.Now(), channels.FileAs(channels.Filing{Pending: true}))
	require.NoError(t, err)
	final, err := cs.Tombstone(ctx, "A-1", "final", time.Now(), channels.FileAs(channels.Filing{Pending: true}))
	require.NoError(t, err)
	c.add(draft)
	c.add(final)

	require.NoError(t, c.confirm(ctx, "A-1", "draft", false))
	assert.Equal(t, 1, pendingCount(c))
	assert.True(t, woke(c))
	require.NoError(t, c.confirm(ctx, "B-1", "", true), "nothing pending for another id")
	assert.False(t, woke(c))
	require.NoError(t, c.confirm(ctx, "A-1", "", true))
	assert.Zero(t, pendingCount(c))
	for _, b := range batchesOf(t, cs, "A-1") {
		assert.False(t, b.Filing.Pending)
	}

	// A batch already gone is not an error.
	require.NoError(t, cs.Untombstone(ctx, draft, time.Now()))
	c.add(draft)
	require.NoError(t, c.confirm(ctx, "A-1", "", true))
}

// The worker lock of one directory is held by one process at a time: a
// second taker is refused at once, and takes it once the first lets go.
func TestRealtimeLeaderLock(t *testing.T) {
	st := memstore.New()
	l, err := realtimeLeaderLock(nil, "", st)
	require.NoError(t, err)
	release, ok, err := l(t.Context())
	require.NoError(t, err)
	require.True(t, ok)
	release()

	dir := t.TempDir()
	a, err := realtimeLeaderLock(storage.NewSafeFS(storage.NewOsFS()), dir, st)
	require.NoError(t, err)
	b, err := realtimeLeaderLock(storage.NewSafeFS(storage.NewOsFS()), dir, st)
	require.NoError(t, err)
	releaseA, ok, err := a(t.Context())
	require.NoError(t, err)
	require.True(t, ok)
	assert.FileExists(t, dir+"/"+realtimeWorkerLockFile)
	_, ok, err = b(t.Context())
	require.NoError(t, err)
	assert.False(t, ok, "a second process does not take a held lock")
	releaseA()
	releaseB, ok, err := b(t.Context())
	require.NoError(t, err)
	assert.True(t, ok, "the lock passes on once released")
	releaseB()
}

// switchLock is a worker lock another process may hold. It counts the runs
// it granted and refused.
type switchLock struct {
	mu       sync.Mutex
	other    bool
	err      error
	granted  int
	refused  int
	attempts int
}

func (l *switchLock) take(context.Context) (func(), bool, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.attempts++
	if l.err != nil {
		return nil, false, l.err
	}
	if l.other {
		l.refused++
		return nil, false, nil
	}
	l.granted++
	return func() {}, true, nil
}

func (l *switchLock) set(other bool, err error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.other, l.err = other, err
}

func (l *switchLock) counts() (granted, refused, attempts int) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.granted, l.refused, l.attempts
}

// workerFixture is a worker over a file channel store, with a sweeper that
// erases a due batch.
type workerFixture struct {
	cs    channels.Store
	tombs *tombstoneConfirmer
	w     *realtimeWorker
	lock  *switchLock
}

func newWorkerFixture(t *testing.T) workerFixture {
	t.Helper()
	_, cs, st := newChannelAliases(t)
	tombs, err := newTombstoneConfirmer(cs)
	require.NoError(t, err)
	local, err := realtime.NewLocal(time.Now)
	require.NoError(t, err)
	sweeper, err := realtime.NewSweeper(realtime.SweepDeps{
		Store: cs, Config: realtimeConfigSource{}, Anchors: anchorExistence{st: st},
		Marker: local, Clock: time.Now,
	})
	require.NoError(t, err)
	l := &switchLock{}
	w, err := newRealtimeWorker(sweeper, l.take, tombs.wake)
	require.NoError(t, err)
	return workerFixture{cs: cs, tombs: tombs, w: w, lock: l}
}

// fileDue files a confirmed batch for id that is already due.
func (f workerFixture) fileDue(ctx context.Context, t *testing.T, id string) {
	t.Helper()
	past := time.Now().Add(-time.Minute)
	appendCtx(ctx, t, f.cs, chanKey(id, "", "room"))
	due := channels.Filing{Policy: channels.PolicyDelete, PurgeAfter: past}
	_, err := f.cs.Tombstone(ctx, id, "", past, channels.FileAs(due))
	require.NoError(t, err)
}

// The worker purges a due batch on its first run and again on a wake, and
// stop ends it.
func TestRealtimeWorker_RunsAndStops(t *testing.T) {
	ctx := context.Background()
	f := newWorkerFixture(t)
	_, err := newRealtimeWorker(nil, nil, nil)
	require.Error(t, err)
	f.fileDue(ctx, t, "GONE-1")

	s := &Services{realtimeWorker: f.w}
	done := RunRealtimeWorker(ctx, s)
	require.Eventually(t, func() bool { return len(batchesOf(t, f.cs, "GONE-1")) == 0 },
		5*time.Second, 10*time.Millisecond, "the first run purges")
	select {
	case <-RunRealtimeWorker(ctx, s):
	default:
		t.Fatal("a second run returns at once")
	}

	f.fileDue(ctx, t, "GONE-2")
	f.tombs.ping()
	require.Eventually(t, func() bool { return len(batchesOf(t, f.cs, "GONE-2")) == 0 },
		5*time.Second, 10*time.Millisecond, "a wake purges")

	f.w.stop()
	<-done
	f.w.stop()

	select {
	case <-RunRealtimeWorker(ctx, &Services{}):
	default:
		t.Fatal("no worker returns a closed channel")
	}
}

// While another process holds the lock, every run is skipped at once and
// erases nothing; once that process lets go, the next run takes over.
func TestRealtimeWorker_SkipsWhileHeldElsewhereThenTakesOver(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	f := newWorkerFixture(t)
	f.lock.set(true, nil)
	f.fileDue(ctx, t, "GONE-1")

	done := make(chan struct{})
	go func() { f.w.run(ctx); close(done) }()
	require.Eventually(t, func() bool { _, refused, _ := f.lock.counts(); return refused == 1 },
		5*time.Second, 10*time.Millisecond, "the start run is refused")
	f.tombs.ping()
	require.Eventually(t, func() bool { _, refused, _ := f.lock.counts(); return refused == 2 },
		5*time.Second, 10*time.Millisecond, "a wake run is refused, not queued behind the lock")
	assert.Len(t, batchesOf(t, f.cs, "GONE-1"), 1, "a refused run erases nothing")

	f.lock.set(false, errDiskFull)
	f.tombs.ping()
	require.Eventually(t, func() bool { _, _, attempts := f.lock.counts(); return attempts == 3 },
		5*time.Second, 10*time.Millisecond)
	assert.Len(t, batchesOf(t, f.cs, "GONE-1"), 1, "a lock error skips the run")

	// The holder stopped: the next run takes over.
	f.lock.set(false, nil)
	f.tombs.ping()
	require.Eventually(t, func() bool { return len(batchesOf(t, f.cs, "GONE-1")) == 0 },
		5*time.Second, 10*time.Millisecond, "the next run takes over")
	granted, _, _ := f.lock.counts()
	assert.Equal(t, 1, granted)
	cancel()
	<-done
}
