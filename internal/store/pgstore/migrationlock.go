package pgstore

import (
	"context"
	"errors"
	"sync"

	"github.com/jackc/pgx/v5/pgxpool"
)

// migrationAdvisoryLockKey identifies the data-migration lock (TKT-CPCBR7):
// mutual exclusion between `rela migrate data --apply`, GC applies, and gate
// marker writes across processes sharing one schema. Deliberately NOT
// sweepAdvisoryLockKey: a version-sweep tick holds its key for the duration
// of a capture pass, and sharing would make an operator's apply fail fast
// with a misleading "another migration is running" whenever a tick happens
// to be in flight — while the sweep-vs-migration interplay needs no
// exclusion (update captures dedup by content hash; deletes capture their
// snapshot synchronously before the row goes).
//
// Like every pgstore advisory key, it is used in the two-key
// schema-qualified form (`hashtext(current_schema())` as the second key) so
// tenants sharing a database do not exclude each other (the BUG-CA3VY0
// class).
const migrationAdvisoryLockKey int64 = 0x52_45_4c_4d // "RELM"

// TryMigrationLock attempts the schema-scoped data-migration advisory lock.
// ok=false means another holder is active (not an error). On success the
// lock is session-scoped and lives on ONE pool connection held until release
// — handing it back to the pool earlier would silently void the guarantee
// (the sweep-tick rule). release is safe to call more than once.
//
// This is an optional store capability discovered by type-assert
// (internal/datamigration's consumer-side interface), like Formatter and the
// version capabilities — not part of store.Store.
func (s *Store) TryMigrationLock(ctx context.Context) (release func(), ok bool, err error) {
	pool, isPool := s.db.(*pgxpool.Pool)
	if !isPool {
		// A bare handle (unit-test store) has no pool to pin a session lock
		// to. Error rather than pretend: a caller that got this far chose
		// the store-backed lock and must not fall back to nothing silently.
		return nil, false, errors.New("pgstore: migration lock requires a pgxpool-backed store")
	}
	if pool.Config().MaxConns < 2 {
		// The lock pins one connection for the whole run while the
		// migration's own Tx batches acquire more from the same pool — at
		// pool_max_conns=1 that is a guaranteed self-deadlock, so refuse up
		// front with the remedy in the message.
		return nil, false, errors.New(
			"pgstore: migration lock needs pool_max_conns >= 2 (the lock pins one connection while migration writes use others)")
	}
	conn, err := pool.Acquire(ctx)
	if err != nil {
		return nil, false, err
	}
	locked, err := tryAdvisoryLock(ctx, conn, migrationAdvisoryLockKey)
	if err != nil || !locked {
		conn.Release()
		return nil, false, err
	}
	var releaseOnce sync.Once
	return func() {
		releaseOnce.Do(func() {
			// Detached ctx so the unlock still runs during shutdown; releasing
			// the connection would drop the session lock anyway, but an explicit
			// unlock keeps the pair symmetric (the advisoryUnlock contract).
			advisoryUnlock(context.WithoutCancel(ctx), conn, migrationAdvisoryLockKey)
			conn.Release()
		})
	}, true, nil
}
