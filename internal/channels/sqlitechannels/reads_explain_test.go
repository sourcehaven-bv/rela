package sqlitechannels_test

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/channels/sqlchannels"
	"github.com/Sourcehaven-BV/rela/internal/sqlitedb"
)

// TestReads_UsesIndexes pins the plans of the read marker queries
// (TKT-2B2AIA AC 10) that sqlchannels runs ([sqlchannels.ReadSQL]), on a
// fresh database with no ANALYZE: the unread count of SetRead and of a
// Reads chunk searches channel_messages_counts_idx, a Reads chunk finds
// its channels through the key index, and the read window searches
// channel_reads_reader_idx rather than reading every marker.
func TestReads_UsesIndexes(t *testing.T) {
	ctx := context.Background()
	db, err := sqlitedb.Open(ctx, sqlitedb.Options{Path: filepath.Join(t.TempDir(), "rela.db")})
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	sql := sqlchannels.ReadSQL(2)

	plan := explain(t, db, sql.Mark, int64(1), "alice", "")
	require.Contains(t, plan, "channel_messages_counts_idx")
	require.NotContains(t, plan, "SCAN m\n", "the count must not scan the messages")

	plan = explain(t, db, sql.Reads, "alice", "", "TKT-1", "", "room", "", "TKT-2", "", "room", "")
	require.Contains(t, plan, "channel_messages_counts_idx")
	require.NotContains(t, plan, "SCAN m\n", "the count must not scan the messages")
	require.NotContains(t, plan, "SCAN c\n", "the chunk must not scan every channel")
	require.NotContains(t, plan, "SCAN r\n", "the chunk must not scan every marker")

	plan = explain(t, db, sql.Window, "alice", "", 500)
	require.Contains(t, plan, "channel_reads_reader_idx")
	require.NotContains(t, plan, "SCAN r\n", "the window must not scan every marker")
}
