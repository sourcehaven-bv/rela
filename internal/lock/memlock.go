package lock

import (
	"context"
	"fmt"
	"sync"
)

// MemoryLocker is the in-process [Locker] used by the fs/desktop tier, where
// rela is single-process by nature so an in-process mutex IS the whole
// serialization boundary.
//
// It is not a degraded postgres backend. On a single-user local app there is no
// second process to exclude, so a cross-process lock would buy nothing and cost
// a database.
type MemoryLocker struct {
	mu   sync.Mutex
	held map[string]*memEntry
}

// memEntry refcounts waiters for one key so the map entry can be dropped when
// the last one goes.
//
// Without the refcount, `held` would grow one entry per DISTINCT key ever
// locked and never shrink — a slow leak that is invisible in tests (which use a
// handful of keys) and unbounded in production, where keys derive from request
// data. Tracking waiters rather than deleting on release is what makes the
// cleanup safe: a waiter blocked on the mutex still needs the entry to exist.
type memEntry struct {
	mu      sync.Mutex
	waiters int
}

// NewMemoryLocker returns an in-process Locker.
func NewMemoryLocker() *MemoryLocker {
	return &MemoryLocker{held: make(map[string]*memEntry)}
}

// Acquire implements [Locker].
func (l *MemoryLocker) Acquire(ctx context.Context, key string) (func(), error) {
	if err := ValidateKey(key); err != nil {
		return nil, err
	}
	// Fail before waiting if the caller's deadline has already passed, so an
	// expired ctx behaves the same whether or not the lock happens to be free.
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("lock: acquire %q: %w", key, err)
	}

	e := l.entry(key)

	// sync.Mutex has no ctx-aware Lock, so block in a goroutine and let the
	// caller's ctx win the race. The goroutine cannot leak: it is waiting on a
	// mutex that some holder will release, and on the abandoned path below it
	// releases immediately and drops the refcount.
	acquired := make(chan struct{})
	go func() {
		e.mu.Lock()
		close(acquired)
	}()

	select {
	case <-acquired:
		return l.releaser(key, e), nil
	case <-ctx.Done():
		go func() {
			// Wait for the acquisition we abandoned, then undo it. Without
			// this the mutex would be locked with no owner and the key would
			// wedge for the process lifetime.
			<-acquired
			e.mu.Unlock()
			l.drop(key, e)
		}()
		return nil, fmt.Errorf("lock: acquire %q: %w", key, ctx.Err())
	}
}

// entry returns the refcounted entry for key, registering this caller as a
// waiter.
func (l *MemoryLocker) entry(key string) *memEntry {
	l.mu.Lock()
	defer l.mu.Unlock()
	e, ok := l.held[key]
	if !ok {
		e = &memEntry{}
		l.held[key] = e
	}
	e.waiters++
	return e
}

// drop deregisters a waiter and removes the entry once none remain.
func (l *MemoryLocker) drop(key string, e *memEntry) {
	l.mu.Lock()
	defer l.mu.Unlock()
	e.waiters--
	if e.waiters == 0 {
		// Only delete the entry still registered under this key. A racing
		// Acquire that already replaced it must not have its entry evicted.
		if cur, ok := l.held[key]; ok && cur == e {
			delete(l.held, key)
		}
	}
}

// releaser returns an idempotent release function for a held entry.
func (l *MemoryLocker) releaser(key string, e *memEntry) func() {
	var once sync.Once
	return func() {
		once.Do(func() {
			e.mu.Unlock()
			l.drop(key, e)
		})
	}
}
