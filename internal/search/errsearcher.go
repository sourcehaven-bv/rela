package search

import (
	"context"
	"iter"
)

// ErrSearcher returns a [Searcher] whose Search method yields a
// single (Hit{}, err) pair and stops. Used by wiring sites to
// surface "search backend unavailable" to callers without panicking
// or returning silently empty results.
//
// The yielded error is terminal: callers that respect iter.Seq2
// semantics will see exactly one yield, observe the error, and stop
// iterating.
func ErrSearcher(err error) Searcher {
	return errSearcher{err: err}
}

type errSearcher struct{ err error }

var _ Searcher = errSearcher{}

func (s errSearcher) Search(_ context.Context, _ Query) iter.Seq2[Hit, error] {
	return func(yield func(Hit, error) bool) {
		yield(Hit{}, s.err)
	}
}
