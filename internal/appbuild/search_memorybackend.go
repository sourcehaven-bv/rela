//go:build memorybackend

package appbuild

import (
	"context"
	"errors"
	"io"

	"github.com/Sourcehaven-BV/rela/internal/search"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// newSearchObserver — `memorybackend` build. Uses [search.LinearSearch]:
// in-memory substring matching with no external dependencies. Slow on
// large stores but zero-bytes-of-bleve in the resulting binary, which
// is the point of this build.
func newSearchObserver() store.EntityObserver {
	return search.NewLinearSearch()
}

// buildSearcher — `memorybackend` build. Reuses the
// [search.LinearSearch] passed in as obs. LinearSearch self-populates
// via its EntityObserver hook, so no separate backfill is required
// for stores that wire the observer at open time (memstore does).
func buildSearcher(
	_ context.Context,
	st store.Store,
	obs store.EntityObserver,
) (search.Searcher, io.Closer, error) {
	backend, ok := obs.(*search.LinearSearch)
	if !ok || backend == nil {
		return search.ErrSearcher(errors.New("search index not available")), noopCloser{}, nil
	}
	return search.New(st, backend), backend, nil
}

// noopCloser is returned when no closable resource is held.
type noopCloser struct{}

func (noopCloser) Close() error { return nil }
