// Package appbuild assembles the focused services every project
// entry point (rela-server, rela-desktop, future bindings) needs from
// a project directory. It replaces the legacy workspace.Discover path
// for those entry points: callers receive a [Services] holding
// individually-constructed collaborators (store, metamodel,
// entitymanager, searcher, tracer, validator, templater, config
// loader, state KV) rather than a god-object.
//
// What's not here, and why:
//
//   - lua.WriteDeps: derived per-invocation from the static lua read
//     deps plus the per-call write handle. Built by callers that
//     actually invoke scripts (scheduler tick, script command,
//     automation cascade) — see [Services.LuaWriteDeps].
//   - lua.Cache: an implementation detail of *script.Engine. Callers
//     that need it ask the engine via [Services.ScriptEngine].
//   - File watching: each domain owns its own watch story
//     (fsstore self-watches; dataentry subscribes to data-entry.yaml).
//     [Services] has no watcher methods.
package appbuild

import (
	"context"
	"errors"
	"fmt"
	"io"
	"iter"
	"log/slog"
	"os"
	"path/filepath"
	"sync"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/audit"
	"github.com/Sourcehaven-BV/rela/internal/autocascade"
	"github.com/Sourcehaven-BV/rela/internal/automation"
	"github.com/Sourcehaven-BV/rela/internal/caldavalias"
	"github.com/Sourcehaven-BV/rela/internal/config"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/entitymanager"
	"github.com/Sourcehaven-BV/rela/internal/lua"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/project"
	"github.com/Sourcehaven-BV/rela/internal/script"
	"github.com/Sourcehaven-BV/rela/internal/search"
	"github.com/Sourcehaven-BV/rela/internal/state"
	"github.com/Sourcehaven-BV/rela/internal/storage"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/templating"
	"github.com/Sourcehaven-BV/rela/internal/tracer"
	"github.com/Sourcehaven-BV/rela/internal/validator"
	"github.com/Sourcehaven-BV/rela/internal/visibility"
)

// Services exposes the focused collaborators a project entry point
// needs as method accessors. Construct via [Discover] or [New].
//
// Method form (not exported fields) is the established pattern in
// this codebase: it lets *Services satisfy consumer-side service
// interfaces — `scheduler.WorkspaceProvider`, the data-entry app's
// constructor inputs — through structural typing, without adapters
// at the wiring site.
//
// Every exported method is a one-line accessor for a collaborator this facade
// constructs; the count tracks the number of wired services, not an accreting
// public API. Versions() (TKT-N0IKN9) took it to 21 — a documented facade
// exception, not a ratchet target. ScheduledLuaWriteDeps() (TKT-ZF2DTV) takes
// it to 22: it is a scheduler.WorkspaceProvider interface method, so it must be
// exported; the two redactor-parameterized variants behind it are unexported
// precisely to keep this surface from growing by three. CalDAVAliases()
// (TKT-WAA092) takes it to 23: the alias service is constructed here and
// consumed by cmd/rela-server when wiring the data-entry App, so it has to
// cross the package boundary — there is no in-package caller to hide it behind.
// GatedReads() (TKT-UIR41P) takes it to 24: it returns the ACL-bound read
// handles an identity-bearing consumer needs, and it is deliberately ONE method
// returning a bundle rather than three accessors — the three handles must come
// from the same gate to stay consistent, and splitting them would both grow this
// surface by three and let a caller mix a gated reader with a raw tracer.
//
//plimsoll:max-exported-methods=24
type Services struct {
	fs    storage.FS
	paths *project.Context
	meta  *metamodel.Metamodel
	store store.Store
	// versions is the content-versioning service (history reads, version writes,
	// purge), a separate concern injected by the backend recipe — nil on builds
	// without versioning (fs/mem; fsstore uses git). Consumers bind the narrow
	// sub-interface they need; the umbrella is the nil-able field type only.
	versions store.VersionService
	searcher search.Searcher
	// visibleSearcher is the ACL-scoped search seam (TKT-BA8BSX):
	// the generic search.NewVisible wrapper on the fs/memory builds,
	// the pgstore-native implementation on the postgres build.
	visibleSearcher search.VisibleSearcher
	entityManager   entitymanager.EntityManager
	tracer          tracer.Tracer
	validator       validator.Validator
	templater       templating.Templater
	cfgLoader       config.Loader
	stateKV         state.KV
	caldavAliases   *caldavalias.Service
	scriptEngine    *script.Engine
	searchCloser    io.Closer
	acl             acl.ACL
	// aclDeclarative is set when buildACL constructs a Declarative; nil
	// for NopACL, ReadOnlyACL, or when Declarative construction fails.
	aclDeclarative *acl.Declarative
	aclPolicy      *acl.Policy
	audit          audit.Audit

	closeOnce sync.Once
	closeErr  error
}

// FS returns the project filesystem.
func (s *Services) FS() storage.FS { return s.fs }

// Paths returns the project context (root, metamodel path, etc.).
func (s *Services) Paths() *project.Context { return s.paths }

// Meta returns the loaded metamodel.
func (s *Services) Meta() *metamodel.Metamodel { return s.meta }

// Store returns the authoritative store.
func (s *Services) Store() store.Store { return s.store }

// Versions returns the content-versioning service, or nil on a build without
// versioning (fs/mem). Consumers must nil-check and bind the narrow sub-interface
// they use (history read, purge, …) rather than the umbrella.
func (s *Services) Versions() store.VersionService { return s.versions }

// Searcher returns the search service (a sentinel error-searcher when
// the search backend failed to construct).
func (s *Services) Searcher() search.Searcher { return s.searcher }

// VisibleSearcher returns the ACL-scoped search seam. Per-backend: the
// generic scope-filter wrapper on the fs/memory builds, the native
// SQL-composed implementation on the postgres build.
func (s *Services) VisibleSearcher() search.VisibleSearcher { return s.visibleSearcher }

// EntityManager returns the production write path.
func (s *Services) EntityManager() entitymanager.EntityManager { return s.entityManager }

// ACL returns the authorization gate wired into entitymanager. Exposed
// so entry points (rela-server) can render operator warnings based on
// the active policy — e.g. "non-loopback bind without an acl.yaml" —
// without re-reading the file. The returned value is the exact ACL
// the Manager consults.
func (s *Services) ACL() acl.ACL { return s.acl }

// ACLPolicy returns the *acl.Policy parsed from acl.yaml, or nil when
// no policy file was present or the ACL was injected via [WithACL].
// Exposed so the data-entry server can build the policy-backed
// affordance resolver from the same policy the Manager authorizes
// against, without re-reading the file.
func (s *Services) ACLPolicy() *acl.Policy { return s.aclPolicy }

// ACLDeclarative returns the concrete *acl.Declarative when the wired
// ACL is one (the default when acl.yaml is present and parses); nil
// when ACL is NopACL or a test injected something else via [WithACL].
//
// Exposed so the affordance resolver can be built with the same
// Declarative the Manager uses — keeping the group expansion,
// containment, and Source attribution consistent across write authz
// and affordance verdicts. The field is set at construction time
// alongside `acl`; no runtime type assertion at the accessor.
func (s *Services) ACLDeclarative() *acl.Declarative { return s.aclDeclarative }

// Audit returns the audit sink wired into entitymanager. Exposed so
// dataentry handlers can emit `denied-write` rows for short-circuit
// rejections (affordance gates) that never reach the manager.
func (s *Services) Audit() audit.Audit { return s.audit }

// Tracer returns the graph-traversal service.
func (s *Services) Tracer() tracer.Tracer { return s.tracer }

// Validator returns the entity validator wired to the store + meta +
// Lua read deps.
func (s *Services) Validator() validator.Validator { return s.validator }

// Templater returns the entity/relation template service.
func (s *Services) Templater() templating.Templater { return s.templater }

// ScriptEngine returns the Lua script engine. Callers that need the
// engine's shared lua.Cache (for [lua.WithCache] when building runtimes
// directly) reach it via [script.Engine.LuaCache].
func (s *Services) ScriptEngine() *script.Engine { return s.scriptEngine }

// Config returns the project's data-entry config loader.
func (s *Services) Config() config.Loader { return s.cfgLoader }

// State returns the .rela cache-directory KV (or a sentinel error-KV
// when no cache dir is available).
func (s *Services) State() state.KV { return s.stateKV }

// CalDAVAliases is the CalDAV<->rela resource alias service. Never nil: the
// service is always constructed (an empty table is the normal first-run state),
// so consumers need no nil check.
func (s *Services) CalDAVAliases() *caldavalias.Service { return s.caldavAliases }

// LuaReadDeps materializes the read-only Lua capability bundle with
// UNRESTRICTED reads — the operator-trust-boundary wiring used by the CLI
// and the docs runtime, where whoever runs the binary already has the
// project files (RR-17DMC).
//
// Request-scoped and scheduled callers must use [Services.luaReadDepsFor]
// instead, which binds reads to an identity (DEC-O59WM4).
//
// Cheap to call; rebuild per-runtime so future metamodel reloads propagate.
func (s *Services) LuaReadDeps() lua.ReadDeps {
	root := ""
	if s.paths != nil {
		root = s.paths.Root
	}
	return lua.ReadDeps{
		VisibleReader: visibility.Unrestricted(s.store),
		Tracer:        s.tracer,
		Searcher:      s.searcher,
		Meta:          s.meta,
		ProjectRoot:   root,
	}
}

// LuaReadDepsFor materializes a read bundle whose reads are ACL-bound to
// whatever principal is on the ctx AT CALL TIME (DEC-O59WM4). Use it for
// every identity-bearing path: data-entry requests, automations/cascades
// (which run on the triggering user's ctx), and scheduled tasks (which run
// on their `system:*` principal).
//
// redactor supplies field-level verdicts. Callers that have an affordance
// resolver (data-entry) pass one; callers that do not may pass
// [visibility.NopRedactor], which yields row-gating without field
// redaction — weaker, but never wrong, and the row gate is the part that
// keeps whole entities out of a prompt.
//
// Identity is deliberately NOT captured here: the scheduler stamps its
// principal on the per-task ctx while the deps bundle is built separately,
// so binding an identity at construction would make that principal
// invisible. The wrapped reader resolves it per call.
//
// Falls back to unrestricted reads when no Declarative ACL is configured —
// that is the NopACL path, byte-identical to pre-ACL behavior, not a
// bypass. A construction failure does NOT: it REFUSES, via
// [visibility.DenyReader] / [visibility.DenyTracer] (RR-GKCZO5). See
// [scriptEntityReader] for why an unattended path must not degrade to the
// raw store.
func (s *Services) luaReadDepsFor(redactor visibility.FieldRedactor) lua.ReadDeps {
	deps := s.LuaReadDeps()
	deps.VisibleReader = scriptEntityReader(s.store, s.aclDeclarative, redactor)
	deps.Tracer = scriptTracer(s.tracer, s.store, s.aclDeclarative, redactor)
	return deps
}

// scriptEntityReader returns the read-out handle for a script runtime: ACL
// bound to the ctx principal when a Declarative policy exists, otherwise
// the raw store (the NopACL path — byte-identical to pre-ACL behavior, not
// a bypass).
//
// When a policy IS configured but the gate cannot be built, this REFUSES
// (returns [visibility.DenyReader]) rather than degrading to the raw store
// (RR-GKCZO5). These deps back unattended paths — automation cascades and
// scheduled tasks — where quietly reverting to full-graph reads means an
// unbounded, silent disclosure into whatever the job sends onward. An
// operator who configured a policy has stated intent; honoring it by
// failing loudly beats ignoring it by failing open.
func scriptEntityReader(
	st store.Store, d *acl.Declarative, redactor visibility.FieldRedactor,
) lua.EntityReader {
	if d == nil {
		// Named, not bare: this is the NopACL path and the single largest
		// ungated read surface in the tree, so it must show up in
		// `grep -rn visibility.Unrestricted` like every other one
		// (TKT-1WV50C).
		return visibility.Unrestricted(st)
	}
	if redactor == nil {
		redactor = visibility.NopRedactor{}
	}
	gate, err := visibility.NewDeclarativeGate(d)
	if err != nil {
		slog.Error("appbuild: ACL gate unavailable; script reads REFUSED", "err", err)
		return visibility.DenyReader{}
	}
	reader, err := visibility.NewPolicyReader(gate, redactor, st)
	if err != nil {
		slog.Error("appbuild: policy reader unavailable; script reads REFUSED", "err", err)
		return visibility.DenyReader{}
	}
	sr, err := visibility.NewScriptReader(reader, st, gate)
	if err != nil {
		slog.Error("appbuild: script reader unavailable; script reads REFUSED", "err", err)
		return visibility.DenyReader{}
	}
	return sr
}

// scriptTracer returns the traversal handle for a script runtime, wrapped
// in the visibility decorator when a Declarative policy exists. The trace
// bindings are identical either way — gating is entirely inside the
// decorator (hidden nodes pruned with their subtrees, paths through hidden
// intermediates withheld, titles falling back to IDs).
func scriptTracer(
	tr tracer.Tracer, st store.Store, d *acl.Declarative, redactor visibility.FieldRedactor,
) tracer.Tracer {
	if d == nil {
		return tr
	}
	if redactor == nil {
		redactor = visibility.NopRedactor{}
	}
	gate, err := visibility.NewDeclarativeGate(d)
	if err != nil {
		slog.Error("appbuild: ACL gate unavailable; traversal REFUSED", "err", err)
		return visibility.DenyTracer{}
	}
	vt, err := visibility.NewVisibleTracer(tr, gate, redactor, st)
	if err != nil {
		slog.Error("appbuild: visible tracer unavailable; traversal REFUSED", "err", err)
		return visibility.DenyTracer{}
	}
	return vt
}

// LuaWriteDeps materializes the read-write Lua capability bundle with
// UNRESTRICTED reads (see [Services.LuaReadDeps] for when that is right).
// EntityManager goes in as the wide entitymanager.EntityManager; the
// lua.WriteDeps.EntityManager field is narrower (lua.Mutator) and
// accepts any structural match.
func (s *Services) LuaWriteDeps() lua.WriteDeps {
	return lua.WriteDeps{
		ReadDeps:      s.LuaReadDeps(),
		EntityManager: s.entityManager,
	}
}

// LuaWriteDepsFor is [Services.LuaWriteDeps] with ACL-bound reads — the
// scheduler's wiring (its per-task principal governs what the script can
// see; privilege comes from acl.yaml assignments, never from task config).
// Writes are unaffected: they continue through entitymanager's own ACL,
// with rela.bypass_acl as the sole escalation.
func (s *Services) luaWriteDepsFor(redactor visibility.FieldRedactor) lua.WriteDeps {
	return lua.WriteDeps{
		ReadDeps:      s.luaReadDepsFor(redactor),
		EntityManager: s.entityManager,
	}
}

// ScheduledLuaWriteDeps satisfies scheduler.WorkspaceProvider. Reads are
// ACL-bound to the task's principal (stamped on the per-task ctx), with
// privileges coming from acl.yaml — a job sees what its identity may see,
// nothing more (DEC-O59WM4).
//
// KNOWN LIMITATION (RR-7408F5): appbuild has no affordance resolver, so
// scheduled jobs get ROW gating only — **field-level `visible:` redaction
// does NOT apply here**. A job whose identity may read `person` receives
// every property of it, including ones a human with the same role would
// have redacted in the UI. Row gating still bounds WHICH entities reach a
// prompt, which is the larger half, but do not assume field policy is
// enforced on this path. Documented for operators in
// docs/scheduled-tasks.md next to `run_as`; closing it means wiring an
// affordance resolver into appbuild.
func (s *Services) ScheduledLuaWriteDeps() lua.WriteDeps {
	return s.luaWriteDepsFor(nil)
}

// GatedReads returns the read handles bound to whatever principal is on the
// ctx AT CALL TIME — the reader, the traversal handle, and a validator whose
// candidate set comes from that same gated reader.
//
// This is the read bundle for an identity-bearing, non-HTTP consumer. The MCP
// server is the caller: its handlers, resources, prompts, analyze and export
// surfaces all read through the returned reader, so gating is decided once
// here rather than per handler (DEC-ZBI39P).
//
// The validator matters as much as the reader. `Services.Validator()` is built
// over the RAW store, which is right for the unattended paths that own it —
// but a validation rule evaluated for a requester must not read rows the
// requester cannot see, or a hidden value reaches a violation message
// (TKT-3FL2S6). So this builds a second validator over the gated reader.
//
// Identity is deliberately NOT captured here: the wrapped handles resolve the
// ctx principal per call, so ONE bundle serves every request. Under NopACL
// (no acl.yaml) these are the raw store/tracer — byte-identical to pre-ACL
// behavior, not a bypass. A construction failure REFUSES via
// visibility.DenyReader / DenyTracer rather than degrading to raw reads
// (RR-GKCZO5).
//
// KNOWN LIMITATION, same as [Services.ScheduledLuaWriteDeps]: appbuild has no
// affordance resolver, so this is ROW gating only — field-level `visible:`
// redaction does NOT apply. Row gating bounds WHICH entities are reachable,
// which is the larger half; do not assume field policy is enforced here.
func (s *Services) GatedReads() GatedReadBundle {
	reader := scriptEntityReader(s.store, s.aclDeclarative, nil)
	tr := scriptTracer(s.tracer, s.store, s.aclDeclarative, nil)

	deps := s.LuaReadDeps()
	deps.VisibleReader = reader
	deps.Tracer = tr

	return GatedReadBundle{
		Reader:    gatedGraphReader{rows: reader, raw: s.store},
		Tracer:    tr,
		Validator: validator.New(reader, s.meta, deps),
	}
}

// GatedReadBundle is the result of [Services.GatedReads]: the three read
// handles an identity-bearing consumer needs, each ACL-bound to the ctx
// principal at call time.
type GatedReadBundle struct {
	Reader    GatedGraphReader
	Tracer    tracer.Tracer
	Validator validator.Validator
}

// GatedGraphReader is the row-and-tally read surface returned by
// [Services.GatedReads]. Row reads are ACL-gated; the two counts are not —
// see [gatedGraphReader] for why.
type GatedGraphReader interface {
	GetEntity(ctx context.Context, id string) (*entity.Entity, error)
	ListEntities(ctx context.Context, q store.EntityQuery) iter.Seq2[*entity.Entity, error]
	GetRelation(ctx context.Context, from, relType, to string) (*entity.Relation, error)
	ListRelations(ctx context.Context, q store.RelationQuery) iter.Seq2[*entity.Relation, error]
	CountEntities(ctx context.Context, q store.EntityQuery) (int, error)
	CountRelations(ctx context.Context, q store.RelationQuery) (int, error)
}

// gatedGraphReader composes the ACL-gated row reader with the raw store for
// the two operations the gated reader does not provide.
//
// The split is deliberate, and matches the line `internal/dataentry` already
// draws (`analyzeService.relCounts` is documented "raw (ungated) on purpose"):
//
//   - GetEntity / ListEntities / ListRelations go through `rows`, so a hidden
//     entity is absent and a hidden edge is not listed.
//   - CountEntities / CountRelations go to the raw store. A count is
//     STRUCTURAL: it says how many rows of a declared type exist, never which.
//     Entity *existence* is the secret the row gate protects; an aggregate
//     tally of a type the metamodel already publishes is not.
//   - GetRelation goes to the raw store because a relation is addressed by its
//     two endpoint ids, which the caller must already hold. Reading one
//     therefore confirms nothing about entities the caller could not already
//     name. Relations carry no field-level redaction today (see
//     docs/acl-security.md), so this matches what a live relation GET exposes.
//
// If either judgement changes, this is the one type to fix.
type gatedGraphReader struct {
	rows lua.EntityReader
	raw  store.Store
}

func (g gatedGraphReader) GetEntity(ctx context.Context, id string) (*entity.Entity, error) {
	return g.rows.GetEntity(ctx, id)
}

func (g gatedGraphReader) ListEntities(
	ctx context.Context, q store.EntityQuery,
) iter.Seq2[*entity.Entity, error] {
	return g.rows.ListEntities(ctx, q)
}

func (g gatedGraphReader) ListRelations(
	ctx context.Context, q store.RelationQuery,
) iter.Seq2[*entity.Relation, error] {
	return g.rows.ListRelations(ctx, q)
}

func (g gatedGraphReader) GetRelation(
	ctx context.Context, from, relType, to string,
) (*entity.Relation, error) {
	return g.raw.GetRelation(ctx, from, relType, to)
}

func (g gatedGraphReader) CountEntities(ctx context.Context, q store.EntityQuery) (int, error) {
	return g.raw.CountEntities(ctx, q)
}

func (g gatedGraphReader) CountRelations(ctx context.Context, q store.RelationQuery) (int, error) {
	return g.raw.CountRelations(ctx, q)
}

// Collaborators bundles the fully-built dependencies of a [Services]
// instance. Exposed so external test fixtures (`appbuildtest`) and
// alternative composition roots can assemble a Services without
// poking at unexported fields. Production callers go through [New] /
// [Discover] instead.
//
// Every field is required. [NewFromCollaborators] validates them. The
// production wiring builds a Services from a real filesystem, real
// metamodel, real entity manager, etc.; test fixtures supply
// in-memory equivalents (see `appbuildtest`). There is no production
// code path that runs without a complete Services — making any of
// these optional would force every downstream consumer to nil-check
// what it depends on.
//
// The one nuance: SearchCloser may be nil when the search backend
// does not own a closable resource (the error-Searcher placeholder
// has nothing to close).
type Collaborators struct {
	FS       storage.FS
	Paths    *project.Context
	Meta     *metamodel.Metamodel
	Store    store.Store
	Searcher search.Searcher
	// VisibleSearcher may be nil: the constructor then derives the
	// generic search.NewVisible(Searcher, Store) wrapper, which is the
	// correct implementation for every in-process store. Only wire it
	// explicitly to exercise a native implementation.
	VisibleSearcher search.VisibleSearcher
	EntityManager   entitymanager.EntityManager
	Tracer          tracer.Tracer
	Validator       validator.Validator
	Templater       templating.Templater
	CfgLoader       config.Loader
	StateKV         state.KV
	ScriptEngine    *script.Engine
	ACL             acl.ACL
	Audit           audit.Audit

	// Declarative is the optional concrete *acl.Declarative the test
	// is wiring. When non-nil, [Services.ACLDeclarative] returns it;
	// the affordance resolver path then composes against the same
	// resolver the write path uses (RR-FGJR). When nil — typical when
	// ACL is [acl.NopACL] or [acl.ReadOnlyACL] —
	// [Services.ACLDeclarative] returns nil and the dataentry
	// resolver selector falls through to [NopFieldVerdictResolver].
	//
	// If you set Declarative, ACL must reference the same value
	// (typically ACL == Declarative). The constructor enforces this.
	Declarative *acl.Declarative

	// SearchCloser may be nil — see type doc.
	SearchCloser io.Closer
}

// NewFromCollaborators assembles a [Services] from pre-built
// collaborators. Used by external test packages that want to swap
// individual collaborators (e.g. inject a fake store) without going
// through the full production wiring of [New].
//
// Returns an error when any required field is nil. See [Collaborators]
// for the contract.
func NewFromCollaborators(c Collaborators) (*Services, error) {
	if c.FS == nil {
		return nil, errors.New("appbuild.NewFromCollaborators: FS is required")
	}
	if c.Paths == nil {
		return nil, errors.New("appbuild.NewFromCollaborators: Paths is required")
	}
	if c.Meta == nil {
		return nil, errors.New("appbuild.NewFromCollaborators: Meta is required")
	}
	if c.Store == nil {
		return nil, errors.New("appbuild.NewFromCollaborators: Store is required")
	}
	if c.Searcher == nil {
		return nil, errors.New("appbuild.NewFromCollaborators: Searcher is required")
	}
	if c.EntityManager == nil {
		return nil, errors.New("appbuild.NewFromCollaborators: EntityManager is required")
	}
	if c.Tracer == nil {
		return nil, errors.New("appbuild.NewFromCollaborators: Tracer is required")
	}
	if c.Validator == nil {
		return nil, errors.New("appbuild.NewFromCollaborators: Validator is required")
	}
	if c.Templater == nil {
		return nil, errors.New("appbuild.NewFromCollaborators: Templater is required")
	}
	if c.CfgLoader == nil {
		return nil, errors.New("appbuild.NewFromCollaborators: CfgLoader is required")
	}
	if c.StateKV == nil {
		return nil, errors.New("appbuild.NewFromCollaborators: StateKV is required")
	}
	if c.ScriptEngine == nil {
		return nil, errors.New("appbuild.NewFromCollaborators: ScriptEngine is required")
	}
	if c.ACL == nil {
		return nil, errors.New("appbuild.NewFromCollaborators: ACL is required")
	}
	if c.Audit == nil {
		return nil, errors.New("appbuild.NewFromCollaborators: Audit is required (use audit.Nop{} to opt out)")
	}
	if c.Declarative != nil && c.ACL != acl.ACL(c.Declarative) {
		return nil, errors.New("appbuild.NewFromCollaborators: when Declarative is set, ACL must reference the same value")
	}
	var aclPolicy *acl.Policy
	if c.Declarative != nil {
		aclPolicy = c.Declarative.Policy()
	}
	visible := c.VisibleSearcher
	if visible == nil {
		v, err := search.NewVisible(c.Searcher, c.Store)
		if err != nil {
			return nil, fmt.Errorf("appbuild.NewFromCollaborators: derive VisibleSearcher: %w", err)
		}
		visible = v
	}
	return &Services{
		fs:              c.FS,
		paths:           c.Paths,
		meta:            c.Meta,
		store:           c.Store,
		searcher:        c.Searcher,
		visibleSearcher: visible,
		entityManager:   c.EntityManager,
		tracer:          c.Tracer,
		validator:       c.Validator,
		templater:       c.Templater,
		cfgLoader:       c.CfgLoader,
		stateKV:         c.StateKV,
		scriptEngine:    c.ScriptEngine,
		searchCloser:    c.SearchCloser,
		acl:             c.ACL,
		aclDeclarative:  c.Declarative,
		aclPolicy:       aclPolicy,
		audit:           c.Audit,
	}, nil
}

// buildAutomation wires the automation engine + cascade runner from
// the metamodel. Returns (nil, nil, nil) when the metamodel declares
// no automations — Manager treats that as "automation disabled".
func buildAutomation(meta *metamodel.Metamodel) (*automation.Engine, *autocascade.Runner, error) {
	if len(meta.Automations) == 0 {
		return nil, nil, nil
	}
	autoEngine := automation.NewEngineFromMetamodel(meta, meta.Automations)
	cascadeRunner, err := autocascade.New(autocascade.Deps{Engine: autoEngine})
	if err != nil {
		return nil, nil, fmt.Errorf("build autocascade runner: %w", err)
	}
	return autoEngine, cascadeRunner, nil
}

// Option configures construction of a [Services] bundle. Options are
// optional; production callers typically pass none. Used by entry
// points that need to swap a focused collaborator at startup —
// today, `rela-server --read-only` injects [acl.ReadOnlyACL] via
// [WithACL].
type Option func(*options)

type options struct {
	acl acl.ACL

	// databaseURL, when non-empty, supplies the DSN that [Discover] would
	// otherwise read from the environment. Consumed by [Discover] only:
	// [New] takes the DSN from [Config.DatabaseURL], because a caller
	// building a Config already decides where the data lives.
	databaseURL string
}

// WithACL overrides the auto-loaded ACL with the supplied
// implementation. Default behavior (no option) is to load `acl.yaml`
// from the project root via [acl.LoadPolicy]; on `os.ErrNotExist`
// the default falls back to [acl.NopACL] (allow-all). WithACL is
// how `rela-server --read-only` injects [acl.ReadOnlyACL]: the
// option always wins, even when an `acl.yaml` is present, so the
// flag is an unconditional override.
//
// Tests should prefer [NewForTest] + [WithTestACL] over driving this
// path directly.
func WithACL(a acl.ACL) Option {
	return func(o *options) { o.acl = a }
}

// WithDatabaseURL supplies the PostgreSQL DSN explicitly, instead of letting
// [Discover] read it from $RELA_DATABASE_URL. The option always wins over the
// environment, so a caller that knows where a project's data lives never has to
// mutate process state to say so.
//
// This exists so that *which database a project uses* is an argument rather than
// an ambient property. Two [Services] can then be constructed in one process
// against different databases — impossible via the environment, which is global
// and shared. `rela-desktop` already builds a fresh bundle per project switch,
// and a future multi-tenant server resolves a DSN per tenant.
//
// The invariant this must not break: a DSN carries a password, so it must never
// reach a command line. Passing one here in Go code is fine — sourcing it from a
// flag is not. See [Config.DatabaseURL].
//
// Ignored by the FS and memory builds, which have no DSN.
func WithDatabaseURL(dsn string) Option {
	return func(o *options) { o.databaseURL = dsn }
}

// resolveDatabaseURL decides which DSN [Discover] hands to [New]: an explicit
// [WithDatabaseURL] wins, otherwise $RELA_DATABASE_URL. getenv is injected so
// the precedence is testable without mutating process environment — which is
// itself the ambient-state problem this seam removes.
//
// Split out of Discover because on the FS and memory builds the resolved DSN is
// never consumed, so a test could not otherwise distinguish "option honored"
// from "option ignored" without a live PostgreSQL.
func resolveDatabaseURL(opts []Option, getenv func(string) string) string {
	var o options
	for _, opt := range opts {
		opt(&o)
	}
	if o.databaseURL != "" {
		return o.databaseURL
	}
	return getenv("RELA_DATABASE_URL")
}

// loadACLPolicy reads `acl.yaml` from projectRoot and returns the
// parsed [*acl.Policy], or (nil, nil) when the file is genuinely
// missing (in which case the caller falls back to NopACL — no policy
// declared, no access control desired).
//
// A malformed acl.yaml returns a non-nil error: silently degrading to
// NopACL on parse failure would invert the operator's intent and boot
// the server allow-all on a typo. Per CLAUDE.md "Constructors reject
// nil required fields ... never substitute a no-op silently."
//
// Separated from [buildACL] so the caller can open the store between
// the two phases — v1's [acl.Declarative] needs a [acl.Graph] backed
// by the store.
func loadACLPolicy(projectRoot string) (*acl.Policy, error) {
	policy, err := acl.LoadPolicy(filepath.Join(projectRoot, "acl.yaml"))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			// File genuinely absent → caller falls back to NopACL by
			// intent. Returning (nil, nil) keeps the call site's
			// "if aclPolicy != nil" check simple; a sentinel error
			// would force every caller to errors.Is unnecessarily.
			return nil, nil //nolint:nilnil // (nil, nil) = no policy intended; caller checks aclPolicy != nil
		}
		return nil, fmt.Errorf("appbuild: load acl.yaml: %w", err)
	}
	return policy, nil
}

// buildACL constructs the production ACL from a policy + a store. The
// store backs the [acl.Graph] adapter the resolver needs for member-of
// walks and ancestor probes. A nil policy yields [acl.NopACL]
// (allow-all) — the absence of acl.yaml is taken as "no access control
// intended," which is different from a malformed acl.yaml (handled in
// [loadACLPolicy]).
//
// Returns both the [acl.ACL] interface (consumed by entitymanager) and
// the concrete *acl.Declarative (consumed by the affordance resolver).
// When the result is a Declarative, both returns reference the same
// value — making the "write authz and affordance verdicts share one
// resolver" invariant textual rather than implicit-via-type-assertion.
// For NopACL the second return is nil.
//
// An error from [acl.NewDeclarative] is propagated, not downgraded:
// the operator wrote a policy and the resolver couldn't accept it; the
// server must fail to boot rather than silently allow-all.
// metamodelView adapts *metamodel.Metamodel to acl.MetamodelView. The acl
// package deliberately does not depend on internal/metamodel
// (.go-arch-lint.yml), so it declares the narrow view it needs and the
// wiring site — which imports both — supplies this adapter. Uniqueness is
// computed from the already-exported EntityDef accessors, keeping it out
// of Metamodel's public API (plimsoll load line).
type metamodelView struct{ m *metamodel.Metamodel }

func (v metamodelView) HasEntityType(entityType string) bool {
	return v.m.HasEntityType(entityType)
}

func (v metamodelView) PropertyInfo(entityType, property string) acl.PropertyInfo {
	def, ok := v.m.GetEntityDef(entityType)
	if !ok {
		return acl.PropertyInfo{}
	}
	pd, ok := def.PropertyDefs()[property]
	if !ok {
		return acl.PropertyInfo{}
	}
	return acl.PropertyInfo{Exists: true, Unique: pd.Unique, List: pd.List}
}

func buildACL(policy *acl.Policy, meta *metamodel.Metamodel, st store.Store) (acl.ACL, *acl.Declarative, error) {
	if policy == nil {
		return acl.NopACL{}, nil, nil
	}
	// Schema-dependent policy validation (principal_property references a
	// real, unique property; user_entity_type is a declared type). Run
	// here rather than in acl.LoadPolicy because the acl package
	// deliberately does not depend on metamodel; a mistake must fail the
	// boot, not silently mis-resolve identities at runtime.
	if err := policy.ValidateAgainstMetamodel(metamodelView{meta}); err != nil {
		return nil, nil, fmt.Errorf("appbuild: validate acl policy against metamodel: %w", err)
	}
	// `st` is passed twice: once via NewStoreGraph (the Graph
	// adapter the resolver uses for member-of / ancestor walks), and
	// once as the GraphQueryer (executes store.MatchingIDs for
	// Request.PermitsRead / PermitsReadMany). The store.Store
	// interface embeds both — RR-U06D. A future backend or
	// store-wrapping decorator (audit, metrics) MUST forward
	// GraphQueryer or this compiles while the read gate silently uses
	// the wrong store.
	//
	// WithPrincipalLookup supplies the store-backed resolver for the
	// `principal_property` substitution; NewDeclarative requires it iff
	// the policy enables that lookup (else it is an inert dependency).
	d, err := acl.NewDeclarative(policy, acl.NewStoreGraph(st), st,
		acl.WithPrincipalLookup(acl.NewStorePrincipalLookup(st)))
	if err != nil {
		return nil, nil, fmt.Errorf("appbuild: build acl.Declarative: %w", err)
	}
	return d, d, nil
}

// Config carries the inputs every build of [New] needs, plus
// backend-specific configuration that only some builds consume.
//
// The build-agnostic fields (FS, Paths, ScriptEngine, Audit) are
// required by every scenario — even the postgres build still reads the
// metamodel and templates from the filesystem (see Paths). DatabaseURL
// is consumed only by the postgres build and ignored by the FS and
// memory builds; this is the seam where backend-specific configuration
// enters the composition root without forcing other builds to
// acknowledge it through shared parameters.
type Config struct {
	FS           storage.FS
	Paths        *project.Context
	ScriptEngine *script.Engine
	Audit        audit.Audit

	// DatabaseURL is the PostgreSQL connection string. The caller decides
	// where it comes from: [Discover] takes it from [WithDatabaseURL] or, as a
	// fallback, $RELA_DATABASE_URL; a caller building a Config directly (e.g.
	// rela-desktop, or a per-tenant lookup) simply sets it.
	//
	// The invariant is that a DSN must **never reach a command line** — it
	// carries a password, and a flag would put it in `ps` output and shell
	// history. That is why no binary exposes a --database-url flag. It is NOT
	// an invariant that the value come from the environment; passing it in Go
	// code is fine and is what makes two differently-backed Services
	// constructible in one process.
	//
	// Consumed only by the postgres build; empty (and ignored) in the
	// FS/memory builds.
	DatabaseURL string
}

// validate nil-checks the four build-agnostic collaborators. Each build's
// New calls it first; backend-specific validation (e.g. a required DSN)
// lives in that build's recipe.
func (c Config) validate() error {
	if c.FS == nil {
		return errors.New("appbuild.New: Config.FS is required")
	}
	if c.Paths == nil {
		return errors.New("appbuild.New: Config.Paths is required")
	}
	if c.ScriptEngine == nil {
		return errors.New("appbuild.New: Config.ScriptEngine is required")
	}
	if c.Audit == nil {
		return errors.New("appbuild.New: Config.Audit is required (use audit.Nop{} to opt out)")
	}
	return nil
}

// Discover resolves the project at startDir and constructs every
// service the entry points need. scriptEngine is the long-lived Lua
// engine; production callers pass [script.NewEngine].
//
// Discover constructs a production [audit.Filesystem] under
// .rela/audit/ and resolves the database URL (postgres build) from
// [WithDatabaseURL] when supplied, falling back to the RELA_DATABASE_URL
// environment variable. Neither source is a command-line flag, which is the
// property that matters: a DSN carries a password and must not land in `ps`
// output or shell history. The entry point caller is responsible for stamping
// [principal.Principal] onto the request context (this varies per binary — cli,
// mcp, scheduler, data-entry server).
func Discover(startDir string, scriptEngine *script.Engine, opts ...Option) (*Services, error) {
	fs := storage.NewSafeFS(storage.NewOsFS())
	paths, err := project.Discover(startDir, fs)
	if err != nil {
		return nil, fmt.Errorf("discover project: %w", err)
	}
	// Shared startup path for cli, mcp, scheduler and the data-entry server —
	// warns at most once per process.
	project.WarnIfLegacySchema(paths)
	auditSink, auditErr := audit.NewFilesystem(filepath.Join(paths.CacheDir, "audit"))
	if auditErr != nil {
		return nil, fmt.Errorf("build audit sink: %w", auditErr)
	}

	return New(Config{
		FS:           fs,
		Paths:        paths,
		ScriptEngine: scriptEngine,
		Audit:        auditSink,
		DatabaseURL:  resolveDatabaseURL(opts, os.Getenv),
	}, opts...)
}

// buildBase holds the build-agnostic inputs resolved by [prepare] and
// consumed by [assemble]: the validated config, applied options, the
// resolved ACL (+ parsed policy), and the loaded metamodel. The
// per-scenario New recipes thread this between prepare → openBackend →
// assemble so the shared steps are written exactly once.
type buildBase struct {
	cfg       Config
	opts      options
	acl       acl.ACL
	aclPolicy *acl.Policy
	meta      *metamodel.Metamodel
}

// prepare runs the build-agnostic front half of construction: validate
// config, apply options (so a caller-supplied [WithACL] wins over the
// auto-loaded policy), resolve the ACL, and load the metamodel from
// disk. Every build's New calls this before opening its backend.
//
// Resolving the ACL here (rather than in each recipe) lets us tell
// "operator chose NopACL explicitly" from "operator passed nothing and
// the project has no acl.yaml" — both end up NopACL, but only the
// latter triggers the "consider adding an acl.yaml" warning an entry
// point may render.
func prepare(cfg Config, opts []Option) (*buildBase, error) {
	if err := cfg.validate(); err != nil {
		return nil, err
	}

	var o options
	for _, opt := range opts {
		opt(&o)
	}
	resolvedACL := o.acl
	var aclPolicy *acl.Policy
	if resolvedACL == nil {
		// Load the policy YAML up front; ACL construction is deferred
		// to [assemble] because the v1 [acl.Declarative] needs a
		// store-backed [acl.Graph] adapter and the store isn't open
		// yet at this point in the build.
		var err error
		aclPolicy, err = loadACLPolicy(cfg.Paths.Root)
		if err != nil {
			return nil, err
		}
	}

	meta, _, err := metamodel.NewFSLoader(cfg.FS, cfg.Paths.SchemaPath).Load(context.Background())
	if err != nil {
		return nil, fmt.Errorf("load metamodel: %w", err)
	}

	return &buildBase{cfg: cfg, opts: o, acl: resolvedACL, aclPolicy: aclPolicy, meta: meta}, nil
}

// assemble runs the build-agnostic back half: it takes the opened store
// + searcher (built by the per-scenario openBackend) and wires every
// remaining collaborator — automation, tracer, templater, config loader,
// lua read deps, entitymanager, validator, state KV — into a [Services].
//
// Keeping this shared is the invariant that prevents the three New
// recipes from drifting: a recipe may CHOOSE and ORDER backend steps,
// but build-agnostic wiring lives here and nowhere else.
//
// visible may be nil: assemble then derives the generic
// search.NewVisible(searcher, st) wrapper, the correct ACL-scoped
// search implementation for every in-process store. The postgres
// recipe passes its native implementation instead.

// resolveACL settles which ACL the services use, now that the store is open.
//
// Deferred out of prepare because acl.Declarative needs a store-backed Graph.
// A missing policy yields NopACL, but an error from the Declarative
// constructor is propagated — booting allow-all on a policy that failed to
// parse would silently disable authorization.
func resolveACL(base *buildBase, st store.Store) (acl.ACL, *acl.Declarative, error) {
	if base.acl == nil {
		return buildACL(base.aclPolicy, base.meta, st)
	}
	// RR-36UL: when WithACL was passed a *acl.Declarative, surface it as the
	// declarative value too so the affordance resolver path picks it up.
	// Without this, a caller wiring WithACL(declarative) silently gets
	// NopFieldVerdictResolver because Services.ACLDeclarative() returns nil.
	d, _ := base.acl.(*acl.Declarative)
	return base.acl, d, nil
}

func assemble(
	base *buildBase, st store.Store, searcher search.Searcher,
	visible search.VisibleSearcher, searchCloser io.Closer,
) (*Services, error) {
	cfg := base.cfg

	if visible == nil {
		v, err := search.NewVisible(searcher, st)
		if err != nil {
			return nil, fmt.Errorf("appbuild: derive VisibleSearcher: %w", err)
		}
		visible = v
	}

	resolvedACL, aclDeclarative, err := resolveACL(base, st)
	if err != nil {
		return nil, err
	}

	autoEngine, cascadeRunner, err := buildAutomation(base.meta)
	if err != nil {
		return nil, err
	}

	tr := tracer.New(st)
	templater := templating.NewFSTemplater(cfg.FS, cfg.Paths)
	cfgLoader := config.NewFSLoader(cfg.FS, cfg.Paths.Root)

	// Build the static lua read deps once — the ScriptRunner (automation
	// cascades) is constructed with these.
	//
	// Cascade reads are ACL-BOUND to the ACTING USER (DEC-O59WM4,
	// RR-XC0URX): an automation fires in response to someone's write and
	// runs on that person's ctx, so it reads their view — symmetric with
	// its write path, which already gates through entitymanager and needs
	// an explicit allow_acl_bypass to elevate. Escalating reads is
	// deliberately NOT a config default; it is TKT-ACSBSA (an admin-handle
	// extension), so a cascade that needs more must ask for it in the open.
	readDeps := lua.ReadDeps{
		VisibleReader: scriptEntityReader(st, aclDeclarative, nil),
		Tracer:        scriptTracer(tr, st, aclDeclarative, nil),
		Searcher:      searcher,
		Meta:          base.meta,
		ProjectRoot:   cfg.Paths.Root,
	}

	tw, err := CompileTransitions(base.meta, st, resolvedACL)
	if err != nil {
		return nil, fmt.Errorf("compile transitions: %w", err)
	}

	// Content versioning is a separate injected service (pgstore only; nil
	// elsewhere), NOT a store capability the manager type-asserts. Derived once
	// here and threaded into the recorders and the Services bundle.
	versions := versionServiceFor(st)

	stateKV, aliases, err := buildStateAndAliases(cfg.FS, cfg.Paths)
	if err != nil {
		return nil, err
	}

	mgr, err := entitymanager.New(entitymanager.Deps{
		AliasRewriter:           aliases,
		Store:                   st,
		Meta:                    base.meta,
		Templater:               templater,
		Audit:                   cfg.Audit,
		ACL:                     resolvedACL,
		Automations:             autoEngine,
		Cascade:                 cascadeRunner,
		ScriptRunner:            cascadeScriptRunner(cfg.ScriptEngine, readDeps, st, cfg.Audit),
		VersionRecorder:         versionRecorderFor(versions),
		RelationVersionRecorder: relationVersionRecorderFor(versions),
		Transitions:             tw.Enforcer,
		FieldGate:               entitymanager.AllowAllFieldGate{},
		TransitionGuard:         tw.Guard,
		TransitionGraph:         tw.Graph,
	})
	if err != nil {
		return nil, fmt.Errorf("build entitymanager: %w", err)
	}

	val := validator.New(st, base.meta, readDeps)

	// Start the pgstore version-reconciliation sweep (postgres build only; a
	// no-op elsewhere). It captures create/update versions for settled entities;
	// rename/delete are captured synchronously via the entitymanager hook above.
	startVersionSweepIfSupported(st, base.meta)

	return &Services{
		fs:              cfg.FS,
		paths:           cfg.Paths,
		meta:            base.meta,
		store:           st,
		versions:        versions,
		searcher:        searcher,
		visibleSearcher: visible,
		entityManager:   mgr,
		tracer:          tr,
		validator:       val,
		templater:       templater,
		cfgLoader:       cfgLoader,
		stateKV:         stateKV,
		caldavAliases:   aliases,
		scriptEngine:    cfg.ScriptEngine,
		searchCloser:    searchCloser,
		acl:             resolvedACL,
		aclDeclarative:  aclDeclarative,
		aclPolicy:       base.aclPolicy,
		audit:           cfg.Audit,
	}, nil
}

// versionRecorder adapts a store.VersionWriter to the entitymanager's
// consumer-side VersionRecorder, translating the identically-shaped record. It
// exists only to keep entitymanager depending on its own narrow interface
// rather than on store.VersionWriter directly.
type versionRecorder struct {
	w store.VersionWriter
}

func (r versionRecorder) RecordVersion(ctx context.Context, v entitymanager.VersionRecord) error {
	return r.w.WriteVersion(ctx, store.VersionInput{
		EntityID:      v.EntityID,
		Op:            v.Op,
		PrevID:        v.PrevID,
		Type:          v.Type,
		Content:       v.Content,
		Properties:    v.Properties,
		SchemaHash:    v.SchemaHash,
		Projection:    v.Projection,
		PrincipalUser: v.PrincipalUser,
		PrincipalTool: v.PrincipalTool,
		TriggeredBy:   v.TriggeredBy,
	})
}

// versionRecorderFor returns a synchronous version recorder when a versioning
// service is wired (pgstore), or nil when none is (fsstore/memstore — where the
// entitymanager's version hook then no-ops). vs is the injected version service
// (an untyped nil on non-versioning builds); a nil service yields a nil recorder
// so the manager's nil check works. Takes the service rather than type-asserting
// the store — versioning is a separate injected concern, not a store capability.
func versionRecorderFor(vs store.VersionService) entitymanager.VersionRecorder {
	if vs == nil {
		return nil
	}
	return versionRecorder{w: vs}
}

// relationVersionRecorder adapts a store.RelationVersionWriter to the
// entitymanager's consumer-side RelationVersionRecorder. RecordID is left 0 so
// the store resolves the surrogate lineage id from the composite key at write
// time (correct for the synchronous pre-delete / post-rename capture).
type relationVersionRecorder struct {
	w store.RelationVersionWriter
}

func (r relationVersionRecorder) RecordRelationVersion(
	ctx context.Context, v entitymanager.RelationVersionRecord,
) error {
	return r.w.WriteRelationVersion(ctx, store.RelationVersionInput{
		From:          v.From,
		Type:          v.Type,
		To:            v.To,
		Op:            v.Op,
		PrevFrom:      v.PrevFrom,
		PrevTo:        v.PrevTo,
		Content:       v.Content,
		Properties:    v.Properties,
		SchemaHash:    v.SchemaHash,
		Projection:    v.Projection,
		PrincipalUser: v.PrincipalUser,
		PrincipalTool: v.PrincipalTool,
		TriggeredBy:   v.TriggeredBy,
	})
}

// relationVersionRecorderFor mirrors versionRecorderFor for relation versions.
func relationVersionRecorderFor(vs store.VersionService) entitymanager.RelationVersionRecorder {
	if vs == nil {
		return nil
	}
	return relationVersionRecorder{w: vs}
}

// (startVersionSweepIfSupported is defined per build tag in
// versionsweep_postgres.go / versionsweep_nosweep.go — the postgres build starts
// the pgstore reconciliation sweep, every other build no-ops — which keeps this
// build-agnostic file free of any pgstore import. assemble calls it above.)

// Close releases resources held by Services: store first (so any
// in-flight observer callbacks complete), then the search backend.
//
// Safe to call repeatedly and from multiple goroutines; the close
// sequence runs exactly once. Subsequent calls return the same nil
// (no errors are returned from the close path today — store close
// failures are slog.Warn'd).
func (s *Services) Close() error {
	s.closeOnce.Do(func() {
		if s.store != nil {
			if lc, ok := s.store.(store.Lifecycle); ok {
				if err := lc.Close(); err != nil {
					slog.Warn("appbuild: failed to close store", "error", err)
				}
			}
		}
		if s.searchCloser != nil {
			_ = s.searchCloser.Close()
			s.searchCloser = nil
		}
	})
	return s.closeErr
}

// buildStateAndAliases builds the per-user state store and the CalDAV alias
// service that rides on it.
//
// The alias service is its own injected concern (like the version service),
// not a store capability. Both are built BEFORE the entitymanager because its
// rename/delete hooks take the alias service: only the write choke-point knows
// old->new, so an alias rewrite has to ride along with the write.
func buildStateAndAliases(fs storage.FS, paths *project.Context) (state.KV, *caldavalias.Service, error) {
	stateKV, err := buildStateKV(fs, paths)
	if err != nil {
		return nil, nil, err
	}
	aliases, err := caldavalias.New(context.Background(), stateKV)
	if err != nil {
		// A corrupt alias table must NOT be fatal here.
		//
		// This runs on EVERY appbuild path — `rela list`, `analyze`, the MCP
		// server, the desktop app — the vast majority of which never serve
		// CalDAV. Failing hard meant a truncated file in the gitignored cache
		// dir killed every command on a project with no `caldav:` block at all,
		// citing a subsystem the user had never enabled.
		//
		// The fail-loud reasoning (caldavalias.ErrCorrupt) is still right where
		// it applies: starting CalDAV with an empty table makes every synced
		// client re-create its entries as new entities, doubling a user's list.
		// That check now lives at the serving boundary — registerCalDAVRoutes
		// refuses to mount without a healthy table — so the failure lands on
		// the path that can actually cause the damage.
		slog.Warn("caldav alias table unreadable; CalDAV will refuse to serve. "+
			"Delete the file to start fresh (synced clients will re-create their entries).",
			"path", filepath.Join(paths.CacheDir, "caldav", "aliases.json"),
			"error", err)
		return stateKV, nil, nil
	}
	return stateKV, aliases, nil
}

// buildStateKV returns a state.KV rooted at paths.CacheDir, or a
// sentinel-error KV when the cache dir is unavailable.
//
// Workspace.State() panicked on a malformed cache path because
// workspace was a process-singleton; appbuild is constructed per
// project on a long-running desktop, so an invalid cache dir bubbles
// up as a New() error that LoadProject can surface to the UI instead
// of crashing the host.
func buildStateKV(fs storage.FS, paths *project.Context) (state.KV, error) {
	if fs == nil || paths == nil || paths.CacheDir == "" {
		return nopKV{}, nil
	}
	rfs, err := storage.NewRootedFS(fs, paths.CacheDir)
	if err != nil {
		return nil, fmt.Errorf("build state KV: invalid root %q: %w", paths.CacheDir, err)
	}
	return state.NewFSKV(rfs), nil
}

// nopKV is the fallback state.KV used when no cache directory is
// available. Get returns [os.ErrNotExist] so callers that treat
// missing-key as a normal state (e.g. scheduler reading a never-set
// last-run timestamp) continue to work; Put/Delete silently no-op.
//
// This deliberately differs from workspace.nopState (which returned
// "no repository configured" from every method): scheduler-style
// state callers expect missing-key to be silent, not a hard error.
// A "no backend" condition is the same as an empty backend from the
// caller's point of view.
type nopKV struct{}

func (nopKV) Get(context.Context, string) ([]byte, error) { return nil, os.ErrNotExist }
func (nopKV) Put(context.Context, string, []byte) error   { return nil }
func (nopKV) Delete(context.Context, string) error        { return nil }
