package cli

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"path/filepath"
	"sync"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/audit"
	"github.com/Sourcehaven-BV/rela/internal/autocascade"
	"github.com/Sourcehaven-BV/rela/internal/automation"
	"github.com/Sourcehaven-BV/rela/internal/config"
	"github.com/Sourcehaven-BV/rela/internal/entitymanager"
	"github.com/Sourcehaven-BV/rela/internal/lua"
	relamcp "github.com/Sourcehaven-BV/rela/internal/mcp"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/project"
	"github.com/Sourcehaven-BV/rela/internal/script"
	"github.com/Sourcehaven-BV/rela/internal/search"
	"github.com/Sourcehaven-BV/rela/internal/storage"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/templating"
	"github.com/Sourcehaven-BV/rela/internal/tracer"
	"github.com/Sourcehaven-BV/rela/internal/validator"
)

// mcpServices owns the per-project services the MCP process needs and
// their lifecycle (Close). It is constructed once by [newMCPServices]
// and held for the lifetime of the MCP process. The MCP server itself
// receives a flattened [mcp.Deps] via [mcpServices.Deps] — it never
// holds a reference to this struct, so `internal/mcp` does not depend
// on this wiring code.
type mcpServices struct {
	paths        *project.Context
	meta         *metamodel.Metamodel
	store        store.Store
	searchCloser io.Closer
	tracer       tracer.Tracer
	searcher     search.Searcher
	validator    validator.Validator
	manager      *entitymanager.Manager
	cfg          config.Loader
	scriptEngine *script.Engine
	watcher      relamcp.Watcher

	closeOnce sync.Once
}

// newMCPServices discovers the project at startDir, opens its store
// with the search backend pre-wired as an [store.EntityObserver],
// constructs the focused services MCP needs, and returns a bundle
// whose [mcpServices.Deps] builds the [mcp.Deps] handed to the server.
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

	// openMCPBackend is the per-build seam (see mcp_wiring_{fs,memory,postgres}.go):
	// it opens the store and its searcher together. The FS build pairs an
	// fsstore with an in-memory bleve index; the postgres build builds one
	// pool (from RELA_DATABASE_URL) shared by pgstore and its in-DB search.
	// Everything after this point is build-agnostic.
	st, searcher, searchCloser, backendErr := openMCPBackend(context.Background(), fs, paths, mm)
	if backendErr != nil {
		return nil, backendErr
	}

	svc := &mcpServices{
		paths:        paths,
		meta:         mm,
		store:        st,
		searchCloser: searchCloser,
		tracer:       tracer.New(st),
		searcher:     searcher,
		cfg:          config.NewFSLoader(fs, paths.Root),
		scriptEngine: script.NewEngine(),
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
	auditSink, auditErr := audit.NewFilesystem(filepath.Join(paths.CacheDir, "audit"))
	if auditErr != nil {
		return nil, fmt.Errorf("build audit sink: %w", auditErr)
	}

	mgr, mgrErr := entitymanager.New(entitymanager.Deps{
		Store:        st,
		Meta:         mm,
		Templater:    templating.NewFSTemplater(fs, paths),
		Audit:        auditSink,
		ACL:          acl.NopACL{},
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

// Deps flattens the per-project services into the focused [mcp.Deps]
// the MCP server consumes. Built once at wiring time; the resulting
// value holds domain types only, so the server has no path back to
// this struct or to any composition-root aggregate.
func (s *mcpServices) Deps() relamcp.Deps {
	var root string
	if s.paths != nil {
		root = s.paths.Root
	}
	return relamcp.Deps{
		Store:         s.store,
		Meta:          s.meta,
		Tracer:        s.tracer,
		Searcher:      s.searcher,
		Validator:     s.validator,
		EntityManager: s.manager,
		Config:        s.cfg,
		LuaWriteDeps:  lua.WriteDeps{ReadDeps: s.luaReadDeps(), EntityManager: s.manager},
		LuaCache:      s.scriptEngine.LuaCache(),
		Watcher:       s.watcher,
		ProjectRoot:   root,
	}
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
// search close so no observer callbacks land on a closed index.
//
// Safe to call repeatedly and from multiple goroutines (defer +
// signal-driven shutdown): the close sequence runs exactly once via
// sync.Once. Mirrors appbuild.Services.Close semantics.
func (s *mcpServices) Close() error {
	s.closeOnce.Do(func() {
		if s.watcher != nil {
			s.watcher.Stop()
		}
		if lc, ok := s.store.(store.Lifecycle); ok {
			_ = lc.Close()
		}
		if s.searchCloser != nil {
			_ = s.searchCloser.Close()
			s.searchCloser = nil
		}
	})
	return nil
}

// --- Watcher adapter ---

// storeStartStopper is the optional capability MCP needs from the
// store to start / stop its file watcher. Only fsstore implements it;
// in-memory store backends (memstore, used under //go:build
// memorybackend) cannot watch a filesystem and therefore opt out.
// The adapter silently no-ops in that case — see [mcpWatcher.Start]
// for the operator-visible warning log.
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
	sw, ok := w.store.(storeStartStopper)
	if !ok {
		// Backend doesn't watch (memstore under -tags memorybackend);
		// MCP change notifications will not fire. Warn so operators
		// running a non-FS build see this rather than silently
		// wondering why subscriptions never deliver.
		slog.Warn("mcp: store backend does not support file watching; change notifications are disabled")
		return nil
	}
	return sw.StartWatching()
}

func (w *mcpWatcher) Stop() {
	if sw, ok := w.store.(storeStartStopper); ok {
		sw.StopWatching()
	}
}

func (w *mcpWatcher) Pause()  {}
func (w *mcpWatcher) Resume() {}
