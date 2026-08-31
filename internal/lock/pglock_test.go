package lock_test

import (
	"context"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/lock"
	"github.com/Sourcehaven-BV/rela/internal/lock/locktest"
)

// fakeBackend stands in for a store-backed keyed lock. It delegates to the
// memory locker, so this exercises the ADAPTER's contract (validation,
// ctx handling, release plumbing) without a database.
//
// The real cross-process behaviour is pgstore's and is covered by the
// DB-gated suite there; what can go wrong here is the adapter dropping a
// validation check or mishandling a cancelled ctx, which this catches.
type fakeBackend struct{ inner lock.Locker }

func (f fakeBackend) AcquireKeyedLock(ctx context.Context, key string) (func(), error) {
	return f.inner.Acquire(ctx, key)
}

func TestBackendLocker_Conformance(t *testing.T) {
	locktest.RunAll(t, func(tb testing.TB) lock.Locker {
		l, err := lock.NewBackendLocker(fakeBackend{inner: lock.NewMemoryLocker()})
		if err != nil {
			tb.Fatalf("NewBackendLocker: %v", err)
		}
		return l
	})
}

// TestNewBackendLocker_RejectsNil pins the constructor contract: a nil backend
// must be refused rather than yielding a Locker that serializes nothing.
func TestNewBackendLocker_RejectsNil(t *testing.T) {
	if _, err := lock.NewBackendLocker(nil); err == nil {
		t.Fatal("NewBackendLocker(nil) succeeded, want an error")
	}
}
