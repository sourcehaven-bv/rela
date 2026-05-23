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
	return &Services{
		fs:            c.FS,
		paths:         c.Paths,
		meta:          c.Meta,
		store:         c.Store,
		searcher:      c.Searcher,
		entityManager: c.EntityManager,
		tracer:        c.Tracer,
		validator:     c.Validator,
		templater:     c.Templater,
		cfgLoader:     c.CfgLoader,
		stateKV:       c.StateKV,
		scriptEngine:  c.ScriptEngine,
		searchCloser:  c.SearchCloser,
		acl:           c.ACL,
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

// loadACL reads `acl.yaml` from projectRoot and returns the
// appropriate [acl.ACL] implementation. Missing file → [acl.NopACL]
// (allow-all). Other load errors are logged and downgraded to
// NopACL so a malformed policy never bricks the server — the
// operator sees the error in logs and can fix the file without
// restarting. This is the same "tolerate, warn" philosophy the
// metamodel loader uses.
func loadACL(projectRoot string) acl.ACL {
	policy, err := acl.LoadPolicy(filepath.Join(projectRoot, "acl.yaml"))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return acl.NopACL{}
		}
		slog.Warn("appbuild: failed to load acl.yaml; falling back to NopACL", "error", err)
		return acl.NopACL{}
	}
	return acl.NewDeclarative(policy)
}

// validateNewArgs nil-checks the four required collaborators of [New].
// Extracted so [New] stays under the linter's function-length budget
// — the validation block is mechanical and obscures the construction
// flow when inlined.
func validateNewArgs(fs storage.FS, paths *project.Context, scriptEngine *script.Engine, auditSink audit.Audit) error {
	if fs == nil {
		return errors.New("appbuild.New: fs is required")
	}
	if paths == nil {
		return errors.New("appbuild.New: paths is required")
	}
	if scriptEngine == nil {
		return errors.New("appbuild.New: scriptEngine is required")
	}
	if auditSink == nil {
		return errors.New("appbuild.New: auditSink is required (use audit.Nop{} to opt out)")
	}
	return nil
}

// Discover resolves the project at startDir and constructs every
// service the entry points need. scriptEngine is the long-lived Lua
// engine; production callers pass [script.NewEngine].
//
// Discover constructs a production [audit.Filesystem] under
// .rela/audit/. The entry point caller is responsible for stamping
// [principal.Principal] onto the request context (this varies per
// binary — cli, mcp, scheduler, data-entry server).
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
	return New(fs, paths, scriptEngine, auditSink, opts...)
}

// New builds the focused services bundle over a caller-supplied
// filesystem, project context, script engine, and audit sink. Used
// directly by rela-desktop (which constructs its own per-project FS)
// and indirectly by [Discover].
//
// auditSink is a required collaborator — pass [audit.Nop] explicitly
// to opt out. Silently substituting a Nop would mask wiring bugs that
// drop forensic data on the floor.
func New(
	fs storage.FS,
	paths *project.Context,
	scriptEngine *script.Engine,
	auditSink audit.Audit,
	opts ...Option,
) (*Services, error) {
	if err := validateNewArgs(fs, paths, scriptEngine, auditSink); err != nil {
		return nil, err
	}

	// Apply options first so a caller-supplied [WithACL] wins over
	// the auto-loaded policy. Defaulting after this lets us tell
	// "operator chose NopACL explicitly" from "operator passed
	// nothing and the project has no acl.yaml" — both end up as
	// NopACL, but only the latter triggers the "consider adding an
	// acl.yaml" warning the entry point may render.
	var o options
	for _, opt := range opts {
		opt(&o)
	}
	if o.acl == nil {
		o.acl = loadACL(paths.Root)
	}

	meta, _, err := metamodel.NewFSLoader(fs, paths.MetamodelPath).Load(context.Background())
	if err != nil {
		return nil, fmt.Errorf("load metamodel: %w", err)
	}

	// Build the search backend BEFORE opening the store so it can be
	// installed as an observer at open time. The FS build returns a
	// bleve index; a future Postgres build returns nil here because
	// Postgres indexes inside the store itself.
	searchBackend := newSearchObserver()
	st, err := openStore(fs, paths, meta, asObserver(searchBackend))
	if err != nil {
		return nil, fmt.Errorf("open store: %w", err)
	}

	autoEngine, cascadeRunner, err := buildAutomation(meta)
	if err != nil {
		return nil, err
	}

	tr := tracer.New(st)
	searcher, searchCloser, err := buildSearcher(context.Background(), st, searchBackend)
	if err != nil {
		return nil, fmt.Errorf("build searcher: %w", err)
	}
	templater := templating.NewFSTemplater(fs, paths)
	cfgLoader := config.NewFSLoader(fs, paths.Root)

	// Build the static lua read deps once — the ScriptRunner is
	// constructed with these, and LuaReadDeps re-derives the same
	// shape on demand.
	readDeps := lua.ReadDeps{
		Store:       st,
		Tracer:      tr,
		Searcher:    searcher,
		Meta:        meta,
		ProjectRoot: paths.Root,
	}

	mgr, err := entitymanager.New(entitymanager.Deps{
		Store:        st,
		Meta:         meta,
		Templater:    templater,
		Audit:        auditSink,
		ACL:          o.acl,
		Automations:  autoEngine,
		Cascade:      cascadeRunner,
		ScriptRunner: script.NewLuaScriptRunner(scriptEngine, readDeps),
	})
	if err != nil {
		return nil, fmt.Errorf("build entitymanager: %w", err)
	}

	val := validator.New(st, meta, readDeps)
	stateKV, err := buildStateKV(fs, paths)
	if err != nil {
		return nil, err
	}

	return &Services{
		fs:            fs,
		paths:         paths,
		meta:          meta,
		store:         st,
		searcher:      searcher,
		entityManager: mgr,
		tracer:        tr,
		validator:     val,
		templater:     templater,
		cfgLoader:     cfgLoader,
		stateKV:       stateKV,
		scriptEngine:  scriptEngine,
		searchCloser:  searchCloser,
		acl:           o.acl,
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
