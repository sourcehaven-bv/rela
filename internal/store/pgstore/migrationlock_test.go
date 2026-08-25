package pgstore_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/store/pgstore"
)

// TestMigrationLock_ExclusiveWithinSchema pins TKT-CPCBR7 AC1: two stores on
// the SAME schema exclude each other; release frees the lock and is safe to
// call twice.
func TestMigrationLock_ExclusiveWithinSchema(t *testing.T) {
	pool := newScopedPool(t)
	a, err := pgstore.New(pool)
	require.NoError(t, err)
	b, err := pgstore.New(pool)
	require.NoError(t, err)
	ctx := context.Background()

	releaseA, ok, err := a.TryMigrationLock(ctx)
	require.NoError(t, err)
	require.True(t, ok, "first acquire must succeed")

	_, ok, err = b.TryMigrationLock(ctx)
	require.NoError(t, err)
	require.False(t, ok, "second store on the same schema must see the lock held")

	releaseA()
	releaseA() // double release must be safe

	releaseB, ok, err := b.TryMigrationLock(ctx)
	require.NoError(t, err)
	require.True(t, ok, "acquire after release must succeed")
	releaseB()
}

// TestMigrationLock_SchemasAreIndependent pins TKT-CPCBR7 AC2 (the
// BUG-CA3VY0 regression class): tenants sharing one database must not
// exclude each other — the lock is schema-qualified.
func TestMigrationLock_SchemasAreIndependent(t *testing.T) {
	a, err := pgstore.New(newScopedPool(t))
	require.NoError(t, err)
	b, err := pgstore.New(newScopedPool(t))
	require.NoError(t, err)
	ctx := context.Background()

	releaseA, ok, err := a.TryMigrationLock(ctx)
	require.NoError(t, err)
	require.True(t, ok)
	defer releaseA()

	releaseB, ok, err := b.TryMigrationLock(ctx)
	require.NoError(t, err)
	require.True(t, ok, "a different schema's lock must be independent")
	releaseB()
}
