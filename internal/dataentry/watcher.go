package dataentry

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/Sourcehaven-BV/rela/internal/workspace"
)

// sseEvent represents a Server-Sent Event with optional JSON data.
type sseEvent struct {
	Type string // Event type (e.g., "refresh", "entity:created", "git:status")
	Data string // JSON data payload (empty for simple events)
}

// eventBroker manages SSE client connections for live-reload notifications.
type eventBroker struct {
	mu      sync.Mutex
	clients map[chan sseEvent]struct{}
}

func newEventBroker() *eventBroker {
	return &eventBroker{clients: make(map[chan sseEvent]struct{})}
}

func (b *eventBroker) subscribe() chan sseEvent {
	ch := make(chan sseEvent, 4)
	b.mu.Lock()
	b.clients[ch] = struct{}{}
	b.mu.Unlock()
	return ch
}

func (b *eventBroker) unsubscribe(ch chan sseEvent) {
	b.mu.Lock()
	delete(b.clients, ch)
	b.mu.Unlock()
}

// broadcast sends a simple event (backward compatible).
func (b *eventBroker) broadcast(eventType string) {
	b.broadcastEvent(sseEvent{Type: eventType, Data: eventType})
}

// broadcastEvent sends an event with optional JSON data.
func (b *eventBroker) broadcastEvent(event sseEvent) {
	b.mu.Lock()
	for ch := range b.clients {
		select {
		case ch <- event:
		default: // skip slow clients
		}
	}
	b.mu.Unlock()
}

// broadcastEntityEvent sends an entity mutation event (create/update/delete).
func (b *eventBroker) broadcastEntityEvent(action, entityType, entityID string) {
	data, _ := json.Marshal(map[string]string{
		"type": entityType,
		"id":   entityID,
	})
	b.broadcastEvent(sseEvent{
		Type: "entity:" + action,
		Data: string(data),
	})
}

// broadcastGitStatus sends a git status update event.
func (b *eventBroker) broadcastGitStatus() {
	b.broadcastEvent(sseEvent{Type: "git:status", Data: "{}"})
}

// StartWatching begins file watching for live-reload of views when project
// files change. The workspace handles metamodel + graph reload; this method
// adds dataentry-specific side-effects (config reload, SSE broadcast).
// Stop via a.ws.StopWatching().
//
// coverage-ignore: requires real filesystem events via fsnotify
func (a *App) StartWatching() error {
	paths := a.ws.Paths()
	configPath := filepath.Join(paths.Root, ConfigFile)
	metamodelDir := filepath.Join(paths.Root, "metamodel")

	return a.ws.StartWatching(workspace.WatchOptions{
		ExtraFiles: []string{configPath},
		ExtraDirs:  []string{metamodelDir},
		OnReload: func(events []workspace.ChangeEvent) {
			for _, e := range events {
				slog.Debug("file changed", "path", e.Path, "op", e.Op)
			}
			a.onReload(events)
			a.broker.broadcast("refresh")
		},
	})
}

// StartGitFetch begins periodic git fetch in the background.
// Returns a stop function to shut down the fetcher.
//
// coverage-ignore: background goroutine with timer
func (a *App) StartGitFetch() (stop func()) {
	a.mu.RLock()
	gitOps := a.gitOps
	cfg := a.Cfg.Git
	a.mu.RUnlock()

	if gitOps == nil || cfg == nil || cfg.FetchInterval <= 0 {
		return func() {} // no-op if git not configured or fetch disabled
	}

	interval := time.Duration(cfg.FetchInterval) * time.Second
	ticker := time.NewTicker(interval)
	done := make(chan struct{})

	go func() {
		for {
			select {
			case <-ticker.C:
				if err := gitOps.Fetch(); err != nil {
					slog.Warn("git fetch error", "error", err)
				} else {
					// Broadcast git status update so UI can refresh
					a.broker.broadcast("git")
				}
			case <-done:
				ticker.Stop()
				return
			}
		}
	}()

	slog.Info("git background fetch started", "interval", interval)
	return func() {
		close(done)
	}
}

// onReload handles dataentry-specific side-effects after the workspace has
// already reloaded the metamodel and re-synced the graph. It re-reads the
// data-entry config if changed, updates convenience aliases, and rebuilds
// styles/templates.
func (a *App) onReload(events []workspace.ChangeEvent) {
	a.mu.Lock()
	defer a.mu.Unlock()

	paths := a.ws.Paths()
	configPath := filepath.Join(paths.Root, ConfigFile)
	metamodelDir := filepath.Join(paths.Root, "metamodel") + string(filepath.Separator)
	needConfigReload := false
	needMetamodelReload := false

	for _, e := range events {
		switch {
		case e.Path == configPath:
			needConfigReload = true
		case e.Path == paths.MetamodelPath:
			needMetamodelReload = true
		case strings.HasPrefix(e.Path, metamodelDir):
			needMetamodelReload = true
		}
	}

	if needConfigReload {
		cfgData, err := a.ws.ReadProjectFile(ConfigFile)
		if err != nil {
			slog.Warn("config reload error", "error", err)
		} else {
			var cfg Config
			if unmarshalErr := yaml.Unmarshal(cfgData, &cfg); unmarshalErr != nil {
				slog.Warn("config parse error", "error", unmarshalErr)
			} else {
				a.Cfg = &cfg
				slog.Info("config reloaded")
			}
		}
	}

	// Update convenience aliases (workspace already reloaded)
	a.meta = a.ws.Meta()
	a.g = a.ws.Graph()

	// Rebuild styles if config or metamodel changed
	if needConfigReload || needMetamodelReload {
		a.styleMap, a.styledTypes = buildStyleMap(a.Cfg, a.meta)
		a.userPalette = a.loadUserPalette()
		a.palette = ResolvePalette(a.Cfg.Palette, a.userPalette)
		// Update OpenAPI generator with new metamodel
		if a.openAPIGen != nil {
			a.openAPIGen.UpdateMetamodel(a.meta)
		}
	}
}

// handleSSE serves Server-Sent Events for live-reload notifications.
// Connected browsers receive events when project files change or entities are modified.
// Event types:
//   - refresh: Files changed, reload needed
//   - git: Git status changed
//   - git:status: Git status update (with empty JSON data)
//   - entity:created: Entity created (data: {"type": "...", "id": "..."})
//   - entity:updated: Entity updated (data: {"type": "...", "id": "..."})
//   - entity:deleted: Entity deleted (data: {"type": "...", "id": "..."})
func (a *App) handleSSE(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming not supported", http.StatusInternalServerError)
		return
	}

	// SSE is reachable only from same-origin or explicitly allow-listed
	// dev origins (enforced by requireSameOrigin middleware). No CORS
	// reflection — that previously let any website subscribe to live
	// project events.

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")

	ch := a.broker.subscribe()
	defer a.broker.unsubscribe(ch)

	// Send initial keepalive
	fmt.Fprintf(w, ": connected\n\n")
	flusher.Flush()

	for {
		select {
		case event := <-ch:
			fmt.Fprintf(w, "event: %s\ndata: %s\n\n", event.Type, event.Data)
			flusher.Flush()
		case <-r.Context().Done():
			return
		}
	}
}

// reloadLockMiddleware wraps an http.Handler so that every request holds
// the App's read-lock, preventing concurrent reloads from swapping state
// mid-request. It also sets no-cache headers to ensure browsers always
// fetch fresh data after file changes trigger a reload.
//
// Note: Static files (/static/*) are served separately and bypass this
// middleware, so they retain normal caching behavior.
func (a *App) reloadLockMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		a.mu.RLock()
		defer a.mu.RUnlock()
		w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
		next.ServeHTTP(w, r)
	})
}
