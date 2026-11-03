//go:build postgres

package dataentry

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/channels"
	"github.com/Sourcehaven-BV/rela/internal/channels/pgchannels"
)

// benchChannelStore is the PostgreSQL channel store in a fresh schema,
// with its inbox grid.
func benchChannelStore(tb testing.TB) (cs channels.Store, grid int) {
	tb.Helper()
	pool, _ := conflictTestSchema(tb)
	store, err := pgchannels.New(pool)
	require.NoError(tb, err)
	return store, LiveInboxGrid
}
