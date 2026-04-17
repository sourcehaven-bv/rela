package lua

import (
	"github.com/Sourcehaven-BV/rela/internal/entitymanager"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/tracer"
)

// Services bundles the backend services a Lua runtime needs.
type Services struct {
	Store    store.Store
	Manager  entitymanager.EntityManager
	Tracer   tracer.Tracer
	Searcher store.Searcher
	Meta     *metamodel.Metamodel

	// ProjectRoot is the absolute project path. Used internally by
	// rela.write_file to resolve relative output directories. Not exposed
	// to Lua scripts directly.
	ProjectRoot string

	// Sync is an optional callback invoked by rela.sync().
	// If nil, sync() is a no-op.
	Sync func() error
}
