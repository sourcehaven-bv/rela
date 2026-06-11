package cli

import (
	"log/slog"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/appbuild"
	relamcp "github.com/Sourcehaven-BV/rela/internal/mcp"
	"github.com/Sourcehaven-BV/rela/internal/script"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// mcpServices owns the per-project services the MCP process needs and
// their lifecycle (Close). It is constructed once by [newMCPServices]
// and held for the lifetime of the MCP process. The MCP server itself
// receives a flattened [mcp.Deps] via [mcpServices.Deps] — it never
// holds a reference to this struct, so `internal/mcp` does not depend
// on this wiring code.
//
// The service stack (store, search, metamodel, entitymanager,
// automation, audit, validator, lua deps) is built by [appbuild] — the
// single composition root shared with rela-server and rela-desktop — so
// the MCP wiring is not a second composition root. Only the watcher
// adapter is MCP-specific (appbuild deliberately has no watcher story).
type mcpServices struct {
	svc     *appbuild.Services
	watcher relamcp.Watcher
}

// newMCPServices discovers the project at startDir, builds the focused
// services via [appbuild.Discover], and returns a bundle whose
// [mcpServices.Deps] builds the [mcp.Deps] handed to the server.
//
// MCP is wired with [acl.NopACL] (allow-all) on purpose: it is a local
// stdio transport, so anyone who can launch `rela mcp` already has
// filesystem write access to the entity files and can edit them
// directly, bypassing any gate. The filesystem is the trust boundary
// here, not the tool surface — policy enforcement on MCP would defend
// nothing. Access control that matters belongs on the deployed HTTP API,
// which serves callers who do not have direct file access. WithACL makes
// this an explicit, justified opt-out rather than a silent default.
func newMCPServices(startDir string) (*mcpServices, error) {
	svc, err := appbuild.Discover(startDir, script.NewEngine(), appbuild.WithACL(acl.NopACL{}))
	if err != nil {
		return nil, err
	}
	return &mcpServices{
		svc:     svc,
		watcher: &mcpWatcher{store: svc.Store()},
	}, nil
}

// Deps flattens the per-project services into the focused [mcp.Deps]
// the MCP server consumes. Built once at wiring time; the resulting
// value holds domain types only, so the server has no path back to
// this struct or to any composition-root aggregate.
func (s *mcpServices) Deps() relamcp.Deps {
	return relamcp.Deps{
		Store:         s.svc.Store(),
		Meta:          s.svc.Meta(),
		Tracer:        s.svc.Tracer(),
		Searcher:      s.svc.Searcher(),
		Validator:     s.svc.Validator(),
		EntityManager: s.svc.EntityManager(),
		Config:        s.svc.Config(),
		LuaWriteDeps:  s.svc.LuaWriteDeps(),
		LuaCache:      s.svc.ScriptEngine().LuaCache(),
		Watcher:       s.watcher,
		ProjectRoot:   s.svc.Paths().Root,
	}
}

// Close stops the watcher and releases the underlying services (store
// then search backend, in that order). Safe to call repeatedly — both
// the watcher and [appbuild.Services.Close] are idempotent.
func (s *mcpServices) Close() error {
	if s.watcher != nil {
		s.watcher.Stop()
	}
	return s.svc.Close()
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
