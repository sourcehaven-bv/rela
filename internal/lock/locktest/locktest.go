// Package locktest is the conformance harness for [lock.Locker]
// implementations. Any new backend must pass [RunAll].
//
// It exists because the Locker contract is carried in prose on the interface,
// and the clauses that matter most are the ones a backend author would not
// think to test:
//
//   - Distinct keys must NOT contend. A backend that ignored the key and
//     serialized everything would satisfy "mutual exclusion" while destroying
//     the only property the seam exists to provide. This is the first test for
//     that reason.
//   - Acquire must be interruptible by ctx. A backend that blocks forever turns
//     a slow holder into a hung request path, which is exactly the failure the
//     keyed lock was introduced to avoid.
//   - Release must be idempotent, because callers defer it and may also call it
//     early on an error path.
package locktest

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/lock"
)

// Factory returns a fresh Locker for one subtest. Implementations that need
// cleanup should register it on tb.
//
// Backends whose locks are visible across processes must return a locker scoped
// so that concurrently-running subtests cannot collide — a per-call schema or
// key namespace.
type Factory func(tb testing.TB) lock.Locker

// RunAll runs every conformance test against the Locker produced by newLocker.
func RunAll(t *testing.T, newLocker Factory) {
	t.Helper()
	for name, fn := range map[string]func(*testing.T, Factory){
		"DistinctKeysDoNotContend":  testDistinctKeysDoNotContend,
		"SameKeyExcludes":           testSameKeyExcludes,
		"ReleaseIsIdempotent":       testReleaseIsIdempotent,
		"ContextCancelWhileWaiting": testContextCancelWhileWaiting,
		"ExpiredContextFails":       testExpiredContextFails,
		"ReleaseAfterPanic":         testReleaseAfterPanic,
		"SerializesConcurrent":      testSerializesConcurrent,
		"RejectsInvalidKeys":        testRejectsInvalidKeys,
	} {
		t.Run(name, func(t *testing.T) { fn(t, newLocker) })
	}
}

// testDistinctKeysDoNotContend is THE test for this seam. If it fails, the
// backend has degraded to a global mutex and the burst behaviour the design
// depends on is gone — while every other test here still passes.
func testDistinctKeysDoNotContend(t *testing.T, newLocker Factory) {
	l := newLocker(t)
	ctx := context.Background()

	relA, err := l.Acquire(ctx, "alpha")
	if err != nil {
		t.Fatalf("acquire alpha: %v", err)
	}
	defer relA()

	// Must not block: a different key is a different lock.
	done := make(chan error, 1)
	go func() {
		rel, err := l.Acquire(ctx, "beta")
		if err == nil {
			rel()
		}
		done <- err
	}()

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("acquire beta while alpha held: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("acquiring a DIFFERENT key blocked while alpha was held: " +
			"the backend is ignoring the key and serializing everything")
	}
}

func testSameKeyExcludes(t *testing.T, newLocker Factory) {
	l := newLocker(t)
	ctx := context.Background()

	rel, err := l.Acquire(ctx, "same")
	if err != nil {
		t.Fatalf("first acquire: %v", err)
	}

	blocked := make(chan struct{})
	go func() {
		defer close(blocked)
		rel2, err := l.Acquire(ctx, "same")
		if err == nil {
			rel2()
		}
	}()

	select {
	case <-blocked:
		t.Fatal("second acquire of a held key returned before release")
	case <-time.After(100 * time.Millisecond):
		// Still blocked, as required.
	}

	rel()

	select {
	case <-blocked:
	case <-time.After(5 * time.Second):
		t.Fatal("second acquire did not proceed after release")
	}
}

func testReleaseIsIdempotent(t *testing.T, newLocker Factory) {
	l := newLocker(t)
	ctx := context.Background()

	rel, err := l.Acquire(ctx, "idem")
	if err != nil {
		t.Fatalf("acquire: %v", err)
	}
	rel()
	rel() // must not panic, must not release a later holder's lock

	// The key must still be acquirable, and the extra release must not have
	// corrupted the backend's bookkeeping.
	rel2, err := l.Acquire(ctx, "idem")
	if err != nil {
		t.Fatalf("re-acquire after double release: %v", err)
	}
	rel2()
}

func testContextCancelWhileWaiting(t *testing.T, newLocker Factory) {
	l := newLocker(t)

	rel, err := l.Acquire(context.Background(), "busy")
	if err != nil {
		t.Fatalf("acquire: %v", err)
	}
	defer rel()

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	start := time.Now()
	rel2, err := l.Acquire(ctx, "busy")
	if err == nil {
		rel2()
		t.Fatal("acquire succeeded while the key was held by another caller")
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("want an error satisfying context.DeadlineExceeded, got %v", err)
	}
	if elapsed := time.Since(start); elapsed > 5*time.Second {
		t.Fatalf("acquire ignored the deadline: waited %v", elapsed)
	}
}

func testExpiredContextFails(t *testing.T, newLocker Factory) {
	l := newLocker(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	// Free key, but the caller is already done — it must fail rather than
	// succeed by luck of the lock being uncontended.
	rel, err := l.Acquire(ctx, "expired")
	if err == nil {
		rel()
		t.Fatal("acquire with an already-cancelled ctx succeeded")
	}
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("want an error satisfying context.Canceled, got %v", err)
	}
}

// testReleaseAfterPanic pins that a deferred release still runs when the
// critical section panics — the caller's `defer rel()` must not leave the key
// wedged for the process lifetime.
func testReleaseAfterPanic(t *testing.T, newLocker Factory) {
	l := newLocker(t)
	ctx := context.Background()

	func() {
		defer func() { _ = recover() }()
		rel, err := l.Acquire(ctx, "panicky")
		if err != nil {
			t.Errorf("acquire: %v", err)
			return
		}
		defer rel()
		panic("boom")
	}()

	acquired := make(chan struct{})
	go func() {
		rel, err := l.Acquire(ctx, "panicky")
		if err == nil {
			rel()
		}
		close(acquired)
	}()

	select {
	case <-acquired:
	case <-time.After(5 * time.Second):
		t.Fatal("key still held after a panicking critical section released it")
	}
}

// testSerializesConcurrent is the property callers actually depend on: a
// read-modify-write under one key never interleaves. A backend that hands the
// lock to two callers at once fails here even if testSameKeyExcludes passes,
// since that test only exercises one contending pair.
func testSerializesConcurrent(t *testing.T, newLocker Factory) {
	l := newLocker(t)
	ctx := context.Background()

	const goroutines = 8
	const increments = 25

	var (
		mu      sync.Mutex // guards counter, so the race detector reports OUR bug not the test's
		counter int
		inside  atomic.Int32
		overlap atomic.Bool
		wg      sync.WaitGroup
	)

	for range goroutines {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for range increments {
				rel, err := l.Acquire(ctx, "counter")
				if err != nil {
					t.Errorf("acquire: %v", err)
					return
				}
				if inside.Add(1) != 1 {
					overlap.Store(true)
				}
				mu.Lock()
				counter++
				mu.Unlock()
				inside.Add(-1)
				rel()
			}
		}()
	}
	wg.Wait()

	if overlap.Load() {
		t.Error("two callers were inside the critical section for one key simultaneously")
	}
	if want := goroutines * increments; counter != want {
		t.Errorf("counter = %d, want %d (lost updates)", counter, want)
	}
}

func testRejectsInvalidKeys(t *testing.T, newLocker Factory) {
	l := newLocker(t)
	ctx := context.Background()

	// Held to the same table as state.ValidateKey so a caller deriving both a
	// lock key and a state key from one source meets one contract, not two.
	for _, key := range []string{
		"",
		"..",
		"a/../b",
		"a//b",
		"back\\slash",
		"nul\x00byte",
		"ctrl\x01char",
		fmt.Sprintf("%0*d", lock.MaxKeyLen+1, 0),
	} {
		t.Run(fmt.Sprintf("%q", key), func(t *testing.T) {
			rel, err := l.Acquire(ctx, key)
			if err == nil {
				rel()
				t.Errorf("Acquire(%q) succeeded, want a validation error", key)
			}
		})
	}
}
