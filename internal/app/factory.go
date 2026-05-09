// Package app provides factories that construct the concrete services
// needed by each rela entry point (cli, data-entry server, desktop,
// MCP). Today that is a single factory: FSFactory, which opens an
// fsstore rooted at a project directory.
package app

import (
	"errors"
	"fmt"

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
		CacheKey:       ".rela",
		Schemas:        buildSchemas(meta),
	})
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
