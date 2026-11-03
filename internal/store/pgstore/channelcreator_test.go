package pgstore_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/store/pgstore"
)

// TestMigration0034_ChannelCreatorBackfill pins migration 0034 (TKT-ZZ4PNX
// AC 4): a schema at version 33 gains created_by_user and the creator
// index; a row with a valid author gets its user; a row whose created_by
// holds the \u0000 escape jsonb refuses, or is not JSON, keeps the empty string instead
// of failing the migration; and the migration leaves no helper function.
func TestMigration0034_ChannelCreatorBackfill(t *testing.T) {
	pool := newScopedPool(t) // migrated to the latest version
	ctx := context.Background()
	for _, stmt := range []string{
		`DROP INDEX channels_creator_idx`,
		`ALTER TABLE channels DROP COLUMN created_by_user`,
		`UPDATE schema_version SET version = 33`,
	} {
		_, err := pool.Exec(ctx, stmt)
		require.NoError(t, err, stmt)
	}
	rows := map[string]string{
		"TKT-1": `{"user":"alice","tool":"cli"}`,
		"TKT-2": `{"user":"bob\u0000x"}`,
		"TKT-3": `not json`,
		"TKT-4": `{"tool":"cli"}`,
	}
	for id, by := range rows {
		_, err := pool.Exec(ctx, `INSERT INTO channels (anchor_id, type, state, created_by, created_at)
			VALUES ($1, 'room', 'open', $2, 1)`, id, by)
		require.NoError(t, err)
	}

	require.NoError(t, pgstore.Migrate(ctx, pool))
	require.NoError(t, pgstore.Migrate(ctx, pool), "a second run is harmless")

	want := map[string]string{"TKT-1": "alice", "TKT-2": "", "TKT-3": "", "TKT-4": ""}
	for id, user := range want {
		var got string
		require.NoError(t, pool.QueryRow(ctx, `SELECT created_by_user FROM channels WHERE anchor_id = $1`, id).
			Scan(&got))
		require.Equal(t, user, got, id)
	}
	var indexes, functions int
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM pg_indexes
		WHERE schemaname = current_schema() AND indexname = 'channels_creator_idx'`).Scan(&indexes))
	require.Equal(t, 1, indexes)
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM pg_proc WHERE proname = 'rela_creator_user'`).
		Scan(&functions))
	require.Zero(t, functions)
	current, target, err := pgstore.Status(ctx, pool)
	require.NoError(t, err)
	require.Equal(t, target, current)
}
