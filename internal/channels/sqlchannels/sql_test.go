package sqlchannels_test

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/channels"
	"github.com/Sourcehaven-BV/rela/internal/channels/channelstest"
	"github.com/Sourcehaven-BV/rela/internal/channels/sqlchannels"
	"github.com/Sourcehaven-BV/rela/internal/channels/sqlitechannels"
	"github.com/Sourcehaven-BV/rela/internal/sqlitedb"
)

func newStore(t *testing.T) (*sqlchannels.Store, *sqlitedb.DB) {
	t.Helper()
	db, err := sqlitedb.Open(context.Background(), sqlitedb.Options{Path: filepath.Join(t.TempDir(), "rela.db")})
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	s, err := sqlitechannels.New(db.DB())
	require.NoError(t, err)
	return s, db
}

// TestConformance runs the shared contract over SQLite, the database every
// build can test without a server. pgchannels runs it over PostgreSQL.
func TestConformance(t *testing.T) {
	channelstest.RunAll(t, func(t *testing.T) channelstest.Harness {
		t.Helper()
		s, db := newStore(t)
		return channelstest.Harness{
			Store: s,
			// A v19 row upgraded by the v20 rung: the column default fills
			// cardinality.
			SeedLegacyType: func(t *testing.T, typ string, anchor channels.AnchorMode) {
				t.Helper()
				_, err := db.DB().ExecContext(context.Background(),
					`INSERT INTO channel_types (type, anchor_mode) VALUES (?, ?)`, typ, string(anchor))
				require.NoError(t, err)
			},
			Raw: func(t *testing.T) []byte {
				t.Helper()
				raw, err := sqlchannels.Raw(context.Background(), s)
				require.NoError(t, err)
				return raw
			},
		}
	})
}

func TestNewRejectsNil(t *testing.T) {
	_, err := sqlchannels.New(nil, sqlchannels.Dialect{Name: "x"})
	require.Error(t, err)
}

// A damaged row surfaces as an error naming the backend, not as a panic or
// a silently empty message.
func TestDamagedRows(t *testing.T) {
	ctx := context.Background()
	s, db := newStore(t)
	k := channels.Key{Anchor: channels.Anchor{ID: "TKT-1"}, Type: "room"}
	_, err := s.Append(ctx, k, channels.NewMessage{Type: "text", Author: channelstest.Alice, At: channelstest.T0})
	require.NoError(t, err)

	_, err = db.DB().ExecContext(ctx, `UPDATE channel_messages SET doc = 'nope'`)
	require.NoError(t, err)
	_, err = s.Get(ctx, k, 1)
	require.ErrorContains(t, err, "sqlitechannels")
	_, err = s.Messages(ctx, k, channels.Page{})
	require.ErrorContains(t, err, "damaged")

	_, err = db.DB().ExecContext(ctx, `UPDATE channel_messages SET pending_at = 5`)
	require.NoError(t, err)
	_, err = s.StalePending(ctx, channelstest.T0)
	require.ErrorContains(t, err, "damaged")

	_, err = db.DB().ExecContext(ctx, `INSERT INTO channel_audit (anchor_id, record) VALUES ('TKT-1', 'nope')`)
	require.NoError(t, err)
	_, err = s.Audit(ctx, "TKT-1")
	require.ErrorContains(t, err, "damaged")

	_, err = db.DB().ExecContext(ctx, `UPDATE channels SET created_by = 'nope'`)
	require.NoError(t, err)
	_, err = s.Head(ctx, k)
	require.ErrorContains(t, err, "damaged")
	_, err = s.Keys(ctx)
	require.Error(t, err)
	_, err = s.ListByLifecycle(ctx, "bogus", channelstest.T0)
	require.ErrorIs(t, err, channels.ErrInvalidMessage)
}

// A closed database fails every call with a wrapped error.
func TestClosedDatabase(t *testing.T) {
	ctx := context.Background()
	s, db := newStore(t)
	require.NoError(t, db.Close())
	k := channels.Key{Anchor: channels.Anchor{ID: "TKT-1"}, Type: "room"}
	_, err := s.Append(ctx, k, channels.NewMessage{Type: "text", Author: channelstest.Alice, At: channelstest.T0})
	require.ErrorContains(t, err, "sqlitechannels")
	_, err = s.Messages(ctx, k, channels.Page{})
	require.Error(t, err)
	_, err = s.ListEntity(ctx, "TKT-1")
	require.Error(t, err)
	_, err = sqlchannels.Raw(ctx, s)
	require.Error(t, err)
}
