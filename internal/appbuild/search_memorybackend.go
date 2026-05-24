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
func newSearchObserver() *search.LinearSearch {
	return search.NewLinearSearch()
}

// asObserver widens the per-build search backend to
// [store.EntityObserver] without the typed-nil-into-interface trap.
// Mirrors the FS-build helper of the same name.
func asObserver(b *search.LinearSearch) store.EntityObserver {
	if b == nil {
		return nil
	}
	return b
}

// buildSearcher — `memorybackend` build. Reuses the
// [search.LinearSearch] passed in as backend. LinearSearch
// self-populates via its EntityObserver hook, so no separate backfill
// is required for stores that wire the observer at open time
// (memstore does).
func buildSearcher(
	_ context.Context,
	st store.Store,
	backend *search.LinearSearch,
) (search.Searcher, io.Closer, error) {
	if backend == nil {
		return search.ErrSearcher(errors.New("search index not available")), noopCloser{}, nil
	}
	return search.New(st, backend), backend, nil
}

// noopCloser is returned when no closable resource is held.
type noopCloser struct{}

func (noopCloser) Close() error { return nil }
