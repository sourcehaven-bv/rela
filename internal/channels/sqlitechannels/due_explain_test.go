package sqlitechannels_test

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/channels"
	"github.com/Sourcehaven-BV/rela/internal/channels/channelstest"
	"github.com/Sourcehaven-BV/rela/internal/channels/sqlitechannels"
	"github.com/Sourcehaven-BV/rela/internal/sqlitedb"
)

// dueQuery is the WHERE and ORDER BY of sqlchannels' ListSessionsDue for
// two types. It is a copy: keep it in step with that query.
const dueQuery = `SELECT id FROM channels
	WHERE session_next_due_at > 0 AND tomb = '' AND type IN (?, ?)
	AND state = ? AND session_next_due_at <= ?
	ORDER BY session_next_due_at, anchor_id, anchor_face, type, name LIMIT ?`

// TestListSessionsDue_UsesIndex pins the plan shape of the due query, in
// the style of sqlitestore's EXPLAIN tests: on a fresh database with no
// ANALYZE, SQLite picks channels_session_due_idx from the query alone.
func TestListSessionsDue_UsesIndex(t *testing.T) {
	ctx := context.Background()
	db, err := sqlitedb.Open(ctx, sqlitedb.Options{Path: filepath.Join(t.TempDir(), "rela.db")})
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	s, err := sqlitechannels.New(db.DB())
	require.NoError(t, err)
	for i := range 20 {
		k := channels.Key{Anchor: channels.Anchor{ID: fmt.Sprintf("TKT-%d", i)}, Type: "room"}
		_, err = s.Append(ctx, k, channels.NewMessage{Type: "text", Author: channelstest.Alice, At: channelstest.T0})
		require.NoError(t, err)
	}

	rows, err := db.DB().QueryContext(ctx, "EXPLAIN QUERY PLAN "+dueQuery,
		"room", "topic", string(channels.StateOpen), int64(1), 256)
	require.NoError(t, err)
	defer func() { _ = rows.Close() }()
	var plan strings.Builder
	for rows.Next() {
		var id, parent, unused int
		var detail string
		require.NoError(t, rows.Scan(&id, &parent, &unused, &detail))
		plan.WriteString(detail + "\n")
	}
	require.NoError(t, rows.Err())
	t.Logf("plan:\n%s", plan.String())
	require.Contains(t, plan.String(), "channels_session_due_idx")
	require.NotContains(t, plan.String(), "SCAN channels\n", "the due query must not scan the table")
}

// TestListSessionsDue_SkipsDamagedRow runs the damaged-row check with
// invalid JSON in session_restarts and in created_by.
func TestListSessionsDue_SkipsDamagedRow(t *testing.T) {
	for _, col := range []string{"session_restarts", "created_by"} {
		t.Run(col, func(t *testing.T) {
			ctx := context.Background()
			db, err := sqlitedb.Open(ctx, sqlitedb.Options{Path: filepath.Join(t.TempDir(), "rela.db")})
			require.NoError(t, err)
			t.Cleanup(func() { _ = db.Close() })
			s, err := sqlitechannels.New(db.DB())
			require.NoError(t, err)
			channelstest.RunDamagedDueRow(t, s, func(t *testing.T, k channels.Key) {
				t.Helper()
				_, err := db.DB().ExecContext(ctx, `UPDATE channels SET `+col+` = '{not json' WHERE anchor_id = ?`, k.ID)
				require.NoError(t, err)
			})
		})
	}
}
