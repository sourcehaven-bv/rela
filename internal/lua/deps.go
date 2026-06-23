package lua

import (
	"context"

	"github.com/Sourcehaven-BV/rela/internal/entity"
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

// Mutator is the consumer-side write surface Lua bindings call into
// from rela.create_entity / rela.update_entity / rela.delete_entity /
// rela.create_relation / rela.delete_relation. Defined here at the
// consumer per CLAUDE.md "interfaces at the call site"; the wiring
// site supplies an implementation (the production one being the
// project's EntityManager).
//
// Five methods — RenameEntity and UpdateRelation are intentionally
// absent because no Lua binding invokes them. Narrowed from the
// wider EntityManager interface in TKT-IF37 to drop lua's transitive
// dependency on internal/entitymanager.
type Mutator interface {
	CreateEntity(ctx context.Context, e *entity.Entity, opts entity.CreateOptions) (*entity.CreateResult, error)
	UpdateEntity(ctx context.Context, e *entity.Entity) (*entity.UpdateResult, error)
	DeleteEntity(ctx context.Context, id string, cascade bool) (*entity.DeleteResult, error)
	CreateRelation(ctx context.Context, from, relType, to string, opts entity.RelationOptions) (*entity.Relation, error)
	DeleteRelation(ctx context.Context, from, relType, to string) error
}

// WriteDeps is the capability bundle required to run a read-write Lua runtime.
// A runtime built from WriteDeps (see NewWriter) additionally exposes
// create/update/delete bindings for entities and relations.
type WriteDeps struct {
	ReadDeps
	EntityManager Mutator

	// ElevatedManager, when non-nil, is a write handle whose mutations skip
	// the ACL deny (TKT-D8T148). It is set ONLY for an allow_acl_bypass
	// automation action; its presence is what makes the runtime register
	// rela.bypass_acl(fn). Nil on every other runtime, so rela.bypass_acl is
	// absent and a script cannot elevate.
	ElevatedManager Mutator
}
