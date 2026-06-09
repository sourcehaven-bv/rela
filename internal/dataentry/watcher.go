package dataentry

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/Sourcehaven-BV/rela/internal/config"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// storeWatcher is the optional capability dataentry needs from a
// store to drive live-reload of entity/relation files. fsstore
// satisfies it; in-memory backends don't. Defined here at the
// consumer per CLAUDE.md "interfaces at the call site".
type storeWatcher interface {
	StartWatching() error
}

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

// StartWatching begins file watching for live-reload. It does two
// independent things:
//
//  1. Subscribes to `data-entry.yaml` via the config loader so SPA
//     reloads pick up dashboard / palette / form config changes.
//  2. Asks the store to start watching its own entity/relation files
//     (feature-detected via [storeWatcher] — fsstore implements it;
//     in-memory backends don't). The store's observer chain handles
//     reindex automatically; no callback wiring is needed at this
//     layer.
//
// The returned error covers only the config-subscriber failure
// (step 1). Store-watcher errors (step 2) are logged via slog.Warn
// because store watching is a live-reload nice-to-have, not a
// startup requirement.
//
// Stop via [App.StopWatching]. Note: that only releases the config
// subscription — the store-watcher lifecycle is owned by the store
// (closed when the store is closed).
//
// coverage-ignore: requires real filesystem events via fsnotify
func (a *App) StartWatching() error {
	// (1) data-entry.yaml subscription.
	if sub, ok := a.cfgLoader.(config.Subscriber); ok {
		stop, err := sub.Subscribe(context.Background(), ConfigFile, func() {
			a.rebuildState(true, false)
			a.broker.broadcast("refresh")
		})
		if err != nil {
			return err
		}
		a.stopConfigWatch = stop
	}

	// (2) Store-level watcher (fsstore). Errors are logged, not
	// returned — store watching is a live-reload nice-to-have, not a
	// startup requirement. Stores that don't implement [storeWatcher]
	// (memstore, etc.) get a debug log instead of silence so
	// "why isn't live-reload working" doesn't require a goose chase.
	if sw, ok := a.store.(storeWatcher); ok {
		if err := sw.StartWatching(); err != nil {
			slog.Warn("store watcher not started", "error", err)
		}
	} else {
		slog.Debug("store does not implement watching; live-reload of external edits disabled",
			"store", fmt.Sprintf("%T", a.store))
	}

	// (3) Store-event -> SSE bridge. Subscribes to the store's change watcher
	// and broadcasts entity changes to connected browsers. This is the path
	// that makes a write by ANOTHER process visible live (the pgstore
	// cross-process feed, TKT-WZYWM9) — and also surfaces fsstore's
	// external-file-edit events, which previously had no consumer. Local API
	// writes flow through here too, so the inline broadcasts in the write
	// handlers were removed to avoid double-broadcasting.
	a.startStoreEventBridge()

	return nil
}

// storeEventBufSize is the buffer for the store-event subscription. The watcher
// drops events on a full buffer (it's lossy by contract), but since each event
// only triggers a re-fetch hint to the browser, a modest buffer plus the
// browser's own re-snapshot keeps the UI consistent.
const storeEventBufSize = 64

// startStoreEventBridge subscribes to the store's change feed and pumps entity
// events to the SSE broker. Only entity create/update/delete are broadcast
// (relations/attachments are not part of the live feed today — matching the
// prior inline-broadcast behavior). Idempotent re-snapshot semantics: a
// duplicate event just nudges the browser to re-fetch again, which is harmless.
//
// # Audit-isolation invariant
//
// The SSE broker NEVER carries audit records. The wire payload is
// `{type, id}` — an entity marker only. Subject.ID / Subject.FromID
// from `denied-write` audit rows must NOT be forwarded here: the
// audit log is the only intended audience for principal-attribution
// detail, and broadcasting it via SSE would leak the principal-to-
// entity topology to every event subscriber.
//
// If a future feature needs an audit-aware SSE channel, it must
// expose a SEPARATE broker, gated by per-subscriber ACL, with its
// own redaction policy. The regression test
// `TestSSE_DoesNotFlowAuditEvents` pins this invariant.
func (a *App) startStoreEventBridge() {
	events, cancel := a.store.Subscribe(storeEventBufSize)
	a.stopStoreWatch = cancel
	go a.pumpStoreEvents(events)
}

// pumpStoreEvents maps store.Events to SSE entity broadcasts until the
// subscription channel closes (on cancel / store Close).
func (a *App) pumpStoreEvents(events <-chan store.Event) {
	for ev := range events {
		switch ev.Op {
		case store.EventEntityCreated:
			a.broker.broadcastEntityEvent("created", ev.EntityType, ev.EntityID)
		case store.EventEntityUpdated:
			a.broker.broadcastEntityEvent("updated", ev.EntityType, ev.EntityID)
		case store.EventEntityDeleted:
			a.broker.broadcastEntityEvent("deleted", ev.EntityType, ev.EntityID)
		default:
			// Relation/attachment events are not part of the live feed (no
			// browser broadcast existed for them before either). Ignored.
		}
	}
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

// rebuildState re-reads changed inputs and publishes a fresh AppState
// snapshot atomically. Readers observe either the pre-reload or the
// post-reload snapshot, never a torn state.
func (a *App) rebuildState(configChanged, metaChanged bool) {
	current := a.State()
	if current == nil {
		return
	}

	newCfg := current.Cfg
	if configChanged {
		cfgData, err := a.cfgLoader.Load(context.Background(), ConfigFile)
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

	newMeta := a.Meta()

	newStyleMap := current.StyleMap
	newStyledTypes := current.StyledTypes
	newUserPalette := current.UserPalette
	newPalette := current.Palette
	newOpenAPI := current.OpenAPIGen

	if configChanged || metaChanged {
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
