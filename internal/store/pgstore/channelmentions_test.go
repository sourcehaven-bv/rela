package pgstore_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/store/pgstore"
)

// TestMigration0037_ChannelMentions pins migration 0037 (TKT-6RXAC0
// AC 9): a schema at version 36 gains channel_mentions and its three indexes,
// old messages gain no rows, and a second run changes nothing.
func TestMigration0037_ChannelMentions(t *testing.T) {
	pool := newScopedPool(t) // migrated to the latest version
	ctx := context.Background()
	for _, stmt := range []string{
		`DROP INDEX channel_mentions_target_idx`,
		`DROP INDEX channel_mentions_cap_idx`,
		`DROP INDEX channel_mentions_kind_idx`,
		`DROP TABLE channel_mentions`,
		`UPDATE schema_version SET version = 36`,
	} {
		_, err := pool.Exec(ctx, stmt)
		require.NoError(t, err, stmt)
	}
	_, err := pool.Exec(ctx, `INSERT INTO channels (id, anchor_id, type, state, created_by, created_at, closed_at,
		archived_at, reopened_at) VALUES (1, 'TKT-1', 'room', 'open', '{}', 10, 0, 0, 0)`)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `INSERT INTO channel_messages (channel_id, seq, rev, doc)
		VALUES (1, 1, 1, '{"type":"text","props":{"body":"`+"`USR-7`"+`"}}')`)
	require.NoError(t, err)

	require.NoError(t, pgstore.Migrate(ctx, pool))
	require.NoError(t, pgstore.Migrate(ctx, pool), "a second run is harmless")

	var indexes, rows int
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM pg_indexes WHERE schemaname = current_schema()
		AND indexname IN ('channel_mentions_target_idx', 'channel_mentions_kind_idx',
		'channel_mentions_cap_idx')`).Scan(&indexes))
	require.Equal(t, 3, indexes)
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM channel_mentions`).Scan(&rows))
	require.Zero(t, rows, "no backfill")
	current, target, err := pgstore.Status(ctx, pool)
	require.NoError(t, err)
	require.Equal(t, 37, current)
	require.Equal(t, target, current)
}
