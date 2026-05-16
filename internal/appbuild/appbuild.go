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
	"log/slog"
	"os"
	"sync"

	"github.com/Sourcehaven-BV/rela/internal/app"
	"github.com/Sourcehaven-BV/rela/internal/autocascade"
	"github.com/Sourcehaven-BV/rela/internal/automation"
	"github.com/Sourcehaven-BV/rela/internal/config"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/entitymanager"
	"github.com/Sourcehaven-BV/rela/internal/lua"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/project"
	"github.com/Sourcehaven-BV/rela/internal/script"
	"github.com/Sourcehaven-BV/rela/internal/search"
	"github.com/Sourcehaven-BV/rela/internal/search/bleveindex"
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
	searchBackend *bleveindex.Index

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

// Tracer returns the graph-traversal service.
func (s *Services) Tracer() tracer.Tracer { return s.tracer }

// Validator returns the entity validator wired to the store + meta +
// Lua read deps.
func (s *Services) Validator() validator.Validator { return s.validator }

// Templater returns the entity/relation template service.
func (s *Services) Templater() templating.Templater { return s.templater }

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

// Discover resolves the project at startDir and constructs every
// service the entry points need. scriptEngine is the long-lived Lua
// engine; production callers pass [script.NewEngine].
func Discover(startDir string, scriptEngine *script.Engine) (*Services, error) {
	fs := storage.NewSafeFS(storage.NewOsFS())
	paths, err := project.Discover(startDir, fs)
	if err != nil {
		return nil, fmt.Errorf("discover project: %w", err)
	}
	return New(fs, paths, scriptEngine)
}

// New builds the focused services bundle over a caller-supplied
// filesystem and project context. Used directly by rela-desktop
// (which constructs its own per-project FS) and indirectly by
// [Discover].
func New(fs storage.FS, paths *project.Context, scriptEngine *script.Engine) (*Services, error) {
	if fs == nil {
		return nil, errors.New("appbuild.New: fs is required")
	}
	if paths == nil {
		return nil, errors.New("appbuild.New: paths is required")
	}
	if scriptEngine == nil {
		return nil, errors.New("appbuild.New: scriptEngine is required")
	}

	meta, _, err := metamodel.NewFSLoader(fs, paths.MetamodelPath).Load(context.Background())
	if err != nil {
		return nil, fmt.Errorf("load metamodel: %w", err)
	}

	// Search backend BEFORE store so it can be installed as an
	// observer at open time. Backend failure is non-fatal — Searcher
	// surfaces an explicit error when queried.
	var searchBackend *bleveindex.Index
	if idx, idxErr := bleveindex.NewMem(); idxErr == nil {
		searchBackend = idx
	} else {
		slog.Warn("appbuild: failed to create search index", "error", idxErr)
	}

	factory := &app.FSFactory{FS: fs, Paths: paths}
	if searchBackend != nil {
		factory.AddObserver(searchBackend)
	}
	st, err := factory.OpenStore(meta)
	if err != nil {
		return nil, fmt.Errorf("open store: %w", err)
	}

	// Backfill the search index — observers are NOT invoked for
	// entities already on disk at open time.
	if searchBackend != nil {
		if backfillErr := backfillSearchBackend(context.Background(), searchBackend, st); backfillErr != nil {
			slog.Warn("appbuild: failed to index entities", "error", backfillErr)
		}
	}

	// Wire automation engine + cascade runner; both optional, the
	// pair is supplied to Manager together (Manager's constructor
	// enforces "both or neither").
	var autoEngine *automation.Engine
	var cascadeRunner *autocascade.Runner
	if len(meta.Automations) > 0 {
		autoEngine = automation.NewEngineFromMetamodel(meta.Automations)
		r, rerr := autocascade.New(autocascade.Deps{Engine: autoEngine})
		if rerr != nil {
			return nil, fmt.Errorf("build autocascade runner: %w", rerr)
		}
		cascadeRunner = r
	}

	tr := tracer.New(st)
	var searcher search.Searcher
	if searchBackend != nil {
		searcher = search.New(st, searchBackend)
	} else {
		searcher = search.ErrSearcher(errors.New("search index not available"))
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
		searchBackend: searchBackend,
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
		if s.searchBackend != nil {
			_ = s.searchBackend.Close()
			s.searchBackend = nil
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

// backfillSearchBackend populates a search backend with every entity
// currently in the store. Errors from individual entities are
// collected and returned together so callers see the complete picture.
func backfillSearchBackend(ctx context.Context, backend *bleveindex.Index, s store.Store) error {
	if backend == nil || s == nil {
		return nil
	}
	entities := make([]*entity.Entity, 0)
	var listErrs []error
	for e, err := range s.ListEntities(ctx, store.EntityQuery{}) {
		if err != nil {
			listErrs = append(listErrs, err)
			continue
		}
		entities = append(entities, e)
	}
	indexed, indexErr := backend.IndexBatch(entities)
	if len(listErrs) == 0 && indexErr == nil {
		return nil
	}
	skipped := len(entities) - indexed
	return fmt.Errorf("backfill indexed %d entities, skipped %d, list errors: %v, index error: %w",
		indexed, skipped, listErrs, indexErr)
}
