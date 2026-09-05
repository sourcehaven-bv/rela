//go:build sqlite

package appbuild

import (
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/store/sqlitestore"
)

// TestSQLiteStoreSatisfiesConfigProvider pins the type assertion in
// layerStoreConfig.
//
// Worth a test of its own because failing it is SILENT: a store that does not
// satisfy storeConfigProvider simply falls through to the disk-only loader,
// with no error logged and nothing to notice — the store-backed config layer
// would never be installed and every test of it would still pass, since they
// exercise the loader directly. Two things break the assertion invisibly:
// putting ProjectFiles on Conn but not Store (it was, at first), and the two
// interfaces drifting apart.
func TestSQLiteStoreSatisfiesConfigProvider(t *testing.T) {
	var s any = (*sqlitestore.Store)(nil)
	if _, ok := s.(storeConfigProvider); !ok {
		t.Error("sqlitestore.Store does not satisfy storeConfigProvider; " +
			"the store-backed config layer would silently never be installed")
	}
}
