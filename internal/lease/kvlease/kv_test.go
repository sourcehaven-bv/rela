package kvlease

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/filelock"
	"github.com/Sourcehaven-BV/rela/internal/lease"
	"github.com/Sourcehaven-BV/rela/internal/lease/leasetest"
	"github.com/Sourcehaven-BV/rela/internal/state"
	"github.com/Sourcehaven-BV/rela/internal/storage"
	"github.com/Sourcehaven-BV/rela/internal/testutil/logtest"
)

var t0 = time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC)

var alice = lease.Holder{Identity: "alice", User: "Alice", Tool: "mcp"}

// newMemKV returns an in-memory state.KV, so a test can open a second Store
// over the same storage.
func newMemKV(t *testing.T) state.KV {
	t.Helper()
	mem := storage.NewMemFS()
	require.NoError(t, mem.MkdirAll("/root", 0o755))
	rfs, err := storage.NewRootedFS(mem, "/root")
	require.NoError(t, err)
	return state.NewFSKV(rfs)
}

// newDiskStore opens a Store over the on-disk KV in dir with its own file
// lock on the lock file there, as one process would.
func newDiskStore(t *testing.T, dir string) *Store {
	t.Helper()
	rfs, err := storage.NewRootedFS(storage.NewSafeFS(storage.NewOsFS()), dir)
	require.NoError(t, err)
	locker, err := filelock.New(filepath.Join(dir, "leases.lock"))
	require.NoError(t, err)
	s, err := New(state.NewFSKV(rfs), locker)
	require.NoError(t, err)
	return s
}

func TestConformance_MemFS(t *testing.T) {
	t.Parallel()
	leasetest.RunAll(t, func(t *testing.T) lease.Store {
		t.Helper()
		s, err := New(newMemKV(t), filelock.ProcessPrivate{})
		require.NoError(t, err)
		return s
	})
}

func TestConformance_Disk(t *testing.T) {
	t.Parallel()
	leasetest.RunAll(t, func(t *testing.T) lease.Store {
		t.Helper()
		return newDiskStore(t, t.TempDir())
	})
}

func TestNew_RejectsNil(t *testing.T) {
	t.Parallel()
	_, err := New(nil, filelock.ProcessPrivate{})
	require.Error(t, err)
	_, err = New(newMemKV(t), nil)
	require.Error(t, err)
}

// Two Stores over one directory stand in for two processes: each has its own
// mutex and its own file lock, so only the lock file keeps both from winning.
func TestTwoStores_OneWinner(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	dir := t.TempDir()
	stores := []*Store{newDiskStore(t, dir), newDiskStore(t, dir)}

	const perStore = 20
	var (
		wg   sync.WaitGroup
		mu   sync.Mutex
		wins int
	)
	errs := make(chan error, 2*perStore)
	for si, s := range stores {
		for i := range perStore {
			wg.Go(func() {
				h := lease.Holder{Identity: fmt.Sprintf("u-%d-%d", si, i)}
				hash := leasetest.Hash(fmt.Sprintf("h-%d-%d", si, i))
				_, err := s.Claim(ctx, "TKT-HOT", h, hash, t0, t0.Add(time.Minute), "")
				var held *lease.HeldError
				switch {
				case err == nil:
					mu.Lock()
					wins++
					mu.Unlock()
				case errors.As(err, &held):
				default:
					errs <- err
				}
			})
		}
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		require.NoError(t, err)
	}
	require.Equal(t, 1, wins, "exactly one claim may win across the two stores")
}

// The conformance suite holds one Store per subtest, so it cannot show that
// leases and the token counter outlive the process.
func TestReopen_KeepsLeasesAndToken(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	kv := newMemKV(t)
	first, err := New(kv, filelock.ProcessPrivate{})
	require.NoError(t, err)
	l, err := first.Claim(ctx, "TKT-1", alice, leasetest.Hash("h1"), t0, t0.Add(time.Hour), "")
	require.NoError(t, err)
	require.NoError(t, first.Release(ctx, leasetest.Hash("h1"), alice.Identity, t0))
	l2, err := first.Claim(ctx, "TKT-2", alice, leasetest.Hash("h2"), t0, t0.Add(time.Hour), "")
	require.NoError(t, err)
	require.NoError(t, first.Close())

	second, err := New(kv, filelock.ProcessPrivate{})
	require.NoError(t, err)
	got, err := second.ListByHolder(ctx, alice.Identity, t0)
	require.NoError(t, err)
	require.Equal(t, []lease.Lease{l2}, got, "a lease must survive a restart")

	// Same instant: only the stored counter keeps the token increasing.
	l3, err := second.Claim(ctx, "TKT-1", alice, leasetest.Hash("h3"), t0, t0.Add(time.Hour), "")
	require.NoError(t, err)
	require.Greater(t, l3.Token, l2.Token)
	require.Greater(t, l2.Token, l.Token)
}

// A deleted document loses the counter. The version key, which stays above
// every token issued, keeps tokens increasing even at the same instant, and
// keeps the version moving.
func TestDeletedDocument_TokenAndVersionKeepIncreasing(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	kv := newMemKV(t)
	s, err := New(kv, filelock.ProcessPrivate{})
	require.NoError(t, err)
	for i := range 3 {
		_, err = s.Claim(ctx, fmt.Sprintf("TKT-%d", i), alice, leasetest.Hash(fmt.Sprintf("h%d", i)), t0, t0.Add(time.Hour), "")
		require.NoError(t, err)
	}
	last, err := s.Get(ctx, []string{"TKT-2"}, t0)
	require.NoError(t, err)
	v, err := s.Version(ctx)
	require.NoError(t, err)

	require.NoError(t, kv.Delete(ctx, StateKey))

	l, err := s.Claim(ctx, "TKT-9", alice, leasetest.Hash("h9"), t0, t0.Add(time.Hour), "")
	require.NoError(t, err)
	require.Greater(t, l.Token, last["TKT-2"].Token)
	v2, err := s.Version(ctx)
	require.NoError(t, err)
	require.Greater(t, v2, v, "the version must not restart with the document")
}

// A damaged document is reset and written back by the first load that finds
// it, under the lock: one warning, which names the key but never the
// contents. Not parallel: it swaps slog's process-wide default logger.
func TestCorruptDocument_ResetOnceWithoutLoggingContents(t *testing.T) {
	ctx := context.Background()
	var buf bytes.Buffer
	logtest.SetDefault(t, slog.NewTextHandler(&buf, nil))

	kv := newMemKV(t)
	secret := `{"leases": SECRET-CONTENT`
	require.NoError(t, kv.Put(ctx, StateKey, []byte(secret)))
	s, err := New(kv, filelock.ProcessPrivate{})
	require.NoError(t, err)

	got, err := s.Get(ctx, []string{"TKT-1"}, t0)
	require.NoError(t, err, "a damaged document reads as empty")
	require.Empty(t, got)
	data, err := kv.Get(ctx, StateKey)
	require.NoError(t, err)
	require.True(t, json.Valid(data), "the first load must write the reset document back")

	_, err = s.Get(ctx, []string{"TKT-1"}, t0)
	require.NoError(t, err)
	_, err = s.Claim(ctx, "TKT-1", alice, leasetest.Hash("h1"), t0, t0.Add(time.Hour), "")
	require.NoError(t, err)

	require.Equal(t, 1, strings.Count(buf.String(), "damaged"), "one warning, not one per load")
	require.Contains(t, buf.String(), StateKey)
	require.NotContains(t, buf.String(), "SECRET-CONTENT")
}

func TestStoreCap_CountsLiveLeasesOnly(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	kv := newMemKV(t)
	d := document{Token: 1, Leases: map[string]storedLease{}}
	for i := range lease.MaxPerStore {
		expires := t0.Add(time.Hour)
		if i == 0 {
			expires = t0.Add(time.Minute)
		}
		d.Leases[fmt.Sprintf("TKT-%d", i)] = storedLease{
			Identity: fmt.Sprintf("u-%d", i), LeaseHash: fmt.Sprintf("h-%d", i), Token: 1, Since: t0, Expires: expires,
		}
	}
	data, err := json.Marshal(d)
	require.NoError(t, err)
	require.NoError(t, kv.Put(ctx, StateKey, data))
	s, err := New(kv, filelock.ProcessPrivate{})
	require.NoError(t, err)

	_, err = s.Claim(ctx, "TKT-X", alice, leasetest.Hash("hx"), t0, t0.Add(time.Hour), "")
	require.ErrorIs(t, err, lease.ErrFull)
	require.NotContains(t, err.Error(), "alice", "the full-store error names no holder")

	_, err = s.Claim(ctx, "TKT-X", alice, leasetest.Hash("hx"), t0.Add(time.Minute), t0.Add(time.Hour), "")
	require.NoError(t, err, "an expired lease does not count toward the cap")
}

// Not parallel: it swaps slog's process-wide default logger.
func TestVersion_ReadsOnlyTheVersionKey(t *testing.T) {
	ctx := context.Background()
	var buf bytes.Buffer
	logtest.SetDefault(t, slog.NewTextHandler(&buf, nil))

	kv := newMemKV(t)
	s, err := New(kv, filelock.ProcessPrivate{})
	require.NoError(t, err)

	v, err := s.Version(ctx)
	require.NoError(t, err)
	require.Zero(t, v, "no version key is version 0")

	require.NoError(t, kv.Put(ctx, StateKey, []byte("not json")))
	v, err = s.Version(ctx)
	require.NoError(t, err)
	require.Zero(t, v, "the version poll must not parse the document")

	require.NoError(t, kv.Put(ctx, VersionKey, []byte("41\n")))
	v, err = s.Version(ctx)
	require.NoError(t, err)
	require.Equal(t, uint64(41), v)

	require.NoError(t, kv.Put(ctx, VersionKey, []byte("x")))
	for range 3 {
		v, err = s.Version(ctx)
		require.NoError(t, err, "an unparsable version key counts as missing")
		require.Zero(t, v)
	}
	require.Equal(t, 1, strings.Count(buf.String(), "not a number"), "one warning per store")
}

// With the version key unparsable, the token rests on the document's counter
// and the microsecond floor, so it still increases at one instant.
func TestUnparsableVersionKey_TokenKeepsIncreasing(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	kv := newMemKV(t)
	s, err := New(kv, filelock.ProcessPrivate{})
	require.NoError(t, err)
	var last uint64
	for i := range 3 {
		hash := leasetest.Hash(fmt.Sprintf("h%d", i))
		l, claimErr := s.Claim(ctx, fmt.Sprintf("TKT-%d", i), alice, hash, t0, t0.Add(time.Hour), "")
		require.NoError(t, claimErr)
		last = l.Token
	}
	require.NoError(t, kv.Put(ctx, VersionKey, []byte("garbage")))

	l, err := s.Claim(ctx, "TKT-9", alice, leasetest.Hash("h9"), t0, t0.Add(time.Hour), "")
	require.NoError(t, err)
	require.Greater(t, l.Token, last)
	v, err := s.Version(ctx)
	require.NoError(t, err)
	require.Greater(t, v, l.Token, "the rewritten version key is above every token again")
}

// On windows the version is read under the lock; setting the field runs that
// path on every OS.
func TestVersion_UnderTheLock(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	s := newDiskStore(t, t.TempDir())
	s.lockVersion = true
	_, err := s.Claim(ctx, "TKT-1", alice, leasetest.Hash("h1"), t0, t0.Add(time.Hour), "")
	require.NoError(t, err)
	v, err := s.Version(ctx)
	require.NoError(t, err)
	require.NotZero(t, v)

	failing, err := New(newMemKV(t), failingLocker{})
	require.NoError(t, err)
	failing.lockVersion = true
	_, err = failing.Version(ctx)
	require.Error(t, err)
}

// A caller waiting for the store's semaphore stops when its context ends.
func TestLock_WaitEndsWithContext(t *testing.T) {
	t.Parallel()
	s, err := New(newMemKV(t), filelock.ProcessPrivate{})
	require.NoError(t, err)
	unlock, err := s.lock(context.Background())
	require.NoError(t, err)
	defer unlock()

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	_, err = s.Get(ctx, []string{"TKT-1"}, t0)
	require.ErrorIs(t, err, context.DeadlineExceeded)
}

// failingLocker refuses every lock.
type failingLocker struct{}

func (failingLocker) Lock(context.Context) (func(), error) { return nil, errors.New("no lock") }

func TestLockFailure_IsReturned(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	s, err := New(newMemKV(t), failingLocker{})
	require.NoError(t, err)
	_, err = s.Claim(ctx, "TKT-1", alice, leasetest.Hash("h1"), t0, t0.Add(time.Hour), "")
	require.Error(t, err)
	_, err = s.Get(ctx, []string{"TKT-1"}, t0)
	require.Error(t, err)
}
