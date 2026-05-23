//go:build !postgres && !memorybackend

package appbuild

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/search"
	"github.com/Sourcehaven-BV/rela/internal/search/bleveindex"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// newSearchObserver builds the search backend that should be wired as a
// store observer at OpenStore time. For the FS build this is an
// in-memory bleve index; for a future Postgres build the searcher
// indexes inside the store, so this returns (nil, nil) there.
//
// A nil observer is non-fatal: [buildSearcher] downstream returns an
// error-Searcher so callers receive an explicit "search not available"
// message instead of crashing.
func newSearchObserver() store.EntityObserver {
	idx, err := bleveindex.NewMem()
	if err != nil {
		slog.Warn("appbuild: failed to create search index", "error", err)
		return nil
	}
	return idx
}

// buildSearcher returns the Searcher backed by the store and the
// observer previously installed at OpenStore time. The FS path requires
// the observer to be the bleve index from [newSearchObserver]; if it is
// nil or a different concrete type the searcher returns errors.
//
// Returns the searcher, a closer that releases the index resources, and
// any backfill error. Backfill is run synchronously here because the
// observer is not invoked for entities already on disk at open time.
func buildSearcher(
	ctx context.Context,
	st store.Store,
	obs store.EntityObserver,
) (search.Searcher, io.Closer, error) {
	backend, ok := obs.(*bleveindex.Index)
	if !ok || backend == nil {
		return search.ErrSearcher(errors.New("search index not available")), noopCloser{}, nil
	}
	if err := backfillSearchBackend(ctx, backend, st); err != nil {
		slog.Warn("appbuild: failed to index entities", "error", err)
	}
	return search.New(st, backend), backend, nil
}

// backfillSearchBackend populates a search backend with every entity
// currently in the store. Errors from individual entities are
// collected and returned together so callers see the complete picture.
func backfillSearchBackend(ctx context.Context, backend *bleveindex.Index, s store.Store) error {
	if backend == nil || s == nil {
		return nil
	}
	entities := make([]*entity.Entity, 0)
	var listErrs []error
	for e, err := range s.ListEntities(ctx, store.EntityQuery{}) {
		if err != nil {
			listErrs = append(listErrs, err)
			continue
		}
		entities = append(entities, e)
	}
	indexed, indexErr := backend.IndexBatch(entities)
	if len(listErrs) == 0 && indexErr == nil {
		return nil
	}
	skipped := len(entities) - indexed
	return fmt.Errorf("backfill indexed %d entities, skipped %d, list errors: %v, index error: %w",
		indexed, skipped, listErrs, indexErr)
}

// noopCloser is returned when no closable resource is held.
type noopCloser struct{}

func (noopCloser) Close() error { return nil }
