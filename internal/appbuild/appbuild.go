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
	"log/slog"
	"os"
	"path/filepath"
	"sync"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/audit"
	"github.com/Sourcehaven-BV/rela/internal/autocascade"
	"github.com/Sourcehaven-BV/rela/internal/automation"
	"github.com/Sourcehaven-BV/rela/internal/config"
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
)

// Services exposes the focused collaborators a project entry point
// needs as method accessors. Construct via [Discover] or [New].
//
// Method form (not exported fields) is the established pattern in
// this codebase: it lets *Services satisfy consumer-side service
// interfaces — `scheduler.WorkspaceProvider`, the data-entry app's
// constructor inputs — through structural typing, without adapters
// at the wiring site.
type Services struct {
	fs            storage.FS
	paths         *project.Context
	meta          *metamodel.Metamodel
	store         store.Store
	searcher      search.Searcher
	entityManager entitymanager.EntityManager
	tracer        tracer.Tracer
	validator     validator.Validator
	templater     templating.Templater
	cfgLoader     config.Loader
	stateKV       state.KV
	scriptEngine  *script.Engine
	searchCloser  io.Closer
	acl           acl.ACL
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

// Searcher returns the search service (a sentinel error-searcher when
// the search backend failed to construct).
func (s *Services) Searcher() search.Searcher { return s.searcher }

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

// LuaReadDeps materializes the read-only Lua capability bundle.
// Cheap to call; rebuild per-runtime so future metamodel reloads
// propagate.
func (s *Services) LuaReadDeps() lua.ReadDeps {
	root := ""
	if s.paths != nil {
		root = s.paths.Root
	}
	return lua.ReadDeps{
		Store:       s.store,
		Tracer:      s.tracer,
		Searcher:    s.searcher,
		Meta:        s.meta,
		ProjectRoot: root,
	}
}

// LuaWriteDeps materializes the read-write Lua capability bundle.
// EntityManager goes in as the wide entitymanager.EntityManager; the
// lua.WriteDeps.EntityManager field is narrower (lua.Mutator) and
// accepts any structural match.
func (s *Services) LuaWriteDeps() lua.WriteDeps {
	return lua.WriteDeps{
		ReadDeps:      s.LuaReadDeps(),
		EntityManager: s.entityManager,
	}
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
	FS            storage.FS
	Paths         *project.Context
	Meta          *metamodel.Metamodel
	Store         store.Store
	Searcher      search.Searcher
	EntityManager entitymanager.EntityManager
	Tracer        tracer.Tracer
	Validator     validator.Validator
	Templater     templating.Templater
	CfgLoader     config.Loader
	StateKV       state.KV
	ScriptEngine  *script.Engine
	ACL           acl.ACL
	Audit         audit.Audit

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
	return &Services{
		fs:             c.FS,
		paths:          c.Paths,
		meta:           c.Meta,
		store:          c.Store,
		searcher:       c.Searcher,
		entityManager:  c.EntityManager,
		tracer:         c.Tracer,
		validator:      c.Validator,
		templater:      c.Templater,
		cfgLoader:      c.CfgLoader,
		stateKV:        c.StateKV,
		scriptEngine:   c.ScriptEngine,
		searchCloser:   c.SearchCloser,
		acl:            c.ACL,
		aclDeclarative: c.Declarative,
		aclPolicy:      aclPolicy,
		audit:          c.Audit,
	}, nil
}

// buildAutomation wires the automation engine + cascade runner from
// the metamodel. Returns (nil, nil, nil) when the metamodel declares
// no automations — Manager treats that as "automation disabled".
func buildAutomation(meta *metamodel.Metamodel) (*automation.Engine, *autocascade.Runner, error) {
	if len(meta.Automations) == 0 {
		return nil, nil, nil
	}
	autoEngine := automation.NewEngineFromMetamodel(meta.Automations)
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
func buildACL(policy *acl.Policy, st store.Store) (acl.ACL, *acl.Declarative, error) {
	if policy == nil {
		return acl.NopACL{}, nil, nil
	}
	d, err := acl.NewDeclarative(policy, acl.NewStoreGraph(st))
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

	// DatabaseURL is the PostgreSQL connection string, sourced from the
	// RELA_DATABASE_URL environment variable (see [Discover]). It is
	// deliberately env-only — never a command-line flag — so the
	// credential-bearing DSN does not leak into process listings or shell
	// history. Consumed only by the postgres build; empty (and ignored) in
	// the FS/memory builds.
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
// .rela/audit/ and resolves the database URL (postgres build) from the
// RELA_DATABASE_URL environment variable — env-only, so the credential
// never appears on a command line. The entry point caller is responsible
// for stamping [principal.Principal] onto the request context (this varies
// per binary — cli, mcp, scheduler, data-entry server).
func Discover(startDir string, scriptEngine *script.Engine, opts ...Option) (*Services, error) {
	fs := storage.NewSafeFS(storage.NewOsFS())
	paths, err := project.Discover(startDir, fs)
	if err != nil {
		return nil, fmt.Errorf("discover project: %w", err)
	}
	auditSink, auditErr := audit.NewFilesystem(filepath.Join(paths.CacheDir, "audit"))
	if auditErr != nil {
		return nil, fmt.Errorf("build audit sink: %w", auditErr)
	}
	return New(Config{
		FS:           fs,
		Paths:        paths,
		ScriptEngine: scriptEngine,
		Audit:        auditSink,
		DatabaseURL:  os.Getenv("RELA_DATABASE_URL"),
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

	meta, _, err := metamodel.NewFSLoader(cfg.FS, cfg.Paths.MetamodelPath).Load(context.Background())
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
func assemble(base *buildBase, st store.Store, searcher search.Searcher, searchCloser io.Closer) (*Services, error) {
	cfg := base.cfg

	// Now the store is open: build the Declarative ACL with a
	// store-backed Graph. This is deferred from prepare because
	// acl.Declarative needs the store. NopACL on missing policy; an
	// error from the Declarative constructor is propagated (don't
	// silently boot allow-all on a broken policy). If the caller
	// injected an ACL via WithACL, base.acl is already set.
	resolvedACL := base.acl
	var aclDeclarative *acl.Declarative
	if resolvedACL == nil {
		var err error
		resolvedACL, aclDeclarative, err = buildACL(base.aclPolicy, st)
		if err != nil {
			return nil, err
		}
	} else if d, ok := resolvedACL.(*acl.Declarative); ok {
		// RR-36UL: when WithACL was passed a *acl.Declarative,
		// surface it on aclDeclarative too so the affordance
		// resolver path picks it up. Without this, a caller wiring
		// WithACL(declarative) silently gets NopFieldVerdictResolver
		// because Services.ACLDeclarative() returns nil.
		aclDeclarative = d
	}

	autoEngine, cascadeRunner, err := buildAutomation(base.meta)
	if err != nil {
		return nil, err
	}

	tr := tracer.New(st)
	templater := templating.NewFSTemplater(cfg.FS, cfg.Paths)
	cfgLoader := config.NewFSLoader(cfg.FS, cfg.Paths.Root)

	// Build the static lua read deps once — the ScriptRunner is
	// constructed with these, and LuaReadDeps re-derives the same
	// shape on demand.
	readDeps := lua.ReadDeps{
		Store:       st,
		Tracer:      tr,
		Searcher:    searcher,
		Meta:        base.meta,
		ProjectRoot: cfg.Paths.Root,
	}

	mgr, err := entitymanager.New(entitymanager.Deps{
		Store:        st,
		Meta:         base.meta,
		Templater:    templater,
		Audit:        cfg.Audit,
		ACL:          resolvedACL,
		Automations:  autoEngine,
		Cascade:      cascadeRunner,
		ScriptRunner: script.NewLuaScriptRunner(cfg.ScriptEngine, readDeps),
	})
	if err != nil {
		return nil, fmt.Errorf("build entitymanager: %w", err)
	}

	val := validator.New(st, base.meta, readDeps)
	stateKV, err := buildStateKV(cfg.FS, cfg.Paths)
	if err != nil {
		return nil, err
	}

	return &Services{
		fs:             cfg.FS,
		paths:          cfg.Paths,
		meta:           base.meta,
		store:          st,
		searcher:       searcher,
		entityManager:  mgr,
		tracer:         tr,
		validator:      val,
		templater:      templater,
		cfgLoader:      cfgLoader,
		stateKV:        stateKV,
		scriptEngine:   cfg.ScriptEngine,
		searchCloser:   searchCloser,
		acl:            resolvedACL,
		aclDeclarative: aclDeclarative,
		aclPolicy:      base.aclPolicy,
		audit:          cfg.Audit,
	}, nil
}

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
