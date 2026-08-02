package pgstore_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/store/pgstore"
)

// The migrate and write locks are schema-scoped for the same reason the version
// sweep's is (see TestSweepAdvisoryLockIsSchemaScoped): PostgreSQL advisory
// locks are database-GLOBAL, and many rela schemas can share one database.
//
// Unqualified, the write lock made every write transaction serialize against
// writes to UNRELATED schemas, and the migrate lock made two schemas' startup
// migrations queue behind each other. Neither loses data the way the sweep did,
// but both are the same defect.
const (
	migrateLockKey = 0x52_45_4c_41 // migrateAdvisoryLockKey ("RELA")
	writeLockKey   = 0x52_45_4c_57 // writeAdvisoryLockKey ("RELW")
)

// TestXactAdvisoryLocksAreSchemaScoped proves the two transaction-scoped locks
// (migrate, write) are independent across schemas while staying mutually
// exclusive within one.
//
// pg_advisory_xact_lock is blocking, so this asserts via pg_try_advisory_xact_lock
// on a held transaction rather than by racing two blocking acquisitions.
func TestXactAdvisoryLocksAreSchemaScoped(t *testing.T) {
	for _, tc := range []struct {
		name string
		key  int
	}{
		{"migrate", migrateLockKey},
		{"write", writeLockKey},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			poolA := newScopedPool(t)
			poolB := newScopedPool(t)

			const lockSQL = `SELECT pg_try_advisory_xact_lock($1::int, hashtext(current_schema()))`

			// Hold the lock inside an open transaction on schema A.
			connA, err := poolA.Acquire(ctx)
			require.NoError(t, err)
			defer connA.Release()
			txA, err := connA.Begin(ctx)
			require.NoError(t, err)
			defer txA.Rollback(ctx)

			var okA bool
			require.NoError(t, txA.QueryRow(ctx, lockSQL, tc.key).Scan(&okA))
			require.True(t, okA, "schema A takes the %s lock", tc.name)

			// Schema B takes the SAME logical lock concurrently — only possible if
			// it is scoped by schema.
			connB, err := poolB.Acquire(ctx)
			require.NoError(t, err)
			defer connB.Release()
			txB, err := connB.Begin(ctx)
			require.NoError(t, err)
			defer txB.Rollback(ctx)

			var okB bool
			require.NoError(t, txB.QueryRow(ctx, lockSQL, tc.key).Scan(&okB))
			require.True(t, okB,
				"schema B must take the %s lock while A holds it — scoped, not global", tc.name)

			// Within schema A, a second session is still excluded.
			connA2, err := poolA.Acquire(ctx)
			require.NoError(t, err)
			defer connA2.Release()
			txA2, err := connA2.Begin(ctx)
			require.NoError(t, err)
			defer txA2.Rollback(ctx)

			var okA2 bool
			require.NoError(t, txA2.QueryRow(ctx, lockSQL, tc.key).Scan(&okA2))
			require.False(t, okA2,
				"a second session in schema A must still be excluded from the %s lock", tc.name)
		})
	}
}

// TestMigrateDoesNotBlockAnotherSchema pins the migrate lock end-to-end through
// the real Migrate call: a migration against one schema must not be serialized
// behind an unrelated schema holding the migrate lock.
//
// It holds BOTH lock forms on schema A — the schema-scoped two-key form that
// production takes now, and the bare one-key form it took before. Holding both
// is what makes the test falsifying: with only the two-key form held, a
// regression back to the bare key would take a lock nobody holds and pass
// anyway, since Postgres treats the one-key and two-key spaces as disjoint.
//
// Migrate is idempotent, so re-running it on an already-migrated schema is a
// no-op that still takes the lock — exactly what is under test.
func TestMigrateDoesNotBlockAnotherSchema(t *testing.T) {
	ctx := context.Background()
	poolA := newScopedPool(t)
	poolB := newScopedPool(t)

	// Hold schema A's migrate lock — both forms — for an open transaction.
	connA, err := poolA.Acquire(ctx)
	require.NoError(t, err)
	defer connA.Release()
	txA, err := connA.Begin(ctx)
	require.NoError(t, err)
	defer txA.Rollback(ctx)

	var held bool
	require.NoError(t, txA.QueryRow(ctx,
		`SELECT pg_try_advisory_xact_lock($1::int, hashtext(current_schema()))`,
		migrateLockKey).Scan(&held))
	require.True(t, held, "should hold schema A's scoped migrate lock")

	// The pre-fix key space. Holding it means a regression to the bare form
	// blocks here rather than silently passing.
	require.NoError(t, txA.QueryRow(ctx,
		`SELECT pg_try_advisory_xact_lock($1)`, migrateLockKey).Scan(&held))
	require.True(t, held, "should hold the legacy database-global migrate key too")

	// Migrating schema B must complete promptly. With a database-global key it
	// would block on A's transaction until this test's timeout.
	done := make(chan error, 1)
	go func() { done <- pgstore.Migrate(ctx, poolB) }()

	select {
	case err := <-done:
		require.NoError(t, err, "schema B must migrate while schema A holds ITS migrate lock")
	case <-time.After(10 * time.Second):
		t.Fatal("schema B's migration blocked on schema A's migrate lock — the key is not schema-scoped")
	}
}

// TestWriteTxDoesNotBlockAnotherSchema is the write-lock analog, driven
// through the real Store.Tx rather than a mirrored SQL literal. Same
// both-forms-held construction, for the same falsifiability reason.
func TestWriteTxDoesNotBlockAnotherSchema(t *testing.T) {
	ctx := context.Background()
	poolA := newScopedPool(t)
	poolB := newScopedPool(t)

	connA, err := poolA.Acquire(ctx)
	require.NoError(t, err)
	defer connA.Release()
	txA, err := connA.Begin(ctx)
	require.NoError(t, err)
	defer txA.Rollback(ctx)

	var held bool
	require.NoError(t, txA.QueryRow(ctx,
		`SELECT pg_try_advisory_xact_lock($1::int, hashtext(current_schema()))`,
		writeLockKey).Scan(&held))
	require.True(t, held, "should hold schema A's scoped write lock")
	require.NoError(t, txA.QueryRow(ctx,
		`SELECT pg_try_advisory_xact_lock($1)`, writeLockKey).Scan(&held))
	require.True(t, held, "should hold the legacy database-global write key too")

	sB, err := pgstore.New(poolB)
	require.NoError(t, err)
	t.Cleanup(func() { _ = sB.Close() })

	done := make(chan error, 1)
	go func() {
		done <- sB.Tx(ctx, func(tx store.Store) error {
			return tx.CreateEntity(ctx, mkEntity("TENANT-B-TX", "ticket", "written under Tx"))
		})
	}()

	select {
	case err := <-done:
		require.NoError(t, err, "schema B must write while schema A holds ITS write lock")
	case <-time.After(10 * time.Second):
		t.Fatal("schema B's write Tx blocked on schema A's write lock — the key is not schema-scoped")
	}
}

// TestSweepCapturesWhileAnotherSchemaHoldsLock complements the lock-level
// assertion in TestSweepAdvisoryLockIsSchemaScoped by pinning the OBSERVABLE
// consequence: it asserts on captured versions, not on lock acquisition.
//
// That distinction is the point. A test that only checks a lock was taken still
// passes if the keys collide in some future refactor; this one fails, because
// the symptom of the original bug was silent — schema B's create/update
// versions were simply never captured, with no error, warning or metric.
func TestSweepCapturesWhileAnotherSchemaHoldsLock(t *testing.T) {
	ctx := context.Background()
	poolA := newScopedPool(t)
	poolB := newScopedPool(t)

	// Hold schema A's version lock on a dedicated session — what a mid-tick
	// sweep in A looks like to the rest of the database.
	connA, err := poolA.Acquire(ctx)
	require.NoError(t, err)
	defer connA.Release()

	const lockSQL = `SELECT pg_try_advisory_lock($1::int, hashtext(current_schema()))`
	const sweepLockKey = 0x52_45_4c_56 // sweepAdvisoryLockKey ("RELV")

	var lockedA bool
	require.NoError(t, connA.QueryRow(ctx, lockSQL, sweepLockKey).Scan(&lockedA))
	require.True(t, lockedA, "should have taken schema A's version lock")
	defer func() {
		_, _ = connA.Exec(context.Background(),
			`SELECT pg_advisory_unlock($1::int, hashtext(current_schema()))`, sweepLockKey)
	}()

	// Sanity: A's lock is genuinely held, so B is not merely succeeding because
	// nothing was locked at all.
	connA2, err := poolA.Acquire(ctx)
	require.NoError(t, err)
	var reAcquired bool
	require.NoError(t, connA2.QueryRow(ctx, lockSQL, sweepLockKey).Scan(&reAcquired))
	if reAcquired {
		_, _ = connA2.Exec(ctx,
			`SELECT pg_advisory_unlock($1::int, hashtext(current_schema()))`, sweepLockKey)
	}
	connA2.Release()
	require.False(t, reAcquired, "schema A's lock should be exclusively held")

	// Sweep schema B while A's lock is held. B must capture regardless.
	sB, err := pgstore.New(poolB)
	require.NoError(t, err)
	t.Cleanup(func() { _ = sB.Close() })

	require.NoError(t, sB.CreateEntity(ctx, mkEntity("TENANT-B-1", "ticket", "b writes too")))
	_, err = poolB.Exec(ctx,
		`UPDATE entities SET updated_at = now() - interval '1 hour' WHERE id = 'TENANT-B-1'`)
	require.NoError(t, err)

	sB.StartVersionSweep(
		stubProvider{hash: "schema-abc", json: []byte(`{"entities":{},"types":{}}`)},
		pgstore.SweepConfig{
			Interval:     50 * time.Millisecond,
			Idle:         time.Minute,
			MaxStaleness: time.Hour,
			Batch:        100,
		})

	require.Eventually(t, func() bool {
		metas, e := sB.VersionStore().ListVersions(ctx, "TENANT-B-1")
		return e == nil && len(metas) == 1
	}, 5*time.Second, 25*time.Millisecond,
		"schema B must capture its own versions while schema A holds ITS version lock; "+
			"a database-global key silently drops B's capture")
}
