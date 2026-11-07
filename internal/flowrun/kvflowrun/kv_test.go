package kvflowrun

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/filelock"
	"github.com/Sourcehaven-BV/rela/internal/flow"
	"github.com/Sourcehaven-BV/rela/internal/flowrun"
	"github.com/Sourcehaven-BV/rela/internal/flowrun/flowruntest"
	"github.com/Sourcehaven-BV/rela/internal/state"
	"github.com/Sourcehaven-BV/rela/internal/storage"
)

var t0 = time.Date(2026, 11, 7, 9, 0, 0, 0, time.UTC)

func newMemKV(t *testing.T) state.KV {
	t.Helper()
	mem := storage.NewMemFS()
	require.NoError(t, mem.MkdirAll("/root", 0o755))
	rfs, err := storage.NewRootedFS(mem, "/root")
	require.NoError(t, err)
	return state.NewFSKV(rfs)
}

// newDiskStore opens a Store over the on-disk KV in dir with its own file
// lock there, as one process would.
func newDiskStore(t *testing.T, dir string) *Store {
	t.Helper()
	rfs, err := storage.NewRootedFS(storage.NewSafeFS(storage.NewOsFS()), dir)
	require.NoError(t, err)
	locker, err := filelock.New(filepath.Join(dir, "flow-runs.lock"))
	require.NoError(t, err)
	s, err := New(state.NewFSKV(rfs), locker)
	require.NoError(t, err)
	return s
}

func TestConformance_MemFS(t *testing.T) {
	t.Parallel()
	flowruntest.RunAll(t, func(t *testing.T) flowruntest.Harness {
		t.Helper()
		s, err := New(newMemKV(t), filelock.ProcessPrivate{})
		require.NoError(t, err)
		return flowruntest.Harness{Store: s}
	})
}

// TestConformance_Disk runs the contract over an on-disk KV with a real
// file lock; Second opens another Store over the same directory, as a
// second process does.
func TestConformance_Disk(t *testing.T) {
	t.Parallel()
	flowruntest.RunAll(t, func(t *testing.T) flowruntest.Harness {
		t.Helper()
		dir := t.TempDir()
		return flowruntest.Harness{
			Store:  newDiskStore(t, dir),
			Second: func(t *testing.T) flowrun.Store { t.Helper(); return newDiskStore(t, dir) },
		}
	})
}

func TestNewRejectsNil(t *testing.T) {
	_, err := New(nil, filelock.ProcessPrivate{})
	require.Error(t, err)
	_, err = New(newMemKV(t), nil)
	require.Error(t, err)
}

func newRun(id string) flowrun.NewRun {
	return flowrun.NewRun{
		ID: flowrun.RunID(id), Flow: "onboard", Version: "0123456789abcdef", Definition: []byte("d"),
		Subject: entity.Ref{ID: "EMP-" + id}, At: t0,
		Instance: flow.Instance{State: "open", Generation: 1, EnteredAt: t0, NextDueAt: t0.Add(time.Hour)},
	}
}

// TestReopen: a run's state survives a new Store over the same directory.
func TestReopen(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	created, err := newDiskStore(t, dir).Create(ctx, newRun("FR-1"))
	require.NoError(t, err)

	got, err := newDiskStore(t, dir).Get(ctx, "FR-1")
	require.NoError(t, err)
	require.Equal(t, created, got)
}

// TestDamagedDocument: a runs or definitions document that does not decode
// is an error naming the key, and nothing resets it.
func TestDamagedDocument(t *testing.T) {
	ctx := context.Background()
	for _, key := range []string{RunsKey, DefinitionsKey} {
		t.Run(key, func(t *testing.T) {
			kv := newMemKV(t)
			require.NoError(t, kv.Put(ctx, key, []byte("{not json")))
			s, err := New(kv, filelock.ProcessPrivate{})
			require.NoError(t, err)

			_, err = s.Create(ctx, newRun("FR-1"))
			require.ErrorContains(t, err, key)
			require.ErrorContains(t, err, "damaged")
			raw, err := kv.Get(ctx, key)
			require.NoError(t, err)
			require.Equal(t, "{not json", string(raw), "the damaged document is kept")
		})
	}
}

// TestDamagedState: a run whose stored state does not decode fails the
// reads that touch it.
func TestDamagedState(t *testing.T) {
	ctx := context.Background()
	kv := newMemKV(t)
	doc := `{"runs":{"FR-1":{"flow":"onboard","version":"0123456789abcdef","subject_id":"E-1",` +
		`"created_at":"2026-11-07T09:00:00Z","updated_at":"2026-11-07T09:00:00Z","rev":1,"state":"oops"}}}`
	require.NoError(t, kv.Put(ctx, RunsKey, []byte(doc)))
	s, err := New(kv, filelock.ProcessPrivate{})
	require.NoError(t, err)

	_, err = s.Get(ctx, "FR-1")
	require.ErrorContains(t, err, "FR-1")
	_, err = s.List(ctx, flowrun.Query{Limit: 10})
	require.Error(t, err)
}

// TestFull: the store refuses a run beyond MaxRuns.
func TestFull(t *testing.T) {
	ctx := context.Background()
	kv := newMemKV(t)
	d := runsDoc{Runs: map[flowrun.RunID]storedRun{}}
	stateDoc, err := flowrun.EncodeState(flow.Instance{})
	require.NoError(t, err)
	for i := range MaxRuns {
		d.Runs[flowrun.RunID(fmt.Sprintf("FR-%d", i))] = storedRun{
			Flow: "onboard", Version: "0123456789abcdef", SubjectID: fmt.Sprintf("E-%d", i),
			CreatedAt: t0, UpdatedAt: t0, Rev: 1, State: stateDoc,
		}
	}
	s, err := New(kv, filelock.ProcessPrivate{})
	require.NoError(t, err)
	require.NoError(t, s.put(ctx, RunsKey, d))

	_, err = s.Create(ctx, newRun("FR-X"))
	require.ErrorIs(t, err, flowrun.ErrFull)
}

type failingKV struct {
	state.KV
	failPut string
}

func (f failingKV) Put(ctx context.Context, key string, data []byte) error {
	if key == f.failPut {
		return errors.New("disk full")
	}
	return f.KV.Put(ctx, key, data)
}

// TestCreate_RunsWriteFails: a create whose runs write fails leaves only
// an unused definition, which a prune drops.
func TestCreate_RunsWriteFails(t *testing.T) {
	ctx := context.Background()
	kv := newMemKV(t)
	s, err := New(failingKV{KV: kv, failPut: RunsKey}, filelock.ProcessPrivate{})
	require.NoError(t, err)
	_, err = s.Create(ctx, newRun("FR-1"))
	require.ErrorContains(t, err, "disk full")

	healthy, err := New(kv, filelock.ProcessPrivate{})
	require.NoError(t, err)
	_, err = healthy.Get(ctx, "FR-1")
	require.ErrorIs(t, err, flowrun.ErrNotFound)
	n, err := healthy.PruneDefinitions(ctx, t0.Add(time.Minute))
	require.NoError(t, err)
	require.Equal(t, 1, n)
}

type cancelledLocker struct{}

func (cancelledLocker) Lock(context.Context) (func(), error) { return nil, context.Canceled }

func TestLockErrors(t *testing.T) {
	s, err := New(newMemKV(t), cancelledLocker{})
	require.NoError(t, err)
	_, err = s.Get(context.Background(), "FR-1")
	require.ErrorIs(t, err, context.Canceled)

	held, err := New(newMemKV(t), filelock.ProcessPrivate{})
	require.NoError(t, err)
	held.sem <- struct{}{}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err = held.Get(ctx, "FR-1")
	require.ErrorIs(t, err, context.Canceled)
}
