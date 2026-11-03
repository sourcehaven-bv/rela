package pgstore_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/store/pgstore"
)

// TestMigration_SessionCause pins migration 0038 (TKT-V05706X AC 1): a
// schema at version 37 gains channels.session_cause, a row written before
// it reads 0, and a second run changes nothing.
func TestMigration_SessionCause(t *testing.T) {
	pool := newScopedPool(t) // migrated to the latest version
	ctx := context.Background()
	for _, stmt := range []string{
		`ALTER TABLE channels DROP COLUMN session_cause`,
		`UPDATE schema_version SET version = 37`,
	} {
		_, err := pool.Exec(ctx, stmt)
		require.NoError(t, err, stmt)
	}
	_, err := pool.Exec(ctx, `INSERT INTO channels (id, anchor_id, type, state, created_by, created_at, closed_at,
		archived_at, reopened_at, session_state, session_gen) VALUES (1, 'TKT-1', 'room', 'open', '{}', 10, 0, 0, 0,
		'asking', 3)`)
	require.NoError(t, err)

	require.NoError(t, pgstore.Migrate(ctx, pool))
	require.NoError(t, pgstore.Migrate(ctx, pool), "a second run is harmless")

	var cause int64
	require.NoError(t, pool.QueryRow(ctx, `SELECT session_cause FROM channels WHERE id = 1`).Scan(&cause))
	require.Zero(t, cause, "no backfill")
	current, target, err := pgstore.Status(ctx, pool)
	require.NoError(t, err)
	require.Equal(t, 38, current)
	require.Equal(t, target, current)
}
