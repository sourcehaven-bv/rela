package dataentry

import (
	"fmt"
	"html/template"
	"log"
	"net/http"
	"path/filepath"
	"sync"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/Sourcehaven-BV/rela/internal/graph"
	"github.com/Sourcehaven-BV/rela/internal/migration"
	"github.com/Sourcehaven-BV/rela/internal/storage"
)

// eventBroker manages SSE client connections for live-reload notifications.
type eventBroker struct {
	mu      sync.Mutex
	clients map[chan string]struct{}
}

func newEventBroker() *eventBroker {
	return &eventBroker{clients: make(map[chan string]struct{})}
}

func (b *eventBroker) subscribe() chan string {
	ch := make(chan string, 1)
	b.mu.Lock()
	b.clients[ch] = struct{}{}
	b.mu.Unlock()
	return ch
}

func (b *eventBroker) unsubscribe(ch chan string) {
	b.mu.Lock()
	delete(b.clients, ch)
	b.mu.Unlock()
}

func (b *eventBroker) broadcast(event string) {
	b.mu.Lock()
	for ch := range b.clients {
		select {
		case ch <- event:
		default: // skip slow clients
		}
	}
	b.mu.Unlock()
}

// StartWatching begins file watching for live-reload of views when project
// files change. Returns a stop function to shut down the watcher.
//
// coverage-ignore: requires real filesystem events via fsnotify
func (a *App) StartWatching() (stop func(), err error) {
	configPath := filepath.Join(a.projCtx.Root, ConfigFile)

	w, err := storage.NewWatcher(storage.WatchConfig{
		Dirs:       []string{a.projCtx.EntitiesDir, a.projCtx.RelationsDir},
		Files:      []string{a.projCtx.MetamodelPath, configPath},
		Extensions: []string{".md", ".yaml", ".yml"},
		Debounce:   500 * time.Millisecond,
		SkipHidden: true,
		OnChange: func(events []storage.ChangeEvent) {
			for _, e := range events {
				log.Printf("File changed: %s (%s)", e.Path, e.Op)
			}
			a.reload(events)
			a.broker.broadcast("refresh")
		},
	})
	if err != nil {
		return nil, err
	}

	go w.Start()
	return w.Stop, nil
}

// reload re-syncs the graph and optionally reloads metamodel/config when
// the corresponding files have changed. It holds the write lock to ensure
// no handlers are reading stale state during the swap.
func (a *App) reload(events []storage.ChangeEvent) {
	a.mu.Lock()
	defer a.mu.Unlock()

	configPath := filepath.Join(a.projCtx.Root, ConfigFile)
	needConfigReload := false
	needMetamodelReload := false

	for _, e := range events {
		switch e.Path {
		case configPath:
			needConfigReload = true
		case a.projCtx.MetamodelPath:
			needMetamodelReload = true
		}
	}

	if needMetamodelReload {
		newMeta, err := a.repo.LoadMetamodel()
		if err != nil {
			if migration.IsMigrationError(err) {
				log.Printf("Metamodel needs migration, skipping reload: run 'rela migrate'")
			} else {
				log.Printf("Metamodel reload error (keeping previous version): %v", err)
			}
		} else {
			a.meta = newMeta
			log.Println("Metamodel reloaded")
		}
	}

	if needConfigReload {
		cfgData, err := a.fs.ReadFile(configPath)
		if err != nil {
			log.Printf("Config reload error: %v", err)
		} else {
			var cfg Config
			if unmarshalErr := yaml.Unmarshal(cfgData, &cfg); unmarshalErr != nil {
				log.Printf("Config parse error: %v", unmarshalErr)
			} else {
				a.Cfg = &cfg
				log.Println("Config reloaded")
			}
		}
	}

	// Re-sync graph from disk
	newGraph := graph.New()
	result, err := a.repo.Sync(a.meta, newGraph)
	if err != nil {
		log.Printf("Graph sync error: %v", err)
	} else {
		a.g = newGraph
		log.Printf("Graph re-synced: %d entities, %d relations", result.EntitiesLoaded, result.RelationsLoaded)
	}

	// Rebuild styles and templates if config or metamodel changed
	if needConfigReload || needMetamodelReload {
		a.styleMap, a.styledTypes = buildStyleMap(a.Cfg, a.meta)
		tmpl, err := template.New("").Funcs(templateFuncs(a.styleMap, a.styledTypes)).Parse(allTemplates)
		if err != nil {
			log.Printf("Template re-parse error: %v", err)
		} else {
			tmpl, err = tmpl.Parse(graphTemplates)
			if err != nil {
				log.Printf("Graph template re-parse error: %v", err)
			} else {
				a.tmpl = tmpl
			}
		}
	}
}

// handleSSE serves Server-Sent Events for live-reload notifications.
// Connected browsers receive "refresh" events when project files change.
func (a *App) handleSSE(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming not supported", http.StatusInternalServerError)
		return
	}

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
			fmt.Fprintf(w, "event: %s\ndata: %s\n\n", event, event)
			flusher.Flush()
		case <-r.Context().Done():
			return
		}
	}
}

// reloadLockMiddleware wraps an http.Handler so that every request holds
// the App's read-lock, preventing concurrent reloads from swapping state
// mid-request.
func (a *App) reloadLockMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		a.mu.RLock()
		defer a.mu.RUnlock()
		next.ServeHTTP(w, r)
	})
}
