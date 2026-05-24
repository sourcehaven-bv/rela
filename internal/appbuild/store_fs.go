//go:build !postgres && !memorybackend

package appbuild

import (
	"github.com/Sourcehaven-BV/rela/internal/app"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/project"
	"github.com/Sourcehaven-BV/rela/internal/storage"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// openStore opens the project store for this build. The FS build wires
// an [app.FSFactory] rooted at the project paths; the optional observer
// is registered on the factory before OpenStore so it receives the
// initial write events. obs may be nil — that is the no-search case.
//
// A companion file behind //go:build postgres supplies a pgstore-backed
// implementation; obs is ignored there (Postgres indexes inside the
// store itself).
func openStore(
	fs storage.FS,
	paths *project.Context,
	meta *metamodel.Metamodel,
	obs store.EntityObserver,
) (store.Store, error) {
	factory := &app.FSFactory{FS: fs, Paths: paths}
	if obs != nil {
		factory.AddObserver(obs)
	}
	return factory.OpenStore(meta)
}
