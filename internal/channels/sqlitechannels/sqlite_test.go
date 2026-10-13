package sqlitechannels_test

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/channels"
	"github.com/Sourcehaven-BV/rela/internal/channels/channelstest"
	"github.com/Sourcehaven-BV/rela/internal/channels/sqlitechannels"
	"github.com/Sourcehaven-BV/rela/internal/sqlitedb"
)

// The conformance suite runs in sqlchannels, over this adapter. These tests
// cover what is the adapter's own: the transaction and its rollback.

func TestNewRejectsNil(t *testing.T) {
	_, err := sqlitechannels.New(nil)
	require.Error(t, err)
}

// A failed write rolls back every statement before it: the implicit channel
// an invalid reply would have created does not survive.
func TestFailedWriteRollsBack(t *testing.T) {
	ctx := context.Background()
	db, err := sqlitedb.Open(ctx, sqlitedb.Options{Path: filepath.Join(t.TempDir(), "rela.db")})
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	s, err := sqlitechannels.New(db.DB())
	require.NoError(t, err)

	k := channels.Key{Anchor: channels.Anchor{ID: "TKT-1"}, Type: "room"}
	_, err = s.Append(ctx, k, channels.NewMessage{
		Type: "text", Author: channelstest.Alice, At: channelstest.T0, ReplyTo: 7,
	})
	require.ErrorIs(t, err, channels.ErrInvalidReply)
	_, err = s.Head(ctx, k)
	require.ErrorIs(t, err, channels.ErrNotFound)
	audit, err := s.Audit(ctx, "TKT-1")
	require.NoError(t, err)
	require.Empty(t, audit)

	m, err := s.Append(ctx, k, channels.NewMessage{Type: "text", Author: channelstest.Alice, At: channelstest.T0})
	require.NoError(t, err)
	require.Equal(t, int64(1), m.Seq)
	page, err := s.Messages(ctx, k, channels.Page{})
	require.NoError(t, err)
	require.Len(t, page, 1)

	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	_, err = s.Append(cancelled, k, channels.NewMessage{Type: "text", Author: channelstest.Alice, At: channelstest.T0})
	require.Error(t, err)
}
