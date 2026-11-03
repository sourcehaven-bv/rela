package sqlitechannels_test

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/channels/sqlchannels"
	"github.com/Sourcehaven-BV/rela/internal/sqlitedb"
)

// TestGetMessages_UsesIndexes pins the plans of the two GetMessages
// statements ([sqlchannels.GetMessagesSQL]) on a fresh database with no
// ANALYZE (TKT-V05706X AC 2): the channels come from the key index and the
// messages from the primary key, never a scan of either table.
func TestGetMessages_UsesIndexes(t *testing.T) {
	ctx := context.Background()
	db, err := sqlitedb.Open(ctx, sqlitedb.Options{Path: filepath.Join(t.TempDir(), "rela.db")})
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	chans, msgs := sqlchannels.GetMessagesSQL(2, 3)

	plan := explain(t, db, chans, "TKT-1", "", "room", "", "TKT-2", "", "room", "")
	require.NotContains(t, plan, "SCAN channels\n", "the channels must come from the key index")
	require.Contains(t, plan, "SEARCH channels USING")

	plan = explain(t, db, msgs, int64(1), int64(4), int64(1), int64(5), int64(2), int64(1))
	require.NotContains(t, plan, "SCAN channel_messages\n", "the messages must come from the primary key")
	require.Contains(t, plan, "SEARCH channel_messages USING")
}
