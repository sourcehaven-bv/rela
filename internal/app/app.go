// Package app provides factories that construct the concrete services
// (store, filesystem, paths) needed by each rela entry point.
//
// The factory is intentionally minimal: it wires an fsstore on top of
// an existing project context and metamodel so entry points can start
// consuming store events (via fsstore.Subscribe) without each main
// function duplicating the construction logic.
//
// This package does not yet replace workspace — both live side-by-side
// during the migration. Over time, more services (search, attachment,
// templater, config, state) will move here.
package app

import (
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/project"
	"github.com/Sourcehaven-BV/rela/internal/storage"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/store/fsstore"
)

// App bundles the concrete services an entry point needs.
type App struct {
	FS    storage.FS
	Paths *project.Context
	Store *fsstore.FSStore
}

// Config configures App construction.
type Config struct {
	FS    storage.FS
	Paths *project.Context
	Meta  *metamodel.Metamodel // used to derive entity-type schemas
}

// New constructs an App from cfg. The fsstore is created with schemas
// derived from the metamodel so directory names (plural) resolve
// correctly. Callers should defer Close to release resources.
func New(cfg Config) (*App, error) {
	schemas := buildSchemas(cfg.Meta)

	s, err := fsstore.New(fsstore.Config{
		FS:           cfg.FS,
		EntitiesDir:  cfg.Paths.EntitiesDir,
		RelationsDir: cfg.Paths.RelationsDir,
		CacheDir:     cfg.Paths.CacheDir,
		Schemas:      schemas,
	})
	if err != nil {
		return nil, err
	}

	return &App{
		FS:    cfg.FS,
		Paths: cfg.Paths,
		Store: s,
	}, nil
}

// Close releases resources held by the app (store subscribers, watcher).
func (a *App) Close() error {
	if a.Store != nil {
		return a.Store.Close()
	}
	return nil
}

// buildSchemas translates metamodel entity-type definitions into the
// store-facing EntityTypeSchema map used by fsstore.
func buildSchemas(meta *metamodel.Metamodel) map[string]store.EntityTypeSchema {
	if meta == nil {
		return nil
	}
	out := make(map[string]store.EntityTypeSchema, len(meta.Entities))
	for name, et := range meta.Entities {
		out[name] = store.EntityTypeSchema{
			Plural:        et.Plural,
			PropertyOrder: et.PropertyOrder,
		}
	}
	return out
}
