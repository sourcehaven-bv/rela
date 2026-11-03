package pgstore_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/store/pgstore"
)

// TestMigration_SessionPrevious pins migration 0040 (TKT-56TPOTE): a
// schema at version 39 gains channels.session_previous, a row written
// before it reads empty, and a second run changes nothing.
func TestMigration_SessionPrevious(t *testing.T) {
	pool := newScopedPool(t) // migrated to the latest version
	ctx := context.Background()
	for _, stmt := range []string{
		`ALTER TABLE channels DROP COLUMN session_previous`,
		`UPDATE schema_version SET version = 39`,
	} {
		_, err := pool.Exec(ctx, stmt)
		require.NoError(t, err, stmt)
	}
	_, err := pool.Exec(ctx, `INSERT INTO channels (id, anchor_id, type, state, created_by, created_at, closed_at,
		archived_at, reopened_at, session_state, session_gen, session_cause) VALUES (1, 'TKT-1', 'room', 'open', '{}',
		10, 0, 0, 0, 'asking', 3, 7)`)
	require.NoError(t, err)

	require.NoError(t, pgstore.Migrate(ctx, pool))
	require.NoError(t, pgstore.Migrate(ctx, pool), "a second run is harmless")

	var previous string
	require.NoError(t, pool.QueryRow(ctx, `SELECT session_previous FROM channels WHERE id = 1`).Scan(&previous))
	require.Empty(t, previous, "no backfill")
	current, target, err := pgstore.Status(ctx, pool)
	require.NoError(t, err)
	require.Equal(t, 40, current)
	require.Equal(t, target, current)
}
