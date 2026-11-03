//go:build !postgres

package dataentry

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/channels"
	"github.com/Sourcehaven-BV/rela/internal/channels/filechannels"
	"github.com/Sourcehaven-BV/rela/internal/filelock"
	"github.com/Sourcehaven-BV/rela/internal/storage"
)

// benchChannelStore is the file channel store in a temporary directory,
// with its inbox grid.
func benchChannelStore(tb testing.TB) (cs channels.Store, grid int) {
	tb.Helper()
	store, err := filechannels.New(storage.NewOsFS(), tb.TempDir(), filelock.ProcessPrivate{})
	require.NoError(tb, err)
	return store, LiveInboxGridScan
}
