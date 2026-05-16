package cli

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/Sourcehaven-BV/rela/internal/app"
	"github.com/Sourcehaven-BV/rela/internal/autocascade"
	"github.com/Sourcehaven-BV/rela/internal/automation"
	"github.com/Sourcehaven-BV/rela/internal/config"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/entitymanager"
	"github.com/Sourcehaven-BV/rela/internal/lua"
	relamcp "github.com/Sourcehaven-BV/rela/internal/mcp"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/project"
	"github.com/Sourcehaven-BV/rela/internal/script"
	"github.com/Sourcehaven-BV/rela/internal/search"
	"github.com/Sourcehaven-BV/rela/internal/search/bleveindex"
	"github.com/Sourcehaven-BV/rela/internal/storage"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/templating"
	"github.com/Sourcehaven-BV/rela/internal/tracer"
	"github.com/Sourcehaven-BV/rela/internal/validator"
)

// mcpServices is the focused-services bundle MCP needs. It satisfies
// [mcp.Services] without going through workspace. Constructed once by
// [newMCPServices] and held for the lifetime of the MCP process.
type mcpServices struct {
	paths        *project.Context
	meta         *metamodel.Metamodel
	store        store.Store
	backend      *bleveindex.Index
	tracer       tracer.Tracer
	searcher     search.Searcher
	validator    validator.Validator
	manager      *entitymanager.Manager
	cfg          config.Loader
	scriptEngine *script.Engine
	watcher      relamcp.Watcher
}

// compile-time check that *mcpServices satisfies mcp.Services so a
// method-signature drift surfaces here rather than at the call site
// where the value is handed to relamcp.NewServer.
var _ relamcp.Services = (*mcpServices)(nil)

// newMCPServices discovers the project at startDir, opens its store
// with the search backend pre-wired as an [store.EntityObserver],
// constructs the focused services MCP needs, and returns a bundle
// that satisfies [mcp.Services].
func newMCPServices(startDir string) (*mcpServices, error) {
	fs := storage.NewSafeFS(storage.NewOsFS())
	paths, discErr := project.Discover(startDir, fs)
	if discErr != nil {
		return nil, discErr
	}
	mm, _, metaErr := metamodel.NewFSLoader(fs, paths.MetamodelPath).Load(context.Background())
	if metaErr != nil {
		return nil, fmt.Errorf("load metamodel: %w", metaErr)
	}

	// Build the search backend BEFORE opening the store so the store
	// hooks it in as a synchronous observer (TKT-Q1JT pattern).
	var backend *bleveindex.Index
	if idx, idxErr := bleveindex.NewMem(); idxErr == nil {
		backend = idx
	} else {
		slog.Warn("search backend unavailable; MCP search tool will return errors", "error", idxErr)
	}

	factory := &app.FSFactory{FS: fs, Paths: paths}
	if backend != nil {
		factory.AddObserver(backend)
	}

	st, openErr := factory.OpenStore(mm)
	if openErr != nil {
		return nil, fmt.Errorf("open store: %w", openErr)
	}

	// Backfill the initial state — observers are not invoked for
	// entities already on disk when the store opens.
	if backend != nil {
		// Partial-index failures are non-fatal but must be logged so
		// the operator knows search is incomplete.
		if err := backfillBackend(context.Background(), backend, st); err != nil {
			slog.Warn("search index backfill incomplete", "error", err)
		}
	}

	svc := &mcpServices{
		paths:        paths,
		meta:         mm,
		store:        st,
		backend:      backend,
		tracer:       tracer.New(st),
		cfg:          config.NewFSLoader(fs, paths.Root),
		scriptEngine: script.NewEngine(),
	}
	if backend != nil {
		svc.searcher = search.New(st, backend)
	} else {
		svc.searcher = search.ErrSearcher(errors.New("search index not available"))
	}
	svc.validator = validator.New(st, mm, svc.luaReadDeps())

	// Build the Manager. Wire autocascade if metamodel declares
	// automations; the ScriptRunner takes only the static lua.ReadDeps
	// (per-cascade Mutator is supplied via Request.Mutator inside
	// Manager.runWriteCascade — Manager satisfies autocascade.Mutator).
	var autoEngine *automation.Engine
	var cascadeRunner *autocascade.Runner
	if len(mm.Automations) > 0 {
		autoEngine = automation.NewEngineFromMetamodel(mm.Automations)
		r, rerr := autocascade.New(autocascade.Deps{Engine: autoEngine})
		if rerr != nil {
			return nil, fmt.Errorf("build autocascade runner: %w", rerr)
		}
		cascadeRunner = r
	}
	mgr, mgrErr := entitymanager.New(entitymanager.Deps{
		Store:        st,
		Meta:         mm,
		Templater:    templating.NewFSTemplater(fs, paths),
		Automations:  autoEngine,
		Cascade:      cascadeRunner,
		ScriptRunner: script.NewLuaScriptRunner(svc.scriptEngine, svc.luaReadDeps()),
	})
	if mgrErr != nil {
		return nil, fmt.Errorf("build entitymanager: %w", mgrErr)
	}
	svc.manager = mgr

	// Watcher: hand-off to fsstore's own watcher via type assertion.
	// MCP today has no ExtraDirs / ExtraFiles use case, so the
	// adapter is minimal.
	svc.watcher = &mcpWatcher{store: st}

	return svc, nil
}

// --- mcp.Services satisfaction ---

func (s *mcpServices) Store() store.Store                         { return s.store }
func (s *mcpServices) Meta() *metamodel.Metamodel                 { return s.meta }
func (s *mcpServices) Tracer() tracer.Tracer                      { return s.tracer }
func (s *mcpServices) Searcher() search.Searcher                  { return s.searcher }
func (s *mcpServices) Validator() validator.Validator             { return s.validator }
func (s *mcpServices) EntityManager() entitymanager.EntityManager { return s.manager }
func (s *mcpServices) Config() config.Loader                      { return s.cfg }
func (s *mcpServices) Paths() *project.Context                    { return s.paths }
func (s *mcpServices) LuaCache() *lua.Cache                       { return s.scriptEngine.LuaCache() }
func (s *mcpServices) Watcher() relamcp.Watcher                   { return s.watcher }

func (s *mcpServices) LuaWriteDeps() lua.WriteDeps {
	return lua.WriteDeps{ReadDeps: s.luaReadDeps(), EntityManager: s.manager}
}

func (s *mcpServices) luaReadDeps() lua.ReadDeps {
	var root string
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

// Close releases the search backend and store. Store close before
// backend close so no observer callbacks land on a closed bleve.
func (s *mcpServices) Close() error {
	if s.watcher != nil {
		s.watcher.Stop()
	}
	if lc, ok := s.store.(store.Lifecycle); ok {
		_ = lc.Close()
	}
	if s.backend != nil {
		_ = s.backend.Close()
		s.backend = nil
	}
	return nil
}

// --- Watcher adapter ---

// storeStartStopper is the optional capability MCP needs from the
// store to start / stop its file watcher. fsstore implements it; the
// adapter no-ops when the store doesn't.
type storeStartStopper interface {
	StartWatching() error
	StopWatching()
}

// mcpWatcher wraps the store's file watcher to satisfy mcp.Watcher.
// Pause/Resume are no-ops today: fsstore's external watcher does not
// expose pause/resume (it relies on echoTracker self-echo suppression
// to ignore the store's own writes during rename). Keeping the
// methods in the interface preserves the existing API surface and
// leaves room for a future ExtraDirs/ExtraFiles watcher with pause
// semantics.
type mcpWatcher struct {
	store    store.Store
	onChange func()
}

func (w *mcpWatcher) Start(onChange func()) error {
	w.onChange = onChange
	if sw, ok := w.store.(storeStartStopper); ok {
		return sw.StartWatching()
	}
	return nil
}

func (w *mcpWatcher) Stop() {
	if sw, ok := w.store.(storeStartStopper); ok {
		sw.StopWatching()
	}
}

func (w *mcpWatcher) Pause()  {}
func (w *mcpWatcher) Resume() {}

// --- backfill helper ---

// backfillBackend populates a search backend from the store at
// startup. Mirrors the per-entity loop in
// workspace.backfillSearchBackend with the same error-accounting:
// list errors and per-entity index errors are collected and returned
// together, so the caller can log a summary instead of swallowing
// failures silently. Partial-index outcomes are tolerable; a missing
// telemetry path is not.
func backfillBackend(ctx context.Context, backend *bleveindex.Index, st store.Store) error {
	if backend == nil || st == nil {
		return nil
	}
	entities := make([]*entity.Entity, 0)
	var listErrs []error
	for e, err := range st.ListEntities(ctx, store.EntityQuery{}) {
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
