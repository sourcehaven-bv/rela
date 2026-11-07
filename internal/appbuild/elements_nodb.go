//go:build !postgres && !sqlite

package appbuild

import (
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/visibility"
)

// elementRangesFor returns the reference element range reader over st, the
// reader of fsstore and memstore: they store no element columns, so it
// computes each element from the row.
func elementRangesFor(st store.Store) (visibility.ElementRangeReader, error) {
	return naiveRanges(st)
}
