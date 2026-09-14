package lock

import (
	"context"
	"errors"
	"fmt"
)

// KeyedLockBackend is the cross-process locking capability a store may offer,
// declared HERE at the call site so this package does not import a store
// implementation (the consumer-side-interface rule). pgstore.Store satisfies it
// via AcquireKeyedLock; the wiring site supplies it.
type KeyedLockBackend interface {
	AcquireKeyedLock(ctx context.Context, key string) (release func(), err error)
}

// BackendLocker adapts a [KeyedLockBackend] to [Locker].
//
// This is the tier that matters when several rela-server processes serve one
// database: an in-process mutex excludes nothing between them, whereas the
// backend's lock is visible to every process on that schema.
type BackendLocker struct {
	backend KeyedLockBackend
}

// NewBackendLocker returns a Locker backed by b.
//
// Nil: rejected — a nil backend would produce a Locker that silently serializes
// nothing, which is the one failure mode a lock must never have.
func NewBackendLocker(b KeyedLockBackend) (*BackendLocker, error) {
	if b == nil {
		return nil, errors.New("lock: NewBackendLocker: backend is required")
	}
	return &BackendLocker{backend: b}, nil
}

// Acquire implements [Locker].
func (l *BackendLocker) Acquire(ctx context.Context, key string) (func(), error) {
	if err := ValidateKey(key); err != nil {
		return nil, err
	}
	// Check before delegating so an expired caller fails identically on both
	// tiers, rather than depending on whether the backend happens to notice.
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("lock: acquire %q: %w", key, err)
	}
	return l.backend.AcquireKeyedLock(ctx, key)
}
