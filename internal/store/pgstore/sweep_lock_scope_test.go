package pgstore_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestSweepAdvisoryLockIsSchemaScoped is the regression for the global-lock
// starvation the postgres e2e surfaced: the version sweep's advisory lock is a
// fixed key, and PostgreSQL advisory locks are database-GLOBAL. Many schemas run
// on one database (this conformance harness, and the postgres e2e's isolated
// schemas), so if the lock weren't schema-scoped, one schema's sweep would starve
// another's — two servers ticking on the same DB, only one capturing per tick.
//
// The fix folds hashtext(current_schema()) into the lock as the two-key form
// pg_try_advisory_lock(key, hashtext(current_schema())). This test proves two
// distinct schemas can hold the version lock SIMULTANEOUSLY, and that within one
// schema it is still mutually exclusive.
func TestSweepAdvisoryLockIsSchemaScoped(t *testing.T) {
	// Two independent scoped pools == two distinct schemas on one database.
	poolA := newScopedPool(t)
	poolB := newScopedPool(t)
	ctx := context.Background()

	// Mirror the sweep's exact lock statement.
	const lockSQL = `SELECT pg_try_advisory_lock($1::int, hashtext(current_schema()))`
	const key = 0x52_45_4c_56 // sweepAdvisoryLockKey

	connA, err := poolA.Acquire(ctx)
	require.NoError(t, err)
	defer connA.Release()
	connB, err := poolB.Acquire(ctx)
	require.NoError(t, err)
	defer connB.Release()

	var okA, okB bool
	require.NoError(t, connA.QueryRow(ctx, lockSQL, key).Scan(&okA))
	require.True(t, okA, "schema A acquires the version lock")

	// Schema B acquires the SAME logical lock concurrently — only possible if the
	// lock is scoped by schema (else B would be blocked by A holding it globally).
	require.NoError(t, connB.QueryRow(ctx, lockSQL, key).Scan(&okB))
	require.True(t, okB, "schema B acquires concurrently — lock is schema-scoped, not global")

	// Within schema A, a SECOND session must still be excluded (single-writer).
	connA2, err := poolA.Acquire(ctx)
	require.NoError(t, err)
	defer connA2.Release()
	var okA2 bool
	require.NoError(t, connA2.QueryRow(ctx, lockSQL, key).Scan(&okA2))
	require.False(t, okA2, "a second session in schema A is excluded — still mutually exclusive within a schema")
}
