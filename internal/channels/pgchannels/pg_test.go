package pgchannels_test

import (
	"context"
	"fmt"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/channels"
	"github.com/Sourcehaven-BV/rela/internal/channels/channelstest"
	"github.com/Sourcehaven-BV/rela/internal/channels/pgchannels"
	"github.com/Sourcehaven-BV/rela/internal/channels/sqlchannels"
	"github.com/Sourcehaven-BV/rela/internal/store/pgstore"
)

// testDBEnv and requireDBEnv mirror the pgstore suite's gate: a clean skip
// by default, a failure when the environment promised a database.
const (
	testDBEnv    = "RELA_TEST_DATABASE_URL"
	requireDBEnv = "RELA_TEST_DATABASE_REQUIRED"
)

var (
	adminPoolOnce sync.Once
	adminPool     *pgxpool.Pool
	errAdminPool  error
	schemaCounter atomic.Int64
)

// TestConformance runs the shared contract against PostgreSQL, each test in
// its own schema-pinned pool. Second opens an independent pool on the same
// schema, standing in for a second rela-server process.
func TestConformance(t *testing.T) {
	channelstest.RunAll(t, func(t *testing.T) channelstest.Harness {
		t.Helper()
		pool, dsn := newScopedPool(t)
		s, err := pgchannels.New(pool)
		require.NoError(t, err)
		return channelstest.Harness{
			Store: s,
			// A row from before migration 28: the column default fills
			// cardinality.
			SeedLegacyType: func(t *testing.T, typ string, anchor channels.AnchorMode) {
				t.Helper()
				_, err := pool.Exec(context.Background(),
					`INSERT INTO channel_types (type, anchor_mode) VALUES ($1, $2)`, typ, string(anchor))
				require.NoError(t, err)
			},
			Raw: func(t *testing.T) []byte {
				t.Helper()
				raw, err := sqlchannels.Raw(context.Background(), s)
				require.NoError(t, err)
				return raw
			},
			Second: func(t *testing.T) channels.Store {
				t.Helper()
				other, err := pgxpool.New(context.Background(), dsn)
				require.NoError(t, err)
				t.Cleanup(other.Close)
				s, err := pgchannels.New(other)
				require.NoError(t, err)
				return s
			},
		}
	})
}

func TestNewRejectsNil(t *testing.T) {
	_, err := pgchannels.New(nil)
	require.Error(t, err)
}

// TestSchemaIsolation: a channel in one tenant schema is invisible in
// another, and the per-entity lock of one does not serialize the other.
func TestSchemaIsolation(t *testing.T) {
	ctx := context.Background()
	poolA, _ := newScopedPool(t)
	poolB, _ := newScopedPool(t)
	a, err := pgchannels.New(poolA)
	require.NoError(t, err)
	b, err := pgchannels.New(poolB)
	require.NoError(t, err)

	k := channels.Key{Anchor: channels.Anchor{ID: "TKT-1"}, Type: "room"}
	_, err = a.Append(ctx, k, channels.NewMessage{
		Type: "text", Author: channelstest.Alice, At: channelstest.T0, Props: map[string]any{"body": "x"},
	})
	require.NoError(t, err)
	_, err = b.Head(ctx, k)
	require.ErrorIs(t, err, channels.ErrNotFound)
	keys, err := b.Keys(ctx)
	require.NoError(t, err)
	require.Empty(t, keys)
}

// TestLegacyRowHasNoSession pins that a channel row written without the
// session columns of migration 31 reads as an unset session.
func TestLegacyRowHasNoSession(t *testing.T) {
	ctx := context.Background()
	pool, _ := newScopedPool(t)
	s, err := pgchannels.New(pool)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `INSERT INTO channels
		(anchor_id, anchor_face, type, name, tomb, incarnation, state, title, created_by, created_at,
		 closed_at, archived_at, last_seq, rev)
		VALUES ('TKT-1', '', 'room', '', '', '', 'open', '', '{"user":"alice"}', 1, 0, 0, 0, 0)`)
	require.NoError(t, err)
	c, err := s.Head(ctx, channels.Key{Anchor: channels.Anchor{ID: "TKT-1"}, Type: "room"})
	require.NoError(t, err)
	require.True(t, c.Session.IsZero(), "a legacy row has session %+v", c.Session)
	require.True(t, c.Session.EnteredAt.IsZero())
}

// newScopedPool returns a pool pinned to a fresh, migrated schema, and a
// DSN pinned to the same schema.
func newScopedPool(t *testing.T) (pool *pgxpool.Pool, scopedDSN string) {
	t.Helper()
	admin := adminConn(t)
	ctx := context.Background()

	schema := fmt.Sprintf("relachan_%d_%d", os.Getpid(), schemaCounter.Add(1))
	_, err := admin.Exec(ctx, "CREATE SCHEMA "+quoteIdent(schema))
	require.NoError(t, err)
	t.Cleanup(func() {
		_, _ = admin.Exec(context.Background(), "DROP SCHEMA "+quoteIdent(schema)+" CASCADE")
	})

	cfg, err := pgxpool.ParseConfig(os.Getenv(testDBEnv))
	require.NoError(t, err)
	cfg.ConnConfig.RuntimeParams["search_path"] = schema + ",public"
	cfg.MaxConns = 4
	cfg.MinConns = 0
	pool, err = pgxpool.NewWithConfig(ctx, cfg)
	require.NoError(t, err)
	t.Cleanup(pool.Close)
	require.NoError(t, pgstore.Migrate(ctx, pool))

	scopedDSN = cfg.ConnString()
	if !strings.Contains(scopedDSN, "search_path") {
		sep := "?"
		if strings.Contains(scopedDSN, "?") {
			sep = "&"
		}
		scopedDSN += sep + "search_path=" + schema + ",public"
	}
	return pool, scopedDSN
}

func adminConn(t *testing.T) *pgxpool.Pool {
	t.Helper()
	dsn := os.Getenv(testDBEnv)
	if dsn == "" {
		if v := os.Getenv(requireDBEnv); v != "" && v != "0" && !strings.EqualFold(v, "false") {
			t.Fatalf("%s is set, so the pgchannels suite must run, but %s is empty", requireDBEnv, testDBEnv)
		}
		t.Skipf("%s not set; skipping pgchannels integration tests", testDBEnv)
	}
	adminPoolOnce.Do(func() {
		adminPool, errAdminPool = pgxpool.New(context.Background(), dsn)
	})
	require.NoError(t, errAdminPool)
	return adminPool
}

func quoteIdent(name string) string {
	return `"` + strings.ReplaceAll(name, `"`, `""`) + `"`
}

// TestLegacyRowHasNoSchedule pins that a channel row written without the
// timer columns of migration 32 reads with no due time and no restart
// anchors.
func TestLegacyRowHasNoSchedule(t *testing.T) {
	ctx := context.Background()
	pool, _ := newScopedPool(t)
	s, err := pgchannels.New(pool)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `INSERT INTO channels
		(anchor_id, anchor_face, type, name, tomb, incarnation, state, title, created_by, created_at,
		 closed_at, archived_at, last_seq, rev, session_state, session_gen, session_entered_at)
		VALUES ('TKT-1', '', 'room', '', '', '', 'open', '', '{"user":"alice"}', 1, 0, 0, 0, 0, 'detected', 1, 1)`)
	require.NoError(t, err)
	k := channels.Key{Anchor: channels.Anchor{ID: "TKT-1"}, Type: "room"}
	c, err := s.Head(ctx, k)
	require.NoError(t, err)
	require.Equal(t, "detected", c.Session.State)
	require.True(t, c.Session.NextDueAt.IsZero())
	require.Nil(t, c.Session.Restarts)
	due, err := s.ListSessionsDue(ctx, []string{"room"}, channelstest.T0, 10)
	require.NoError(t, err)
	require.Empty(t, due, "a legacy row has no due time")
}

// TestLegacyRowHasNoReminders pins that a channel row written without the
// reminder column of migration 33 reads with no reminder records, and
// that its schedule still lists.
func TestLegacyRowHasNoReminders(t *testing.T) {
	ctx := context.Background()
	pool, _ := newScopedPool(t)
	s, err := pgchannels.New(pool)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `INSERT INTO channels
		(anchor_id, anchor_face, type, name, tomb, incarnation, state, title, created_by, created_at,
		 closed_at, archived_at, last_seq, rev, session_state, session_gen, session_entered_at,
		 session_next_due_at, session_restarts)
		VALUES ('TKT-1', '', 'room', '', '', '', 'open', '', '{"user":"alice"}', 1, 0, 0, 0, 0, 'detected', 1, 1,
		        1, '')`)
	require.NoError(t, err)
	k := channels.Key{Anchor: channels.Anchor{ID: "TKT-1"}, Type: "room"}
	c, err := s.Head(ctx, k)
	require.NoError(t, err)
	require.Equal(t, "detected", c.Session.State)
	require.Nil(t, c.Session.Reminders)
	due, err := s.ListSessionsDue(ctx, []string{"room"}, channelstest.T0, 10)
	require.NoError(t, err)
	require.Len(t, due, 1)
	require.Nil(t, due[0].Session.Reminders)
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
			pool, _ := newScopedPool(t)
			s, err := pgchannels.New(pool)
			require.NoError(t, err)
			channelstest.RunDamagedDueRow(t, s, func(t *testing.T, k channels.Key) {
				t.Helper()
				_, err := pool.Exec(ctx, `UPDATE channels SET `+col+` = $1 WHERE anchor_id = $2`, tc.val, k.ID)
				require.NoError(t, err)
			})
		})
	}
}

// creatorQuery is the creator-branch statement of sqlchannels' ListWaiting
// for two types, in PostgreSQL placeholders. It is a copy: keep it in step
// with that query.
const creatorQuery = `SELECT id FROM channels
	WHERE tomb = '' AND state = $1 AND ((type = $2 AND session_state IN ($3, $4)) OR (type = $5 AND session_state IN ($6)))
	AND created_by_user <> '' AND created_by_user = $7 AND type IN ($8, $9) LIMIT $10`

// TestListWaiting_CreatorUsesIndex pins that the generic plan of the
// creator branch can use the partial channels_creator_idx. pgx caches its
// statements, and a cached statement may run as a generic plan, which
// proves the index predicate only from a literal. Sequential scans are
// off, so the plan picks an index whenever it can.
func TestListWaiting_CreatorUsesIndex(t *testing.T) {
	pool, _ := newScopedPool(t)
	ctx := context.Background()
	conn, err := pool.Acquire(ctx)
	require.NoError(t, err)
	defer conn.Release()
	for _, stmt := range []string{
		`SET enable_seqscan = off`,
		`SET plan_cache_mode = force_generic_plan`,
		`PREPARE creator(text, text, text, text, text, text, text, text, text, int) AS ` + creatorQuery,
	} {
		_, err = conn.Exec(ctx, stmt)
		require.NoError(t, err, stmt)
	}
	rows, err := conn.Query(ctx,
		`EXPLAIN EXECUTE creator('open', 'case', '', 'asked', 'topic', 'asked', 'alice', 'case', 'topic', 20001)`)
	require.NoError(t, err)
	var plan strings.Builder
	for rows.Next() {
		var line string
		require.NoError(t, rows.Scan(&line))
		plan.WriteString(line + "\n")
	}
	require.NoError(t, rows.Err())
	t.Logf("plan:\n%s", plan.String())
	require.Contains(t, plan.String(), "channels_creator_idx")
}
