package pgstore_test

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/store/pgstore"
)

// TestStatusFreshSchemaReportsZero verifies Status reports current=0 against a
// schema that has not been migrated yet (the schema_version table doesn't
// exist), without erroring — the "fresh database" path runDBStatus relies on.
func TestStatusFreshSchema(t *testing.T) {
	admin := adminConn(t)
	ctx := context.Background()

	// A bare schema with no pgstore tables — simulate an un-migrated database.
	// search_path is the test schema ONLY (no public): Status doesn't need
	// pg_trgm, and excluding public keeps the test robust even if the target
	// database shares its public schema with another application.
	schema := freshEmptySchema(t, admin)
	cfg, err := pgxpool.ParseConfig(testDSN(t))
	require.NoError(t, err)
	cfg.ConnConfig.RuntimeParams["search_path"] = schema
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	require.NoError(t, err)
	t.Cleanup(pool.Close)

	current, target, err := pgstore.Status(ctx, pool)
	require.NoError(t, err)
	require.Equal(t, 0, current, "un-migrated schema is version 0")
	require.Equal(t, 2, target, "binary embeds migrations through 0002")
}

// TestStatusAfterMigrate verifies Status reports current==target once Migrate
// has run.
func TestStatusAfterMigrate(t *testing.T) {
	pool := newScopedPool(t) // newScopedPool already migrates
	ctx := context.Background()

	current, target, err := pgstore.Status(ctx, pool)
	require.NoError(t, err)
	require.Equal(t, target, current, "migrated schema is at the target version")
	require.GreaterOrEqual(t, current, 1)
}
