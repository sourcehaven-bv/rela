//go:build memorybackend

package appbuild

import (
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/project"
	"github.com/Sourcehaven-BV/rela/internal/storage"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/store/memstore"
)

// openStore — `memorybackend` build. Backs the project store with an
// in-memory [memstore] instead of an on-disk fsstore. Data is lost on
// process exit; this build exists to prove the [openStore] seam swaps
// cleanly and to seed quick experiments. The fs and paths parameters
// are unused.
func openStore(
	_ storage.FS,
	_ *project.Context,
	_ *metamodel.Metamodel,
	obs store.EntityObserver,
) (store.Store, error) {
	opts := []memstore.Option{}
	if obs != nil {
		opts = append(opts, memstore.WithObserver(obs))
	}
	return memstore.New(opts...), nil
}
