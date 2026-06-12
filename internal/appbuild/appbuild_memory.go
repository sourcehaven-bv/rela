//go:build memorybackend

package appbuild

import (
	"context"
	"errors"
	"io"

	"github.com/Sourcehaven-BV/rela/internal/search"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/store/memstore"
)

// New builds the services bundle for the `memorybackend` build: an
// in-memory memstore plus a zero-dependency LinearSearch index. Data is
// lost on exit; this build exists to prove the backend seam swaps
// cleanly and for quick experiments with no bleve and no database linked.
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

// openBackend opens a memstore with a LinearSearch index wired as a
// write observer. LinearSearch self-populates via the observer hook, so
// no backfill is needed (the store starts empty anyway). The fs/paths in
// base are unused by the store here; the metamodel still came from disk
// via prepare.
func openBackend(_ context.Context, _ *buildBase) (store.Store, search.Searcher, io.Closer, error) {
	idx := search.NewLinearSearch()
	st := memstore.New(memstore.WithObserver(idx))
	if idx == nil {
		return st, search.ErrSearcher(errors.New("search index not available")), noopCloser{}, nil
	}
	return st, search.New(st, idx), idx, nil
}

// noopCloser is returned when no closable search resource is held.
type noopCloser struct{}

func (noopCloser) Close() error { return nil }
