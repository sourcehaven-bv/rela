//go:build memorybackend

package cli

import (
	"context"
	"io"

	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/project"
	"github.com/Sourcehaven-BV/rela/internal/search"
	"github.com/Sourcehaven-BV/rela/internal/storage"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/store/memstore"
)

// openMCPBackend — `memorybackend` build. Backs the MCP store with an
// in-memory memstore and a zero-dependency LinearSearch index wired as a
// write observer (it self-populates, so no backfill is needed). Keeps
// bleve out of the binary. fs/paths are unused by the store; the
// metamodel still came from disk in newMCPServices.
func openMCPBackend(
	_ context.Context,
	_ storage.FS,
	_ *project.Context,
	_ *metamodel.Metamodel,
) (store.Store, search.Searcher, io.Closer, error) {
	idx := search.NewLinearSearch()
	st := memstore.New(memstore.WithObserver(idx))
	return st, search.New(st, idx), idx, nil
}
