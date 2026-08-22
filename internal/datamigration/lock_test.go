package datamigration

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/audit"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/store/memstore"
)

func TestProcessLock_Exclusive(t *testing.T) {
	l := NewProcessLock()
	release, err := l.TryAcquire(t.Context())
	if err != nil {
		t.Fatalf("first acquire: %v", err)
	}
	if _, err := l.TryAcquire(t.Context()); !errors.Is(err, ErrLockHeld) {
		t.Fatalf("second acquire err = %v, want ErrLockHeld", err)
	}
	release()
	release() // double release must be safe
	if release2, err := l.TryAcquire(t.Context()); err != nil {
		t.Fatalf("acquire after release: %v", err)
	} else {
		release2()
	}
}

func TestFSLock_ExclusiveAndReleases(t *testing.T) {
	dir := t.TempDir()
	a, b := newFSLock(dir), newFSLock(dir)

	release, err := a.TryAcquire(t.Context())
	if err != nil {
		t.Fatalf("first acquire: %v", err)
	}
	if _, bErr := b.TryAcquire(t.Context()); !errors.Is(bErr, ErrLockHeld) {
		t.Fatalf("second lock on same dir err = %v, want ErrLockHeld", bErr)
	}
	// Same process, same fsLock value: the embedded process mutex must
	// exclude too (the pid in the file is OUR live pid — without the mutex
	// the staleness check could never say held-by-someone-else).
	if _, aErr := a.TryAcquire(t.Context()); !errors.Is(aErr, ErrLockHeld) {
		t.Fatalf("re-acquire on same lock err = %v, want ErrLockHeld", aErr)
	}
	release()
	if _, statErr := os.Stat(filepath.Join(dir, lockFileName)); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("lock file survives release: %v", statErr)
	}
	release2, err := b.TryAcquire(t.Context())
	if err != nil {
		t.Fatalf("acquire after release: %v", err)
	}
	release2()
}

func TestFSLock_BreaksStaleAndUnparseable(t *testing.T) {
	tests := []struct {
		name    string
		payload []byte
	}{
		{
			name: "dead pid",
			payload: func() []byte {
				// A just-exited child's pid is provably dead on this host.
				data, _ := json.Marshal(lockFilePayload{PID: deadPID(t), AcquiredAt: time.Now().UTC()})
				return data
			}(),
		},
		{name: "unparseable", payload: []byte("not json{")},
		{name: "zero pid", payload: []byte(`{"pid":0}`)},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, lockFileName)
			if err := os.WriteFile(path, tc.payload, 0o644); err != nil {
				t.Fatal(err)
			}
			l := newFSLock(dir)
			release, err := l.TryAcquire(t.Context())
			if err != nil {
				t.Fatalf("stale lock not broken: %v", err)
			}
			release()
		})
	}
}

func TestFSLock_HonorsLivePid(t *testing.T) {
	dir := t.TempDir()
	// A foreign lock file naming a pid that IS alive (ours, but written by
	// "another" lock value so the in-process mutex is not involved) must be
	// honored, never broken.
	data, _ := json.Marshal(lockFilePayload{PID: os.Getpid(), AcquiredAt: time.Now().UTC()})
	if err := os.WriteFile(filepath.Join(dir, lockFileName), data, 0o644); err != nil {
		t.Fatal(err)
	}
	l := newFSLock(dir)
	if _, err := l.TryAcquire(t.Context()); !errors.Is(err, ErrLockHeld) {
		t.Fatalf("live holder's lock was broken: err = %v", err)
	}
}

// deadPID returns a pid that existed and is now gone.
func deadPID(t *testing.T) int {
	t.Helper()
	proc, err := os.StartProcess("/usr/bin/true", []string{"true"}, &os.ProcAttr{})
	if err != nil {
		// Portable fallback: /bin/true on some systems.
		proc, err = os.StartProcess("/bin/true", []string{"true"}, &os.ProcAttr{})
		if err != nil {
			t.Skipf("cannot start helper process: %v", err)
		}
	}
	state, err := proc.Wait()
	if err != nil {
		t.Fatalf("wait: %v", err)
	}
	return state.Pid()
}

// fakeStoreLocker is a store.Store that also offers the migration-lock
// capability, for LockFor selection tests.
type fakeStoreLocker struct {
	store.Store
	held bool
}

func (f *fakeStoreLocker) TryMigrationLock(context.Context) (release func(), ok bool, err error) {
	if f.held {
		return nil, false, nil
	}
	return func() {}, true, nil
}

func TestLockFor_Selection(t *testing.T) {
	t.Run("store capability wins", func(t *testing.T) {
		fake := &fakeStoreLocker{Store: memstore.New(), held: true}
		l := LockFor(fake, t.TempDir())
		if _, err := l.TryAcquire(t.Context()); !errors.Is(err, ErrLockHeld) {
			t.Fatalf("store-held lock not surfaced as ErrLockHeld: %v", err)
		}
		fake.held = false
		release, err := l.TryAcquire(t.Context())
		if err != nil {
			t.Fatalf("store lock acquire: %v", err)
		}
		release()
	})
	t.Run("cache dir falls back to fs lock", func(t *testing.T) {
		dir := t.TempDir()
		l := LockFor(memstore.New(), dir)
		release, err := l.TryAcquire(t.Context())
		if err != nil {
			t.Fatalf("fs lock acquire: %v", err)
		}
		defer release()
		if _, err := os.Stat(filepath.Join(dir, lockFileName)); err != nil {
			t.Fatalf("fs lock file not created — wrong implementation selected: %v", err)
		}
	})
	t.Run("no cache dir falls back to process lock", func(t *testing.T) {
		l := LockFor(memstore.New(), "")
		if _, ok := l.(*ProcessLock); !ok {
			t.Fatalf("selected %T, want *ProcessLock", l)
		}
	})
}

// spyLock records TryAcquire calls; heldErr non-nil makes it contended.
type spyLock struct {
	calls   int
	heldErr error
}

func (s *spyLock) TryAcquire(context.Context) (func(), error) {
	s.calls++
	if s.heldErr != nil {
		return nil, s.heldErr
	}
	return func() {}, nil
}

func TestRunner_DryRunNeverTouchesLock(t *testing.T) {
	spy := &spyLock{}
	r := newTestRunner(t, Deps{Store: seedStore(t), Lock: spy})
	f := mustParse(t, "0001-test.yaml", mustFileYAML(t, metaV1(), metaV2(), v1ToV2Steps))
	if _, err := r.Run(t.Context(), []*File{f}, false); err != nil {
		t.Fatalf("dry-run: %v", err)
	}
	if spy.calls != 0 {
		t.Fatalf("dry-run touched the lock %d times", spy.calls)
	}
}

func TestRunner_ApplyFailsFastWhenLockHeld(t *testing.T) {
	st := seedStore(t)
	spy := &spyLock{heldErr: ErrLockHeld}
	kv := newFakeKV()
	r := newTestRunner(t, Deps{Store: st, Lock: spy, State: kv})
	f := mustParse(t, "0001-test.yaml", mustFileYAML(t, metaV1(), metaV2(), v1ToV2Steps))

	_, err := r.Run(t.Context(), []*File{f}, true)
	if !errors.Is(err, ErrLockHeld) {
		t.Fatalf("apply err = %v, want ErrLockHeld", err)
	}
	// Zero writes: entities untouched, marker untouched.
	if _, has := getEntity(t, st, "TSK-1").Properties["state"]; has {
		t.Fatalf("contended apply wrote to the store")
	}
	if marker, _ := LoadMarker(t.Context(), kv); marker != nil {
		t.Fatalf("contended apply wrote the marker")
	}
}

func TestGC_ApplySkipsWhenLockHeld(t *testing.T) {
	st, kv, gate, m2 := driftSetup(t)
	spy := &spyLock{heldErr: ErrLockHeld}
	g := newTestGC(t, GCDeps{
		Store: st, State: kv, Meta: func() *metamodel.Metamodel { return m2 },
		Verdicts: gate, Lock: spy,
	})
	g.now = func() time.Time { return time.Now().Add(DefaultGrace + time.Hour) }
	res, err := g.Tick(t.Context(), true)
	if err != nil {
		t.Fatalf("Tick: %v", err)
	}
	if res.Skipped == "" || len(res.Deleted) != 0 {
		t.Fatalf("contended apply tick did not skip: %+v", res)
	}
	if _, has := getEntity(t, st, "TSK-1").Properties["tags"]; !has {
		t.Fatalf("contended tick deleted data")
	}
	// Dry-run ticks stay lock-free.
	spy.calls = 0
	if _, err := g.Tick(t.Context(), false); err != nil {
		t.Fatalf("dry-run tick: %v", err)
	}
	if spy.calls != 0 {
		t.Fatalf("dry-run tick touched the lock")
	}
}

func TestGC_ScanFailsWhenLockHeld(t *testing.T) {
	st, kv, gate, m2 := driftSetup(t)
	g := newTestGC(t, GCDeps{
		Store: st, State: kv, Meta: func() *metamodel.Metamodel { return m2 },
		Verdicts: gate, Lock: &spyLock{heldErr: ErrLockHeld},
	})
	if _, err := g.Scan(t.Context()); !errors.Is(err, ErrLockHeld) {
		t.Fatalf("Scan err = %v, want ErrLockHeld", err)
	}
}

func TestGate_AdoptionSkipsOnContention(t *testing.T) {
	kv := newFakeKV()
	lock := NewProcessLock()
	g, err := NewGate(kv, lock)
	if err != nil {
		t.Fatal(err)
	}
	// Hold the lock as "the runner".
	release, err := lock.TryAcquire(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	defer release()

	v, err := g.Evaluate(t.Context(), metaV1())
	if err != nil {
		t.Fatalf("Evaluate under contention must not fail: %v", err)
	}
	if v == nil || g.Verdict() != v {
		t.Fatalf("verdict not published under contention")
	}
	if marker, _ := LoadMarker(t.Context(), kv); marker != nil {
		t.Fatalf("contended adoption wrote the marker")
	}

	// Lock released: the next evaluation persists the baseline.
	release()
	if _, err := g.Evaluate(t.Context(), metaV1()); err != nil {
		t.Fatal(err)
	}
	if marker, _ := LoadMarker(t.Context(), kv); marker == nil {
		t.Fatalf("marker not written after lock release")
	}
}

func TestConstructors_RejectNilLock(t *testing.T) {
	if _, err := NewRunner(Deps{
		Store: memstore.New(), Meta: metaV1(), State: newFakeKV(),
		Audit: audit.NewMemory(), ScriptFS: emptyFS,
	}); err == nil {
		t.Fatalf("NewRunner accepted nil Lock")
	}
	if _, err := NewGC(GCDeps{
		Store: memstore.New(), Meta: metaV1, State: newFakeKV(),
		Audit: audit.NewMemory(), Verdicts: fixedVerdict{},
	}); err == nil {
		t.Fatalf("NewGC accepted nil Lock")
	}
}
