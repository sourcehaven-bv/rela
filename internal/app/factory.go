// Package app provides factories that construct the concrete services
// needed by each rela entry point (cli, data-entry server, desktop,
// MCP). Today that is a single factory: FSFactory, which opens an
// fsstore rooted at a project directory.
package app

import (
	"errors"
	"fmt"
	"io/fs"

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
//
// Observers (set via [FSFactory.AddObserver]) are forwarded to
// [fsstore.Config.Observers] when the store is opened: each observer
// is notified synchronously on entity writes (create / update /
// delete / rename). This is the hook used by derived state (e.g.
// search indexes) to stay current; it is NOT invoked for entities
// already on disk when the store is first opened, so callers that
// need an initial snapshot must iterate the store after OpenStore
// returns and feed their observer manually.
type FSFactory struct {
	FS    storage.FS
	Paths *project.Context

	observers []store.EntityObserver
}

// AddObserver registers an entity observer that the next OpenStore
// call will hook into the resulting store. Safe to call repeatedly;
// each observer is invoked exactly once per write event.
func (f *FSFactory) AddObserver(o store.EntityObserver) {
	if o == nil {
		return
	}
	f.observers = append(f.observers, o)
}

// compile-time interface check
var _ store.Factory = (*FSFactory)(nil)

// OpenStore constructs a new fsstore rooted at the project directory.
// Files on disk are plain bytes; confidentiality at the sync boundary
// is the responsibility of git-crypt (or an equivalent tool) rather
// than this process.
//
// meta must be non-nil and declare at least one entity type — fsstore
// rejects an empty Schemas map.
func (f *FSFactory) OpenStore(meta *metamodel.Metamodel) (store.Store, error) {
	if meta == nil {
		return nil, errors.New("app: FSFactory.OpenStore requires a non-nil metamodel")
	}
	rooted, err := storage.NewRootedFS(f.FS, f.Paths.Root)
	if err != nil {
		return nil, fmt.Errorf("app: rooted fs for fsstore: %w", err)
	}
	return fsstore.New(fsstore.Config{
		FS:             f.FS,
		Rooted:         rooted,
		EntitiesKey:    "entities",
		RelationsKey:   "relations",
		AttachmentsKey: "attachments",
		CacheKey:       storeCacheKey,
		Schemas:        buildSchemas(meta),
		Observers:      f.observers,
	})
}

// storeCacheKey is the directory, relative to the project root, that holds
// the store's persisted index.
const storeCacheKey = ".rela"

// DropStoreIndex removes the store's persisted index, so the next store
// opened over the project scans the files. A store persists its index when
// it closes, stamped with the folder times at that moment, so a store that
// closes after another store wrote files it never saw leaves an index that
// looks fresh and omits them.
func (f *FSFactory) DropStoreIndex() error {
	rooted, err := storage.NewRootedFS(f.FS, f.Paths.Root)
	if err != nil {
		return err
	}
	if err := rooted.Remove(fsstore.IndexKey(storeCacheKey)); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	return nil
}

// buildSchemas translates metamodel entity-type definitions into the
// store-facing EntityTypeSchema map used by fsstore. Plural is always
// resolved here (via GetPlural) so fsstore can rely on it being
// non-empty and skip the trim-trailing-"s" guesswork at call time.
func buildSchemas(meta *metamodel.Metamodel) map[string]store.EntityTypeSchema {
	if meta == nil {
		return nil
	}
	out := make(map[string]store.EntityTypeSchema, len(meta.Entities))
	for name, et := range meta.Entities {
		out[name] = store.EntityTypeSchema{
			Plural:        et.GetPlural(name),
			PropertyOrder: et.PropertyOrder,
		}
	}
	return out
}
