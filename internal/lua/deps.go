package lua

import (
	"github.com/Sourcehaven-BV/rela/internal/entitymanager"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/search"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/tracer"
)

// ReadDeps is the capability bundle required to run a read-only Lua runtime.
// A runtime built from ReadDeps (see NewReader) exposes only the query/trace/
// search/output bindings; mutation bindings are not registered.
//
// ProjectRoot is the absolute path used to resolve relative paths in
// rela.write_file.
type ReadDeps struct {
	Store       store.Store
	Tracer      tracer.Tracer
	Searcher    search.Searcher
	Meta        *metamodel.Metamodel
	ProjectRoot string
}

// WriteDeps is the capability bundle required to run a read-write Lua runtime.
// A runtime built from WriteDeps (see NewWriter) additionally exposes
// create/update/delete bindings for entities and relations.
type WriteDeps struct {
	ReadDeps
	EntityManager entitymanager.EntityManager
}
