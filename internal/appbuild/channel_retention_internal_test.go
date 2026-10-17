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
	other, err := f.cs.Tombstone(ctx, "inc-a", "final", time.Now(), channels.Filing{Pending: true})
	require.NoError(t, err)
	require.True(t, other.IsZero(), "no final channels yet")

	_, err = mgr.DeleteEntityFace(ctx, "inc-a", "draft", true)
	require.NoError(t, err)
	got := batchesOf(t, f.cs, "inc-a")
	require.Len(t, got, 1)
	assert.Equal(t, entity.Face("draft"), got[0].Handle.Anchor().Face)
	assert.False(t, got[0].Filing.Pending)
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
	draft, err := cs.Tombstone(ctx, "A-1", "draft", time.Now(), channels.Filing{Pending: true})
	require.NoError(t, err)
	final, err := cs.Tombstone(ctx, "A-1", "final", time.Now(), channels.Filing{Pending: true})
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

func TestRealtimeLeaderLock(t *testing.T) {
	st := memstore.New()
	l, err := realtimeLeaderLock(nil, "", st)
	require.NoError(t, err)
	release, err := l(t.Context())
	require.NoError(t, err)
	release()

	dir := t.TempDir()
	l, err = realtimeLeaderLock(storage.NewSafeFS(storage.NewOsFS()), dir, st)
	require.NoError(t, err)
	release, err = l(t.Context())
	require.NoError(t, err)
	assert.FileExists(t, dir+"/"+realtimeWorkerLockFile)
	release()
}

// countingLock fails its first takes, then grants the lead.
type countingLock struct {
	mu    sync.Mutex
	fails int
	takes int
}

func (l *countingLock) take(context.Context) (func(), error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.takes++
	if l.takes <= l.fails {
		return nil, errDiskFull
	}
	return func() {}, nil
}

// The worker purges a due batch on its first lead and again on a wake, and
// stop ends it.
func TestRealtimeWorker_RunsAndStops(t *testing.T) {
	ctx := context.Background()
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

	_, err = newRealtimeWorker(nil, nil, nil)
	require.Error(t, err)
	lock := &countingLock{}
	w, err := newRealtimeWorker(sweeper, lock.take, tombs.wake)
	require.NoError(t, err)

	past := time.Now().Add(-time.Minute)
	file := func(id string) {
		appendTo(t, cs, chanKey(id, "", "room"))
		_, err := cs.Tombstone(ctx, id, "", past, channels.Filing{Policy: channels.PolicyDelete, PurgeAfter: past})
		require.NoError(t, err)
	}
	file("GONE-1")

	s := &Services{realtimeWorker: w}
	done := RunRealtimeWorker(ctx, s)
	require.Eventually(t, func() bool { return len(batchesOf(t, cs, "GONE-1")) == 0 },
		5*time.Second, 10*time.Millisecond, "the first lead purges")
	select {
	case <-RunRealtimeWorker(ctx, s):
	default:
		t.Fatal("a second run returns at once")
	}

	file("GONE-2")
	tombs.ping()
	require.Eventually(t, func() bool { return len(batchesOf(t, cs, "GONE-2")) == 0 },
		5*time.Second, 10*time.Millisecond, "a wake purges")

	w.stop()
	<-done
	w.stop()

	select {
	case <-RunRealtimeWorker(ctx, &Services{}):
	default:
		t.Fatal("no worker returns a closed channel")
	}
}

// A lock that is not taken is retried until ctx ends.
func TestRealtimeWorker_LockRetryEndsWithContext(t *testing.T) {
	lock := &countingLock{fails: 1 << 30}
	w := &realtimeWorker{lock: lock.take, closed: make(chan struct{})}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { w.run(ctx); close(done) }()
	require.Eventually(t, func() bool {
		lock.mu.Lock()
		defer lock.mu.Unlock()
		return lock.takes > 0
	}, 5*time.Second, 10*time.Millisecond)
	cancel()
	<-done
}
