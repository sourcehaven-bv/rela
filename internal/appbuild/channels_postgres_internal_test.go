//go:build postgres

package appbuild

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/channels/filechannels"
	"github.com/Sourcehaven-BV/rela/internal/store/memstore"
)

// A store without a pool gets an in-memory channel store rather than the
// node-local cache directory, and rather than failing the assembly.
func TestStoreChannelsFor_NotAPgstore(t *testing.T) {
	cs := storeChannelsFor(memstore.New())
	require.NotNil(t, cs)
	require.IsType(t, &filechannels.Store{}, cs)
}
