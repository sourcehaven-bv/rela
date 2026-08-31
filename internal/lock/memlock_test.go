package lock_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/lock"
	"github.com/Sourcehaven-BV/rela/internal/lock/locktest"
)

func TestMemoryLocker_Conformance(t *testing.T) {
	locktest.RunAll(t, func(testing.TB) lock.Locker {
		return lock.NewMemoryLocker()
	})
}

// TestMemoryLocker_NoEntryLeak pins the refcount bookkeeping: locking many
// distinct keys must not grow the internal map without bound. Keys derive from
// request data in the intended callers, so a leak here is unbounded in
// production while invisible in a test that uses three keys.
func TestMemoryLocker_NoEntryLeak(t *testing.T) {
	l := lock.NewMemoryLocker()
	ctx := context.Background()

	for i := range 1000 {
		rel, err := l.Acquire(ctx, key(i))
		if err != nil {
			t.Fatalf("acquire: %v", err)
		}
		rel()
	}

	if n := lock.MemoryLockerEntries(l); n != 0 {
		t.Errorf("after releasing every lock, %d map entries remain (want 0)", n)
	}
}

// TestMemoryLocker_AbandonedAcquireReleases pins the ctx-cancellation path: a
// caller that gives up waiting must not leave the mutex locked with no owner
// once the original holder releases. Getting this wrong wedges the key for the
// process lifetime, and only shows up under contention.
func TestMemoryLocker_AbandonedAcquireReleases(t *testing.T) {
	l := lock.NewMemoryLocker()

	rel, err := l.Acquire(context.Background(), "contended")
	if err != nil {
		t.Fatalf("acquire: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	if _, err := l.Acquire(ctx, "contended"); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("want DeadlineExceeded, got %v", err)
	}

	// The abandoned waiter is still blocked on the mutex. Releasing hands it
	// the lock, and its cleanup goroutine must immediately give it back.
	rel()

	acquired := make(chan struct{})
	go func() {
		rel2, err := l.Acquire(context.Background(), "contended")
		if err == nil {
			rel2()
		}
		close(acquired)
	}()

	select {
	case <-acquired:
	case <-time.After(5 * time.Second):
		t.Fatal("key wedged: the abandoned waiter never released the lock it was handed")
	}

	if n := lock.MemoryLockerEntries(l); n != 0 {
		t.Errorf("%d map entries remain after an abandoned acquire (want 0)", n)
	}
}

// TestMemoryLocker_ConcurrentDistinctKeys exercises the map bookkeeping under
// real concurrency — entry creation and eviction race on the same map.
func TestMemoryLocker_ConcurrentDistinctKeys(t *testing.T) {
	l := lock.NewMemoryLocker()
	ctx := context.Background()

	var wg sync.WaitGroup
	for i := range 50 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for range 20 {
				rel, err := l.Acquire(ctx, key(i))
				if err != nil {
					t.Errorf("acquire: %v", err)
					return
				}
				rel()
			}
		}()
	}
	wg.Wait()

	if n := lock.MemoryLockerEntries(l); n != 0 {
		t.Errorf("%d map entries remain (want 0)", n)
	}
}

func key(i int) string {
	return "key-" + string(rune('a'+i%26)) + "-" + itoa(i)
}

func itoa(i int) string {
	if i == 0 {
		return "0"
	}
	var b []byte
	for i > 0 {
		b = append([]byte{byte('0' + i%10)}, b...)
		i /= 10
	}
	return string(b)
}
