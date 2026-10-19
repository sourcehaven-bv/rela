package pgstore

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Sourcehaven-BV/rela/internal/store"
)

// keyedLockClassKey namespaces the caller-keyed advisory locks, distinct from
// the migrate / reconcile / migration / sweep keys because advisory locks are
// database-GLOBAL.
//
// Unlike those four, the second slot of the two-key form is NOT
// hashtext(current_schema()) but a hash of (schema, caller key): the caller's
// key is the whole point, and the schema is folded into that hash so tenants
// sharing a database still do not exclude each other (the BUG-CA3VY0 class).
const keyedLockClassKey int64 = 0x52_45_4c_4b // "RELK"

// AcquireKeyedLock takes the schema-scoped advisory lock named key, BLOCKING
// until it is held or ctx is done.
//
// It is the cross-process half of the internal/lock seam: two rela-server
// processes against one database contend here, where an in-process mutex gives
// nothing. Consumers take the lock.Locker interface; this is the postgres
// implementation behind it.
//
// Blocking is the difference from every other advisory lock in this package.
// TryMigrationLock and the sweep use pg_try_advisory_lock and skip when another
// holder is active — right when the other holder is doing the same work.
// A keyed lock guards a caller's own critical section, so skipping would drop
// that caller's work; it waits instead, bounded by ctx.
//
// The lock is session-scoped and pinned to ONE pool connection held until
// release — handing it back earlier would silently void the guarantee (the
// sweep-tick rule). A leaked release is therefore also a leaked connection,
// which is why callers must defer it and pass a ctx with a deadline.
//
// This is an optional store capability discovered by type-assert, like
// Formatter and the version capabilities — not part of store.Store.
func (s *Store) AcquireKeyedLock(ctx context.Context, key string) (release func(), err error) {
	pool, isPool := s.db.(*pgxpool.Pool)
	if !isPool {
		// A bare handle (unit-test store) has no pool to pin a session lock
		// to. Error rather than pretend: a caller that got this far chose the
		// store-backed lock and must not silently fall back to no exclusion.
		return nil, errors.New("pgstore: keyed lock requires a pgxpool-backed store")
	}
	if pool.Config().MaxConns < 2 {
		// The lock pins one connection for as long as it is held while the
		// caller's own writes acquire others from the same pool — at
		// pool_max_conns=1 that is a guaranteed self-deadlock, so refuse up
		// front with the remedy in the message.
		//
		// This guard covers ONE holder, not N. Because each held lock pins a
		// connection, C concurrent holders that also write need C+1 <= MaxConns
		// to make progress: at C == MaxConns every connection is a lock and the
		// writes wait for one that only frees on release. keyedSlots enforces
		// that by admitting at most half the pool as holders; further callers
		// wait for a slot, bounded by their ctx deadline. Prefer a conditional
		// write (unique: / If-Match) over a lock when the critical section is a
		// single write anyway.
		return nil, errors.New(
			"pgstore: keyed lock needs pool_max_conns >= 2 (the lock pins one connection while the caller's writes use others)")
	}

	freeSlot, err := takeKeyedSlot(ctx, s.keyedSlots)
	if err != nil {
		return nil, err
	}
	conn, err := pool.Acquire(ctx)
	if err != nil {
		freeSlot()
		return nil, fmt.Errorf("pgstore: keyed lock acquire connection: %w", err)
	}

	// pg_advisory_lock blocks server-side. pgx cancels the in-flight query when
	// ctx is done, which surfaces here as an error.
	//
	// A cancelled acquire is NOT reliably a lock that was never granted: the
	// server can grant it as the cancellation arrives, leaving the session
	// holding a lock this function is about to report as failed. Destroying the
	// connection rather than returning it to the pool is what makes that safe —
	// a session-scoped lock dies with its session, so the key is released
	// instead of being held by a pooled connection nobody knows is a holder.
	if _, err := conn.Exec(ctx,
		`SELECT pg_advisory_lock($1::int, hashtext(current_schema() || '/' || $2::text))`,
		keyedLockClassKey, key,
	); err != nil {
		conn.Hijack().Close(context.WithoutCancel(ctx))
		freeSlot()
		return nil, fmt.Errorf("pgstore: keyed lock %q: %w", key, err)
	}

	var releaseOnce sync.Once
	return func() {
		releaseOnce.Do(func() {
			defer freeSlot()
			// Detached ctx so the unlock still runs during shutdown; releasing
			// the connection would drop the session lock anyway, but an
			// explicit unlock keeps the pair symmetric and returns a clean
			// connection to the pool rather than one carrying a held lock.
			unlockCtx := context.WithoutCancel(ctx)
			if _, err := conn.Exec(unlockCtx,
				`SELECT pg_advisory_unlock($1::int, hashtext(current_schema() || '/' || $2::text))`,
				keyedLockClassKey, key,
			); err != nil {
				// A session lock dies with its session, so a failed unlock
				// cannot wedge the key — but the connection may still carry it,
				// so destroy rather than return it to the pool.
				slog.Warn("pgstore: keyed advisory unlock failed", "key", key, "error", err)
				conn.Hijack().Close(unlockCtx)
				return
			}
			conn.Release()
		})
	}, nil
}

// TryKeyedLock takes the keyed lock named key on the database st writes to,
// if no session holds it, and returns at once. It reports false, with a nil
// lock, when another holder has it.
//
// It is for work one process does on behalf of all, such as the realtime
// worker: the leader holds it for as long as it leads, and the others try
// it now and then to take over. Holding it pins one pool connection for
// that whole time, because the lock is session-scoped, so it needs
// pool_max_conns >= 3: one for the lock, and at least two left for the
// leader's writes and for requests.
//
// The database frees the lock when it drops the session (an idle timeout,
// a proxy cut, a failover), and the holder is not told. A holder calls
// [HeldLock.Check] now and then and stops acting as the holder once it
// fails.
//
// It shares the key space of [Store.AcquireKeyedLock], so the two exclude
// each other on one key. It takes no keyed slot: a held lock never makes
// another caller wait for one. A store that is not a pool-backed pgstore is
// an error.
func TryKeyedLock(ctx context.Context, st store.Store, key string) (lock *HeldLock, ok bool, err error) {
	pool, isPool := HandleFor(st).(*pgxpool.Pool)
	if !isPool {
		return nil, false, errors.New("pgstore: keyed lock requires a pgxpool-backed store")
	}
	if pool.Config().MaxConns < 3 {
		return nil, false, errors.New(
			"pgstore: a held keyed lock needs pool_max_conns >= 3 (the lock pins one connection for as long as " +
				"it is held, and writes and requests need the others)")
	}
	conn, err := pool.Acquire(ctx)
	if err != nil {
		return nil, false, fmt.Errorf("pgstore: keyed lock acquire connection: %w", err)
	}
	if err := conn.QueryRow(ctx,
		`SELECT pg_try_advisory_lock($1::int, hashtext(current_schema() || '/' || $2::text))`,
		keyedLockClassKey, key,
	).Scan(&ok); err != nil {
		// The answer is lost, so the session may hold the lock: destroy it.
		conn.Hijack().Close(context.WithoutCancel(ctx))
		return nil, false, fmt.Errorf("pgstore: keyed lock %q: %w", key, err)
	}
	if !ok {
		conn.Release()
		return nil, false, nil
	}
	return &HeldLock{conn: conn, key: key}, true, nil
}

// HeldLock is a keyed lock taken with [TryKeyedLock].
type HeldLock struct {
	mu   sync.Mutex
	conn *pgxpool.Conn
	key  string
}

// Check reports whether the lock's session is still alive, with a round
// trip on its connection. A session lock lives exactly as long as its
// session, so a live session still holds the lock. On failure the
// connection is destroyed and the lock counts as lost: Check keeps failing
// and Release does nothing.
func (l *HeldLock) Check(ctx context.Context) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.conn == nil {
		return fmt.Errorf("pgstore: keyed lock %q is no longer held", l.key)
	}
	if _, err := l.conn.Exec(ctx, `SELECT 1`); err != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		l.conn.Hijack().Close(context.WithoutCancel(ctx))
		l.conn = nil
		return fmt.Errorf("pgstore: keyed lock %q lost its session: %w", l.key, err)
	}
	return nil
}

// Release frees the lock and returns its connection to the pool. Safe to
// call more than once.
func (l *HeldLock) Release() {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.conn == nil {
		return
	}
	conn := l.conn
	l.conn = nil
	ctx := context.Background()
	if _, err := conn.Exec(ctx,
		`SELECT pg_advisory_unlock($1::int, hashtext(current_schema() || '/' || $2::text))`,
		keyedLockClassKey, l.key,
	); err != nil {
		slog.Warn("pgstore: keyed advisory unlock failed", "key", l.key, "error", err)
		conn.Hijack().Close(ctx)
		return
	}
	conn.Release()
}

// keyedLockSlots caps how many keyed locks one store holds at once: half the
// pool, at least one. Each holder pins a connection and needs another to
// write, so capping holders here leaves connections for their writes and for
// unrelated requests (see the C+1 <= MaxConns note in
// [Store.AcquireKeyedLock]). Nil for a store without a pool, which cannot take
// a keyed lock anyway.
func keyedLockSlots(db DBTX) chan struct{} {
	pool, isPool := db.(*pgxpool.Pool)
	if !isPool {
		return nil
	}
	return make(chan struct{}, max(1, int(pool.Config().MaxConns)/2))
}

// takeKeyedSlot waits for a free slot in slots and returns the function that
// frees it, or ctx's error if ctx ends first.
func takeKeyedSlot(ctx context.Context, slots chan struct{}) (func(), error) {
	select {
	case slots <- struct{}{}:
		return func() { <-slots }, nil
	case <-ctx.Done():
		return nil, fmt.Errorf("pgstore: keyed lock wait: %w", ctx.Err())
	}
}

// KeyedLocker is the optional store capability [Store.AcquireKeyedLock]
// implements. Declared here so a consumer can type-assert for it without
// importing internal/lock, and so a second postgres-shaped backend can satisfy
// it (the TKT-L3FNEN "discover by interface, not by concrete type" rule).
type KeyedLocker interface {
	AcquireKeyedLock(ctx context.Context, key string) (release func(), err error)
}

// KeyedLockerFor returns st as a [KeyedLocker], or nil when the store does not
// support cross-process keyed locking. Genuinely nil (not a typed nil) so a
// caller's nil-check falls back to the in-process locker.
func KeyedLockerFor(st any) KeyedLocker {
	if kl, ok := st.(KeyedLocker); ok {
		return kl
	}
	return nil
}
