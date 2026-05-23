//go:build !postgres && !memorybackend

package cli

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

// newMCPSearchObserver builds the search backend installed as a store
// observer at open time. FS build: in-memory bleve. Future Postgres
// build: nil (Postgres indexes inside the store itself).
func newMCPSearchObserver() store.EntityObserver {
	idx, err := bleveindex.NewMem()
	if err != nil {
		slog.Warn("search backend unavailable; MCP search tool will return errors", "error", err)
		return nil
	}
	return idx
}

// buildMCPSearcher returns the Searcher backed by the store and the
// observer previously installed at OpenStore time. Backfill is run
// synchronously because the observer is not invoked for entities
// already on disk at open time.
func buildMCPSearcher(
	ctx context.Context,
	st store.Store,
	obs store.EntityObserver,
) (search.Searcher, io.Closer, error) {
	backend, ok := obs.(*bleveindex.Index)
	if !ok || backend == nil {
		return search.ErrSearcher(errors.New("search index not available")), mcpNoopCloser{}, nil
	}
	if err := backfillMCPBackend(ctx, backend, st); err != nil {
		slog.Warn("search index backfill incomplete", "error", err)
	}
	return search.New(st, backend), backend, nil
}

// backfillMCPBackend populates the search backend from the store at
// startup. List errors and per-entity index errors are collected and
// returned together so callers can log a summary instead of silently
// swallowing failures. Partial-index outcomes are tolerable; a
// missing telemetry path is not.
func backfillMCPBackend(ctx context.Context, backend *bleveindex.Index, st store.Store) error {
	if backend == nil || st == nil {
		return nil
	}
	entities := make([]*entity.Entity, 0)
	var listErrs []error
	for e, err := range st.ListEntities(ctx, store.EntityQuery{}) {
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
