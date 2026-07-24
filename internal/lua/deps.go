package lua

import (
	"context"
	"iter"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/search"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/tracer"
)

// EntityReader is the read-OUT surface every graph-reading Lua binding
// goes through. Defined here at the consumer per CLAUDE.md "interfaces at
// the call site" (same rationale as [Mutator] and cacheStore) — three
// methods, which is exactly what the bindings call.
//
// The wiring site decides what backs it, and that decision IS the read-ACL
// (DEC-O59WM4): a visibility-backed adapter binds reads to the acting
// identity, while a plain store.Store satisfies it structurally for the
// operator-trust-boundary paths (CLI, docs runtime). Bindings therefore
// contain no ACL logic at all — they cannot forget to gate, because they
// have nothing else to read through.
type EntityReader interface {
	GetEntity(ctx context.Context, id string) (*entity.Entity, error)
	ListEntities(ctx context.Context, q store.EntityQuery) iter.Seq2[*entity.Entity, error]
	ListRelations(ctx context.Context, q store.RelationQuery) iter.Seq2[*entity.Relation, error]
}

// ReadDeps is the capability bundle required to run a read-only Lua runtime.
// A runtime built from ReadDeps (see NewReader) exposes only query, trace,
// search, schema introspection, and output-to-stdout bindings. It cannot
// mutate the graph and cannot write files — both rela.create_entity et al.
// and rela.write_file are absent from the rela.* table on a reader.
//
// ProjectRoot is the absolute project path; used by writer runtimes to
// resolve the output directory for rela.write_file.
type ReadDeps struct {
	// VisibleReader serves every READ-OUT: rela.get_entity,
	// rela.list_entities, rela.get_relations, rela.search hydration, and
	// markdown entity-refs. Whatever the wiring site puts here defines what
	// scripts can see.
	//
	// A nil VisibleReader DENIES (the binding raises); it never falls back
	// to WritePrepStore. A forgotten wiring must not become an ACL bypass
	// (RR-X9NVHI).
	VisibleReader EntityReader

	// WritePrepStore is the RAW, ungated store handle, used by exactly one
	// binding: rela.update_entity's read-before-write.
	//
	// DO NOT route read-outs through this, and DO NOT "tidy" it away by
	// pointing update_entity at VisibleReader. update_entity does
	// GetEntity → Clone → merge → save, so reading a REDACTED entity there
	// would drop the caller's hidden properties from the clone and ERASE
	// THEM ON SAVE — silent data destruction. This is the read-out /
	// write-prep boundary DEC-ZBI39P calls out; the two fields are separate
	// so the wrong choice is visible at the call site. Nil on reader
	// runtimes (NewReader), which have no write bindings.
	WritePrepStore store.Store

	// Tracer is the graph-traversal surface. Wiring injects either the
	// plain tracer or a visibility decorator over it; the trace bindings
	// are identical either way (they carry no properties, so gating is
	// purely node/title pruning inside the decorator).
	Tracer tracer.Tracer

	// Searcher produces search hits. NOTE: hits themselves are NOT gated —
	// rela.search hydrates each hit through VisibleReader and drops the
	// ones it cannot read, so no hidden entity or property reaches the
	// script. A timing/count residual remains (an attacker could infer that
	// *something* matched); that is the TKT-GGQ0JT class of oracle and is
	// tracked there, not silently ignored here.
	Searcher search.Searcher

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
