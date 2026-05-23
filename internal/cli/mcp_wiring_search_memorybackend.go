//go:build memorybackend

package cli

import (
	"context"
	"errors"
	"io"

	"github.com/Sourcehaven-BV/rela/internal/search"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// newMCPSearchObserver — `memorybackend` build. See the appbuild
// counterpart for the rationale; the linear search backend keeps
// bleve out of the binary.
func newMCPSearchObserver() store.EntityObserver {
	return search.NewLinearSearch()
}

// buildMCPSearcher — `memorybackend` build. Reuses the
// [search.LinearSearch] passed in as obs; it self-populates via the
// EntityObserver hook so no separate backfill is required when the
// store wires the observer at open time (memstore does).
func buildMCPSearcher(
	_ context.Context,
	st store.Store,
	obs store.EntityObserver,
) (search.Searcher, io.Closer, error) {
	backend, ok := obs.(*search.LinearSearch)
	if !ok || backend == nil {
		return search.ErrSearcher(errors.New("search index not available")), mcpNoopCloser{}, nil
	}
	return search.New(st, backend), backend, nil
}
