//go:build !postgres && !memorybackend && !sqlite

package appbuild

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"

	"github.com/Sourcehaven-BV/rela/internal/app"
	"github.com/Sourcehaven-BV/rela/internal/search"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// New builds the services bundle for the default (filesystem) build: an
// fsstore rooted at the project paths plus an in-memory bleve search
// index wired as a write observer. This is the per-scenario recipe — it
// owns only the backend choice; [prepare] and [assemble] do the
// build-agnostic work shared by every build.
func New(cfg Config, opts ...Option) (*Services, error) {
	base, err := prepare(cfg, opts)
	if err != nil {
		return nil, err
	}
	st, searcher, closer, err := openBackend(context.Background(), base)
	if err != nil {
		return nil, err
	}
	// nil VisibleSearcher → assemble derives the generic
	// search.NewVisible wrapper (TKT-BA8BSX); only the postgres
	// recipe wires a native implementation.
	return assemble(base, st, searcher, nil, closer)
}

// openBackend opens the fsstore and the bleve-backed searcher. The bleve
// index is created first and installed as a store observer at open time
// so it receives initial write events; it is then backfilled with
// entities already on disk (the observer is not invoked for those).
//
// A nil index is non-fatal: the store still opens and buildSearcher
// returns an error-Searcher, so read/write paths keep working.
func openBackend(ctx context.Context, base *SharedBase) (store.Store, search.Searcher, io.Closer, error) {
	idx := openSearchIndex(base)

	factory := &app.FSFactory{FS: base.cfg.FS, Paths: base.cfg.Paths}
	if idx != nil {
		factory.AddObserver(idx)
	}
	st, err := factory.OpenStore(base.meta)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("open store: %w", err)
	}

	if idx == nil {
		return st, search.ErrSearcher(errors.New("search index not available")), noopCloser{}, nil
	}
	if err := backfillBleve(ctx, idx, st); err != nil {
		slog.Warn("appbuild: failed to index entities", "error", err)
	}
	return st, search.New(st, idx), idx, nil
}

// noopCloser is returned when no closable search resource is held.
type noopCloser struct{}

func (noopCloser) Close() error { return nil }
