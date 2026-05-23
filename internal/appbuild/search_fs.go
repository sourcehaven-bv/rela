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

// newSearchObserver builds the search index that should be wired as a
// store observer at OpenStore time. The FS build returns an in-memory
// bleve index. Returns nil when index creation fails — non-fatal:
// [buildSearcher] downstream returns an error-Searcher so callers
// receive an explicit "search not available" message instead of
// crashing.
//
// The concrete return type lets the caller plumb the same value into
// [openStore] (which takes [store.EntityObserver]) and [buildSearcher]
// (which needs the bleve handle for backfill) without a runtime type
// assertion. A future Postgres build returns a different concrete from
// its own newSearchObserver; the wiring shape stays per-build.
func newSearchObserver() *bleveindex.Index {
	idx, err := bleveindex.NewMem()
	if err != nil {
		slog.Warn("appbuild: failed to create search index", "error", err)
		return nil
	}
	return idx
}

// asObserver widens the per-build search backend to
// [store.EntityObserver] without the typed-nil-into-interface trap.
// A nil concrete becomes a nil interface (rather than a non-nil
// interface holding a nil pointer), so downstream `if obs != nil`
// checks at the store layer behave as intended.
func asObserver(b *bleveindex.Index) store.EntityObserver {
	if b == nil {
		return nil
	}
	return b
}

// buildSearcher returns the Searcher backed by the store and the
// bleve index previously installed at OpenStore time. A nil index
// yields an error-Searcher so the rest of the services bundle still
// works (read/write paths don't depend on search).
//
// Backfill is run synchronously because the observer is not invoked
// for entities already on disk at open time.
func buildSearcher(
	ctx context.Context,
	st store.Store,
	backend *bleveindex.Index,
) (search.Searcher, io.Closer, error) {
	if backend == nil {
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
