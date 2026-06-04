//go:build !postgres && !memorybackend

package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"

	"github.com/Sourcehaven-BV/rela/internal/app"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/project"
	"github.com/Sourcehaven-BV/rela/internal/search"
	"github.com/Sourcehaven-BV/rela/internal/search/bleveindex"
	"github.com/Sourcehaven-BV/rela/internal/storage"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// openMCPBackend — default (filesystem) build. Opens an fsstore with an
// in-memory bleve index wired as a write observer, then backfills the
// index with entities already on disk. Mirrors the appbuild FS recipe;
// see that package for the rationale. A nil index is non-fatal — the
// store still opens and the searcher is an error-Searcher.
func openMCPBackend(
	ctx context.Context,
	fs storage.FS,
	paths *project.Context,
	meta *metamodel.Metamodel,
) (store.Store, search.Searcher, io.Closer, error) {
	idx, idxErr := bleveindex.NewMem()
	if idxErr != nil {
		slog.Warn("search backend unavailable; MCP search tool will return errors", "error", idxErr)
		idx = nil
	}

	factory := &app.FSFactory{FS: fs, Paths: paths}
	if idx != nil {
		factory.AddObserver(idx)
	}
	st, err := factory.OpenStore(meta)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("open store: %w", err)
	}

	if idx == nil {
		return st, search.ErrSearcher(errors.New("search index not available")), mcpNoopCloser{}, nil
	}
	if err := backfillMCPBackend(ctx, idx, st); err != nil {
		slog.Warn("search index backfill incomplete", "error", err)
	}
	return st, search.New(st, idx), idx, nil
}

// backfillMCPBackend populates the bleve index from the store at startup.
// List and per-entity index errors are collected and returned together so
// callers log a summary rather than silently swallowing failures. Nil
// backend or store is a no-op (exercised by mcp_wiring_search_fs_test.go).
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

// mcpNoopCloser is returned when the FS build has no closable search
// resource (the error-Searcher case). The memory build never needs it;
// the postgres build returns a pool closer.
type mcpNoopCloser struct{}

func (mcpNoopCloser) Close() error { return nil }
