//go:build postgres

package appbuild

import (
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/store/pgstore"
	"github.com/Sourcehaven-BV/rela/internal/visibility"
)

// elementRangesFor returns pgstore's element range reader, which answers a
// range in one statement over the stored element columns. A store that is
// not a pgstore gets the reference reader over st.
func elementRangesFor(st store.Store) (visibility.ElementRangeReader, error) {
	if ps, ok := store.Innermost(st).(*pgstore.Store); ok {
		r, err := pgstore.NewElementRanges(ps)
		if err != nil { // coverage-ignore: invariant: the store is non-nil here
			return nil, err
		}
		return r, nil
	}
	return naiveRanges(st)
}
