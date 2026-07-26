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

	// ElevatedReader, when non-nil, is the RAW read handle backing the
	// admin.get_entity / list_entities / get_relations methods of the
	// bypass_acl closure (TKT-ACSBSA). Reads through it are unredacted and
	// ungated — the closure is the boundary, so a half-elevated read would
	// be a confusing contract.
	//
	// SEPARATE from WritePrepStore on purpose, even though production wires
	// both from the same raw store. WritePrepStore is present on EVERY
	// writer runtime (update_entity's read-before-write needs it); routing
	// elevated reads through it would hand every writer runtime an ungated
	// read path and dissolve the two-key gate that makes elevation
	// opt-in. This field is set at the same site, under the same two
	// conditions, as ElevatedManager.
	//
	// Nil is a DENY, not a fallback: admin.get_entity raises rather than
	// silently reading through the gated VisibleReader. Elevation that
	// quietly degrades to the caller's view is worse than a loud failure —
	// the script would see a partial graph and treat it as complete.
	ElevatedReader EntityReader

	// ElevationRecorder, when non-nil, is notified once per bypass_acl
	// closure that performed at least one elevated READ (TKT-ACSBSA).
	//
	// Elevated WRITES are already audited inside entitymanager
	// (audit.OpACLBypass). Reads never reach entitymanager — they go
	// straight to the store — so without this an operator querying the
	// audit log for elevation sees every elevated write and is silently
	// blind to every elevated read. That asymmetry is the gap this closes.
	//
	// ONCE PER CLOSURE, not per read: a single admin.list_entities can
	// traverse the whole graph, and a per-row audit record would put an
	// unbounded synchronous write on a read path. The forensic question an
	// operator actually asks is "which elevated scopes read raw data", and
	// the closure is the unit of elevation.
	//
	// Nil disables recording. Unlike ElevatedReader, nil here is NOT a deny:
	// a wiring site may legitimately have no audit sink (CLI, tests), and
	// refusing to run automations because of that would be a worse failure
	// than an unrecorded read.
	ElevationRecorder ElevationRecorder
}

// ElevationRecorder receives a notification when a rela.bypass_acl closure
// used its elevated READ capability. Defined here at the consumer per
// CLAUDE.md "interfaces at the call site" — one method, which is all the
// binding needs; the wiring site adapts it onto the audit sink.
type ElevationRecorder interface {
	// RecordElevatedRead is called at most once per closure, after it
	// returns, when at least one elevated read occurred. bindings names the
	// distinct admin read methods used (e.g. "get_entity,list_entities") so
	// the record says what kind of access happened without recording the
	// data itself.
	//
	// Implementations must not block: this runs on the script's goroutine
	// inside the cascade. Errors are the implementation's to handle — the
	// binding has no way to surface them and a failed audit write must not
	// fail the automation.
	RecordElevatedRead(ctx context.Context, bindings []string)
}
