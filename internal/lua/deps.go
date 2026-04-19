package lua

import (
	"github.com/Sourcehaven-BV/rela/internal/entitymanager"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/search"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/tracer"
)

// ReadDeps is the capability bundle required to run a read-only Lua runtime.
// A runtime built from ReadDeps (see NewReader) exposes only query, trace,
// search, schema introspection, and output-to-stdout bindings. It cannot
// mutate the graph and cannot write files — both rela.create_entity et al.
// and rela.write_file are absent from the rela.* table on a reader.
//
// ProjectRoot is the absolute project path; used by writer runtimes to
// resolve the output directory for rela.write_file.
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
