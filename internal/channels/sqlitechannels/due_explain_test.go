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

	plan := explain(t, db, dueQuery, "room", "topic", string(channels.StateOpen), int64(1), 256)
	require.Contains(t, plan, "channels_session_due_idx")
	require.NotContains(t, plan, "SCAN channels\n", "the due query must not scan the table")
}

// creatorQuery is the creator-branch statement of sqlchannels' ListWaiting
// for two types. It is a copy: keep it in step with that query.
const creatorQuery = `SELECT id FROM channels
	WHERE tomb = '' AND state = ? AND ((type = ? AND session_state IN (?, ?)) OR (type = ? AND session_state IN (?)))
	AND created_by_user <> '' AND created_by_user = ? AND type IN (?, ?) LIMIT ?`

// TestListWaiting_CreatorUsesIndex pins that the creator branch searches
// channels_creator_idx, with channels_state_idx present, rather than
// reading every open channel.
func TestListWaiting_CreatorUsesIndex(t *testing.T) {
	ctx := context.Background()
	db, err := sqlitedb.Open(ctx, sqlitedb.Options{Path: filepath.Join(t.TempDir(), "rela.db")})
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	plan := explain(t, db, creatorQuery,
		string(channels.StateOpen), "case", "", "asked", "topic", "asked", "alice", "case", "topic", 20001)
	require.Contains(t, plan, "USING INDEX channels_creator_idx")
	require.NotContains(t, plan, "SCAN channels\n", "the creator branch must not scan the table")
}

// explain returns the EXPLAIN QUERY PLAN details of query, one per line.
func explain(t *testing.T, db *sqlitedb.DB, query string, args ...any) string {
	t.Helper()
	rows, err := db.DB().QueryContext(context.Background(), "EXPLAIN QUERY PLAN "+query, args...)
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
	return plan.String()
}

// TestListSessionsDue_SkipsDamagedRow runs the damaged-row check with
// invalid JSON in session_restarts, session_reminders and created_by, and
// with a reminder record that decodes but is invalid.
func TestListSessionsDue_SkipsDamagedRow(t *testing.T) {
	for _, tc := range []struct{ name, col, val string }{
		{"session_restarts", "session_restarts", "{not json"},
		{"session_reminders", "session_reminders", "{not json"},
		{"created_by", "created_by", "{not json"},
		{"negative reminder count", "session_reminders", `{"status-update":{"since":1,"count":-1}}`},
		{"invalid reminder name", "session_reminders", `{"Bad Name":{"since":1,"count":0}}`},
	} {
		col := tc.col
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			db, err := sqlitedb.Open(ctx, sqlitedb.Options{Path: filepath.Join(t.TempDir(), "rela.db")})
			require.NoError(t, err)
			t.Cleanup(func() { _ = db.Close() })
			s, err := sqlitechannels.New(db.DB())
			require.NoError(t, err)
			channelstest.RunDamagedDueRow(t, s, func(t *testing.T, k channels.Key) {
				t.Helper()
				_, err := db.DB().ExecContext(ctx, `UPDATE channels SET `+col+` = ? WHERE anchor_id = ?`, tc.val, k.ID)
				require.NoError(t, err)
			})
		})
	}
}

// TestListWaiting_DamagedRowsStillFillTheLimit pins that a statement that
// returns its LIMIT is over the ceiling even when some of its rows are
// damaged: rows past the LIMIT were never read, so an answer would be
// partial.
func TestListWaiting_DamagedRowsStillFillTheLimit(t *testing.T) {
	ctx := context.Background()
	db, err := sqlitedb.Open(ctx, sqlitedb.Options{Path: filepath.Join(t.TempDir(), "rela.db")})
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	s, err := sqlitechannels.New(db.DB())
	require.NoError(t, err)
	anchors := make([]channels.Anchor, 0, 4)
	for i := range 4 {
		a := channels.Anchor{ID: fmt.Sprintf("TKT-%d", i)}
		anchors = append(anchors, a)
		_, err = s.Append(ctx, channels.Key{Anchor: a, Type: "room"},
			channels.NewMessage{Type: "text", Author: channelstest.Alice, At: channelstest.T0})
		require.NoError(t, err)
	}
	_, err = db.DB().ExecContext(ctx, `UPDATE channels SET created_by = '{not json' WHERE anchor_id = 'TKT-0'`)
	require.NoError(t, err)
	q := channels.WaitingQuery{States: map[string][]string{"room": {""}}, Anchors: anchors, MaxRows: 4}
	got, err := s.ListWaiting(ctx, q)
	require.NoError(t, err, "four rows stay under a LIMIT of five")
	require.Len(t, got, 3, "the damaged row is skipped")
	q.MaxRows = 3
	_, err = s.ListWaiting(ctx, q)
	require.ErrorIs(t, err, channels.ErrTooManyRows, "three good rows fit the ceiling, but the LIMIT of four was filled")
}
