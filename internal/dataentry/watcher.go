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
		OnChange: func(events []workspace.ChangeEvent) {
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
	gitOps := a.gitOps
	cfg := a.State().Cfg.Git

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

// onReload handles dataentry-specific side-effects after the workspace
// has already reloaded the metamodel and re-synced the graph. It re-reads
// the data-entry config if changed, rebuilds styles and palette if either
// config or metamodel changed, then publishes a new AppState snapshot
// atomically. Readers observe either the pre-reload or the post-reload
// snapshot, never a torn state.
func (a *App) onReload(events []workspace.ChangeEvent) {
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

	current := a.State()
	if current == nil {
		return
	}

	// Start from the current snapshot and override fields that changed.
	newCfg := current.Cfg
	if needConfigReload {
		cfgData, err := a.ws.ReadProjectFile(ConfigFile)
		if err != nil {
			slog.Warn("config reload error", "error", err)
		} else {
			var cfg Config
			if unmarshalErr := yaml.Unmarshal(cfgData, &cfg); unmarshalErr != nil {
				slog.Warn("config parse error", "error", unmarshalErr)
			} else {
				newCfg = &cfg
				slog.Info("config reloaded")
			}
		}
	}

	snap := a.ws.Snapshot()
	newMeta := snap.Meta()

	newStyleMap := current.StyleMap
	newStyledTypes := current.StyledTypes
	newUserPalette := current.UserPalette
	newPalette := current.Palette
	newOpenAPI := current.OpenAPIGen

	if needConfigReload || needMetamodelReload {
		newStyleMap, newStyledTypes = buildStyleMap(newCfg, newMeta)
		// On reload, keep the previous palette if the new file is
		// broken — better to show stale colors than crash or wipe.
		if up, err := a.loadUserPalette(); err != nil {
			slog.Warn("watcher: keeping previous user palette",
				"file", userPaletteFile, "error", err)
		} else {
			newUserPalette = up
			newPalette = ResolvePalette(newCfg.Palette, newUserPalette)
		}
		// Update OpenAPI generator with new metamodel (the generator
		// is internally synchronized; we reuse the same instance).
		if newOpenAPI != nil {
			newOpenAPI.UpdateMetamodel(newMeta)
		}
	}

	a.state.Store(&AppState{
		Cfg:          newCfg,
		Meta:         newMeta,
		StyleMap:     newStyleMap,
		StyledTypes:  newStyledTypes,
		UserDefaults: current.UserDefaults,
		Palette:      newPalette,
		UserPalette:  newUserPalette,
		OpenAPIGen:   newOpenAPI,
	})
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

// noCacheMiddleware sets no-cache headers on dynamic responses so that
// browsers always fetch fresh data after file changes trigger a reload.
// This replaces the previous reloadLockMiddleware, which also held the
// now-deleted App.mu read lock — handlers now get coherent reloadable
// state via a.State() without any lock acquisition.
//
// Static files (/static/*) are served separately and bypass this
// middleware, so they retain normal caching behavior.
func (a *App) noCacheMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
		next.ServeHTTP(w, r)
	})
}
