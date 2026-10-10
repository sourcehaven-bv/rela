package kvpiles

import (
	"context"
	"fmt"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/piles"
	"github.com/Sourcehaven-BV/rela/internal/piles/pilestest"
	"github.com/Sourcehaven-BV/rela/internal/state"
	"github.com/Sourcehaven-BV/rela/internal/storage"
)

var created = time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC)

// newKV returns an in-memory state.KV, so a test can open a second Store
// over the same storage and prove durability.
func newKV(t *testing.T) state.KV {
	t.Helper()
	mem := storage.NewMemFS()
	require.NoError(t, mem.MkdirAll("/root", 0o755))
	rfs, err := storage.NewRootedFS(mem, "/root")
	require.NoError(t, err)
	return state.NewFSKV(rfs)
}

func TestConformance(t *testing.T) {
	t.Parallel()
	pilestest.RunAll(t, func(t *testing.T) piles.Store {
		t.Helper()
		s, err := New(newKV(t), ProcessPrivate{})
		require.NoError(t, err)
		return s
	})
}

func TestNew_RejectsNil(t *testing.T) {
	t.Parallel()
	_, err := New(nil, ProcessPrivate{})
	require.Error(t, err)
	_, err = New(newKV(t), nil)
	require.Error(t, err)
	_, err = NewFileLocker("")
	require.Error(t, err)
}

// newDiskStore opens a Store over the on-disk KV in dir with a FileLocker on
// the lock file there, as appbuild does for one process.
func newDiskStore(t *testing.T, dir string) *Store {
	t.Helper()
	rfs, err := storage.NewRootedFS(storage.NewSafeFS(storage.NewOsFS()), dir)
	require.NoError(t, err)
	locker, err := NewFileLocker(filepath.Join(dir, "piles.lock"))
	require.NoError(t, err)
	s, err := New(state.NewFSKV(rfs), locker)
	require.NoError(t, err)
	return s
}

// Two Stores over one on-disk document stand in for two processes: each has
// its own mutex, so only the file lock keeps their writes from overwriting
// each other.
func TestFileLocker_TwoStoresLoseNothing(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	dir := t.TempDir()
	stores := []*Store{newDiskStore(t, dir), newDiskStore(t, dir)}
	p := piles.Pile{ID: "PIL-AAAA1111", Owner: "alice", Name: "Busy", Icon: "star", Created: created, Updated: created}
	require.NoError(t, stores[0].CreatePile(ctx, p, nil, piles.MaxPiles, piles.MaxItems))

	const perStore = 25
	var wg sync.WaitGroup
	errs := make(chan error, 2*perStore)
	for si, s := range stores {
		for i := range perStore {
			wg.Go(func() {
				ref := entity.Ref{ID: fmt.Sprintf("R-%d-%d", si, i), Face: entity.Face("main")}
				_, err := s.AddItems(ctx, "alice", p.ID, []entity.Ref{ref}, created, piles.MaxItems, piles.EvictOldest)
				errs <- err
			})
		}
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		require.NoError(t, err)
	}
	got, err := stores[1].GetPile(ctx, "alice", p.ID)
	require.NoError(t, err)
	require.Len(t, got.Items, 2*perStore, "no add may be lost between the two stores")
}

func TestFileLocker_WaitEndsWithContext(t *testing.T) {
	t.Parallel()
	path := filepath.Join(t.TempDir(), "piles.lock")
	holder, err := NewFileLocker(path)
	require.NoError(t, err)
	unlock, err := holder.Lock(context.Background())
	require.NoError(t, err)
	defer unlock()

	waiter, err := NewFileLocker(path)
	require.NoError(t, err)
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	_, err = waiter.Lock(ctx)
	require.ErrorIs(t, err, context.DeadlineExceeded, "a held lock must not be taken twice")
}

// The conformance suite holds one Store per subtest, so it cannot show that
// piles outlive the process. This test reopens over the same storage.
func TestDurability_SurvivesReopen(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	kv := newKV(t)
	item := entity.Ref{ID: "TKT-1", Face: entity.Face("draft")}

	first, err := New(kv, ProcessPrivate{})
	require.NoError(t, err)
	p := piles.Pile{ID: "PIL-AAAA1111", Owner: "alice", Name: "Inbox", Icon: "star", Created: created, Updated: created}
	require.NoError(t, first.CreatePile(ctx, p, []entity.Ref{item}, piles.MaxPiles, piles.MaxItems))

	second, err := New(kv, ProcessPrivate{})
	require.NoError(t, err)
	got, err := second.GetPile(ctx, "alice", p.ID)
	require.NoError(t, err, "a pile must survive a restart")
	require.Equal(t, "Inbox", got.Name)
	require.Equal(t, "star", got.Icon)
	require.True(t, got.Created.Equal(created))
	require.Len(t, got.Items, 1)
	require.Equal(t, item, got.Items[0].Ref, "the face must survive a restart")
}

// A corrupt document must fail loudly rather than read as empty: the next
// write would otherwise replace every user's piles with one new pile.
func TestCorruptDocument_IsAnError(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	kv := newKV(t)
	require.NoError(t, kv.Put(ctx, StateKey, []byte("{ not json")))

	s, err := New(kv, ProcessPrivate{})
	require.NoError(t, err)
	_, err = s.ListPiles(ctx, "alice")
	require.Error(t, err)
	err = s.CreatePile(ctx, piles.Pile{ID: "PIL-AAAA1111", Owner: "alice", Name: "x", Icon: "star"},
		nil, piles.MaxPiles, piles.MaxItems)
	require.Error(t, err)

	data, err := kv.Get(ctx, StateKey)
	require.NoError(t, err)
	require.Equal(t, "{ not json", string(data), "a failed write must leave the document untouched")
}
