package store_test

import (
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/store/memstore"
	"github.com/Sourcehaven-BV/rela/internal/store/storetest"
)

// TestRollupGeneric runs the roll-up conformance over memstore, which has no
// native roll-up, so [store.Rollup] takes the Go fold. The backends run the
// same suite; this run attributes the fold's coverage to this package.
func TestRollupGeneric(t *testing.T) {
	t.Parallel()
	storetest.RunRollupTests(t, func(*testing.T) store.Store { return memstore.New() })
}
