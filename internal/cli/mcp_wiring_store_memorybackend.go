//go:build memorybackend

package cli

import (
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/project"
	"github.com/Sourcehaven-BV/rela/internal/storage"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/store/memstore"
)

// openMCPStore — `memorybackend` build. Backs the MCP project store
// with an in-memory [memstore]. Mirrors the appbuild seam; see that
// package for the rationale.
func openMCPStore(
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
