package dataentry

import (
	"fmt"
	"html/template"
	"log"
	"net/http"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/Sourcehaven-BV/rela/internal/repository"
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
	paths := a.repo.Paths()
	configPath := filepath.Join(paths.Root, ConfigFile)
	metamodelDir := filepath.Join(paths.Root, "metamodel")

	opts := repository.WatchOptions{
		ExtraFiles: []string{configPath},
		ExtraDirs:  []string{metamodelDir},
	}
	return a.repo.Watch(opts, func(events []repository.ChangeEvent) {
		for _, e := range events {
			log.Printf("File changed: %s (%s)", e.Path, e.Op)
		}
		a.reload(events)
		a.broker.broadcast("refresh")
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
					log.Printf("Git fetch error: %v", err)
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

	log.Printf("Git background fetch started (every %v)", interval)
	return func() {
		close(done)
	}
}

// reload re-syncs the graph and optionally reloads metamodel/config when
// the corresponding files have changed. It holds the write lock to ensure
// no handlers are reading stale state during the swap.
func (a *App) reload(events []repository.ChangeEvent) {
	a.mu.Lock()
	defer a.mu.Unlock()

	paths := a.repo.Paths()
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
		cfgData, err := a.repo.ReadProjectFile(ConfigFile)
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

	// Reload metamodel + graph via workspace
	result, err := a.ws.Reload()
	if err != nil {
		log.Printf("Workspace reload error: %v", err)
	} else {
		// Update convenience aliases
		a.meta = a.ws.Meta()
		a.g = a.ws.Graph()
		log.Printf("Graph re-synced: %d entities, %d relations", result.EntitiesLoaded, result.RelationsLoaded)
	}

	// Rebuild styles and templates if config or metamodel changed
	if needConfigReload || needMetamodelReload {
		a.styleMap, a.styledTypes = buildStyleMap(a.Cfg, a.meta)
		tmpl, err := template.New("").Funcs(templateFuncs(a.styleMap, a.styledTypes)).Parse(allTemplates())
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
