package app

import (
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/project"
	"github.com/Sourcehaven-BV/rela/internal/storage"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/store/fsstore"
)

// FSFactory is a store.Factory that opens filesystem-backed stores
// (fsstore) rooted at the given project paths. Each OpenStore call
// returns a fresh, independent store — callers that want a single
// long-lived store should open it once and keep it alive.
type FSFactory struct {
	FS    storage.FS
	Paths *project.Context
}

// compile-time interface check
var _ store.Factory = (*FSFactory)(nil)

// OpenStore constructs a new fsstore configured for meta's entity-type
// schemas. The returned store owns its own in-memory index and
// recentHashes cache; close it when done.
func (f *FSFactory) OpenStore(meta *metamodel.Metamodel) (store.Store, error) {
	return fsstore.New(fsstore.Config{
		FS:           f.FS,
		EntitiesDir:  f.Paths.EntitiesDir,
		RelationsDir: f.Paths.RelationsDir,
		CacheDir:     f.Paths.CacheDir,
		Schemas:      buildSchemas(meta),
	})
}
