package kvuserstate

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/state"
	"github.com/Sourcehaven-BV/rela/internal/storage"
	"github.com/Sourcehaven-BV/rela/internal/userstate"
	"github.com/Sourcehaven-BV/rela/internal/userstate/userstatetest"
)

var base = time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC)

// newKV returns an in-memory state.KV plus its backing FS, so a test can
// reopen a second Store over the SAME storage and prove durability.
func newKV(t *testing.T) state.KV {
	t.Helper()
	mem := storage.NewMemFS()
	require.NoError(t, mem.MkdirAll("/root", 0o755))
	rfs, err := storage.NewRootedFS(mem, "/root")
	require.NoError(t, err)
	return state.NewFSKV(rfs)
}

// TestConformance runs the shared contract. Every backend must pass it —
// three implementations without one are three subtly different behaviors.
func TestConformance(t *testing.T) {
	t.Parallel()
	userstatetest.RunAll(t, func(t *testing.T) userstate.Store {
		t.Helper()
		s, err := New(newKV(t))
		require.NoError(t, err)
		t.Cleanup(func() { _ = s.Close() })
		return s
	})
}

func TestNew_RejectsNilKV(t *testing.T) {
	t.Parallel()
	_, err := New(nil)
	require.Error(t, err,
		"a nil KV would silently degrade to forgetting snoozes while claiming to persist them")
}

// The whole point of this backend: state must outlive the process. The
// conformance suite cannot express this — it holds one Store per subtest.
func TestDurability_SurvivesReopen(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	kv := newKV(t)
	key := userstate.Key{User: "alice", Source: "stale", EntityID: "T-1"}

	first, err := New(kv)
	require.NoError(t, err)
	require.NoError(t, first.SetSnooze(ctx, key, base.Add(24*time.Hour)))
	require.NoError(t, first.SetMuted(ctx, "alice", "quips", true))
	require.NoError(t, first.MarkShown(ctx, key, base))
	require.NoError(t, first.Close())

	// A fresh Store over the same storage — the restart case.
	second, err := New(kv)
	require.NoError(t, err)
	t.Cleanup(func() { _ = second.Close() })

	until, ok, err := second.SnoozedUntil(ctx, key, base)
	require.NoError(t, err)
	require.True(t, ok, "a snooze must survive a restart")
	require.True(t, until.Equal(base.Add(24*time.Hour)))

	muted, err := second.Muted(ctx, "alice", "quips")
	require.NoError(t, err)
	require.True(t, muted, "a mute must survive a restart")

	at, ok, err := second.LastShown(ctx, key)
	require.NoError(t, err)
	require.True(t, ok)
	require.True(t, at.Equal(base), "cooldown state must survive a restart")
}

// This state is disposable, so an unreadable document must not stop the app.
// Refusing to serve because a snooze log is corrupt is a worse failure than
// forgetting the snoozes — matching how the scheduler treats its own state.
func TestCorruptDocument_ReadsAsEmpty(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	kv := newKV(t)
	require.NoError(t, kv.Put(ctx, StateKey, []byte("{ this is not json")))

	s, err := New(kv)
	require.NoError(t, err)
	t.Cleanup(func() { _ = s.Close() })

	_, ok, err := s.SnoozedUntil(ctx, userstate.Key{User: "a", Source: "b"}, base)
	require.NoError(t, err, "a corrupt document must not fail the read")
	require.False(t, ok)

	// And it must be writable again rather than staying wedged.
	require.NoError(t, s.SetMuted(ctx, "a", "b", true))
	muted, err := s.Muted(ctx, "a", "b")
	require.NoError(t, err)
	require.True(t, muted)
}

// One user's entries must not be readable as another's — the document is
// shared, so the key must carry the user.
func TestSharedDocument_KeepsUsersDistinct(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	s, err := New(newKV(t))
	require.NoError(t, err)
	t.Cleanup(func() { _ = s.Close() })

	require.NoError(t, s.SetSnooze(ctx,
		userstate.Key{User: "alice", Source: "stale", EntityID: "T-1"}, base.Add(time.Hour)))

	_, ok, err := s.SnoozedUntil(ctx,
		userstate.Key{User: "bob", Source: "stale", EntityID: "T-1"}, base)
	require.NoError(t, err)
	require.False(t, ok, "bob must not inherit alice's snooze from the shared document")
}

// Pruning must actually shrink what is persisted, not just the in-memory
// copy — otherwise the file grows forever and the reopen path reloads rows
// that were supposedly reclaimed.
func TestPrune_ShrinksThePersistedDocument(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	kv := newKV(t)
	s, err := New(kv)
	require.NoError(t, err)

	dead := userstate.Key{User: "alice", Source: "stale", EntityID: "T-dead"}
	require.NoError(t, s.SetSnooze(ctx, dead, base.Add(time.Hour)))

	removed, err := s.Prune(ctx, base.Add(48*time.Hour), 30*24*time.Hour)
	require.NoError(t, err)
	require.Equal(t, 1, removed)
	require.NoError(t, s.Close())

	reopened, err := New(kv)
	require.NoError(t, err)
	t.Cleanup(func() { _ = reopened.Close() })
	_, ok, err := reopened.SnoozedUntil(ctx, dead, base)
	require.NoError(t, err)
	require.False(t, ok, "a pruned snooze must be gone from storage, not just from memory")
}

// Re-muting an already-muted source must not append a duplicate: the mute
// list is rendered to the user as the "what have I turned off?" screen.
func TestSetMuted_NoDuplicates(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	s, err := New(newKV(t))
	require.NoError(t, err)
	t.Cleanup(func() { _ = s.Close() })

	require.NoError(t, s.SetMuted(ctx, "alice", "quips", true))
	require.NoError(t, s.SetMuted(ctx, "alice", "quips", true))

	require.Len(t, s.snapshot().Muted["alice"], 1)
}
