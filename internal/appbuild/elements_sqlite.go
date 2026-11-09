//go:build sqlite

package appbuild

import (
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/store/sqlitestore"
	"github.com/Sourcehaven-BV/rela/internal/visibility"
)

// elementRangesFor returns sqlitestore's element range reader, which answers
// a range in one statement over the stored element columns. A store that is
// not a sqlitestore gets the reference reader over st.
func elementRangesFor(st store.Store) (visibility.ElementRangeReader, error) {
	if ss, ok := store.Innermost(st).(*sqlitestore.Store); ok {
		r, err := sqlitestore.NewElementRanges(ss)
		if err != nil { // coverage-ignore: invariant: the store is non-nil here
			return nil, err
		}
		return r, nil
	}
	return naiveRanges(st)
}
