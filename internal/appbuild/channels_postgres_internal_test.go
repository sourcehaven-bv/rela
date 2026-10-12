//go:build postgres

package appbuild

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/store/memstore"
)

// A store without a pool falls back to the file backend rather than failing
// the assembly.
func TestStoreChannelsFor_NotAPgstore(t *testing.T) {
	require.Nil(t, storeChannelsFor(memstore.New()))
}
