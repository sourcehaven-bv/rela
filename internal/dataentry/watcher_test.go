package dataentry

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/project"
	"github.com/Sourcehaven-BV/rela/internal/repository"
	"github.com/Sourcehaven-BV/rela/internal/storage"
	"github.com/Sourcehaven-BV/rela/internal/workspace"
)

// Minimal metamodel YAML for reload tests.
const testReloadMetamodelYAML = `version: "1.0"
entities:
  ticket:
    label: Ticket
    plural: tickets
    id_prefix: "TKT-"
    id_type: sequential
    properties:
      title:
        type: string
        required: true
      status:
        type: string
relations:
  depends_on:
    label: depends on
    from: [ticket]
    to: [ticket]
`

// Minimal data-entry config YAML for reload tests.
const testReloadConfigYAML = `version: "1.0"
app:
  name: "Test App"
forms:
  create_ticket:
    entity_type: ticket
    title: "New Ticket"
    fields:
      - property: title
        label: "Title"
lists:
  tickets:
    entity_type: ticket
    title: "Tickets"
    columns:
      - property: title
        label: "Title"
navigation:
  - label: "Tickets"
    list: tickets
`

// setupReloadTestApp creates an App backed by MemFS for testing reload and broker logic.
func setupReloadTestApp(t *testing.T) (*App, *storage.MemFS) {
	t.Helper()

	fs := storage.NewMemFS()
	root := "/project"

	ctx := &project.Context{
		Root:                 root,
		MetamodelPath:        root + "/metamodel.yaml",
		CacheDir:             root + "/.rela",
		EntitiesDir:          root + "/entities",
		RelationsDir:         root + "/relations",
		TemplatesDir:         root + "/templates",
		EntityTemplatesDir:   root + "/templates/entities",
		RelationTemplatesDir: root + "/templates/relations",
	}

	// Create directory structure
	_ = fs.MkdirAll(ctx.EntitiesDir+"/tickets", 0o755)
	_ = fs.MkdirAll(ctx.RelationsDir, 0o755)
	_ = fs.MkdirAll(ctx.CacheDir, 0o755)
	_ = fs.MkdirAll(ctx.EntityTemplatesDir, 0o755)
	_ = fs.MkdirAll(ctx.RelationTemplatesDir, 0o755)

	// Write metamodel and config files
	_ = fs.WriteFile(ctx.MetamodelPath, []byte(testReloadMetamodelYAML), 0o644)
	_ = fs.WriteFile(root+"/data-entry.yaml", []byte(testReloadConfigYAML), 0o644)

	// Write a sample entity
	_ = fs.WriteFile(ctx.EntitiesDir+"/tickets/TKT-001.md", []byte(`---
id: TKT-001
type: ticket
title: First Ticket
status: open
---
`), 0o644)

	repo := repository.New(fs, ctx)

	meta, _, err := repo.LoadMetamodel()
	if err != nil {
		t.Fatalf("failed to load metamodel: %v", err)
	}

	g, _, syncErr := repo.Sync(meta)
	if syncErr != nil {
		t.Fatalf("failed to sync graph: %v", syncErr)
	}

	cfg := &Config{
		App: AppConfig{Name: "Test App"},
	}

	ws := workspace.NewWithGraph(repo, meta, g)

	app := newAppFromParts(cfg, meta, g)
	app.ws = ws
	app.broker = newEventBroker()

	return app, fs
}

// --- eventBroker tests ---

func TestEventBrokerSubscribeUnsubscribe(t *testing.T) {
	b := newEventBroker()

	ch1 := b.subscribe()
	ch2 := b.subscribe()

	b.mu.Lock()
	count := len(b.clients)
	b.mu.Unlock()
	if count != 2 {
		t.Fatalf("expected 2 subscribers, got %d", count)
	}

	b.unsubscribe(ch1)

	b.mu.Lock()
	count = len(b.clients)
	b.mu.Unlock()
	if count != 1 {
		t.Fatalf("expected 1 subscriber after unsubscribe, got %d", count)
	}

	b.unsubscribe(ch2)

	b.mu.Lock()
	count = len(b.clients)
	b.mu.Unlock()
	if count != 0 {
		t.Fatalf("expected 0 subscribers, got %d", count)
	}
}

func TestEventBrokerBroadcast(t *testing.T) {
	b := newEventBroker()
	ch1 := b.subscribe()
	ch2 := b.subscribe()

	b.broadcast("refresh")

	select {
	case msg := <-ch1:
		if msg.Type != "refresh" {
			t.Errorf("ch1: expected 'refresh', got %q", msg.Type)
		}
	case <-time.After(100 * time.Millisecond):
		t.Error("ch1: timed out waiting for broadcast")
	}

	select {
	case msg := <-ch2:
		if msg.Type != "refresh" {
			t.Errorf("ch2: expected 'refresh', got %q", msg.Type)
		}
	case <-time.After(100 * time.Millisecond):
		t.Error("ch2: timed out waiting for broadcast")
	}
}

func TestEventBrokerBroadcastSkipsSlowClient(t *testing.T) {
	b := newEventBroker()
	ch := b.subscribe()

	// Fill the channel buffer (capacity 4)
	b.broadcast("first")
	b.broadcast("second")
	b.broadcast("third")
	b.broadcast("fourth")
	// Fifth broadcast should not block — slow client is skipped
	b.broadcast("fifth")

	// Drain all 4 buffered messages
	for i := 0; i < 4; i++ {
		<-ch
	}

	// Channel should be empty now (fifth was dropped)
	select {
	case extra := <-ch:
		t.Errorf("expected no more messages, got %q", extra.Type)
	default:
		// expected
	}
}

func TestEventBrokerUnsubscribeIdempotent(_ *testing.T) {
	b := newEventBroker()
	ch := b.subscribe()

	b.unsubscribe(ch)
	// Double unsubscribe should not panic
	b.unsubscribe(ch)
}

func TestEventBrokerConcurrency(t *testing.T) {
	b := newEventBroker()
	var wg sync.WaitGroup

	// Concurrently subscribe, broadcast, and unsubscribe
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ch := b.subscribe()
			b.broadcast("test")
			// drain
			select {
			case <-ch:
			default:
			}
			b.unsubscribe(ch)
		}()
	}
	wg.Wait()

	b.mu.Lock()
	remaining := len(b.clients)
	b.mu.Unlock()
	if remaining != 0 {
		t.Errorf("expected 0 clients after concurrent test, got %d", remaining)
	}
}

// simulateReload mimics what workspace.StartWatching does: reload the
// workspace (metamodel + graph), then call the dataentry onReload callback.
func (a *App) simulateReload(events []repository.ChangeEvent) {
	_, _ = a.ws.Reload()
	a.onReload(events)
}

// --- reload tests ---

func TestReloadEntityChanges(t *testing.T) {
	app, fs := setupReloadTestApp(t)

	// Verify initial state
	initialCount := len(graphForTest(app).AllNodes())

	// Add a new entity file
	_ = fs.WriteFile(app.ws.Paths().EntitiesDir+"/tickets/TKT-002.md", []byte(`---
id: TKT-002
type: ticket
title: Second Ticket
status: open
---
`), 0o644)

	// Reload with a generic entity change (not metamodel or config)
	app.simulateReload([]repository.ChangeEvent{
		{Path: app.ws.Paths().EntitiesDir + "/tickets/TKT-002.md", Op: repository.OpCreate},
	})

	newCount := len(graphForTest(app).AllNodes())
	if newCount != initialCount+1 {
		t.Errorf("expected %d entities after reload, got %d", initialCount+1, newCount)
	}
}

func TestReloadMetamodelChange(t *testing.T) {
	app, fs := setupReloadTestApp(t)

	// Verify initial metamodel has 'ticket' entity
	if _, ok := app.Meta().GetEntityDef("ticket"); !ok {
		t.Fatal("expected 'ticket' in initial metamodel")
	}

	// Write updated metamodel with an additional entity type
	updatedMeta := `version: "1.0"
entities:
  ticket:
    label: Ticket
    plural: tickets
    id_prefix: "TKT-"
    id_type: sequential
    properties:
      title:
        type: string
        required: true
      status:
        type: string
  component:
    label: Component
    plural: components
    id_prefix: "CMP-"
    id_type: sequential
    properties:
      name:
        type: string
        required: true
relations:
  depends_on:
    label: depends on
    from: [ticket]
    to: [ticket]
`
	_ = fs.WriteFile(app.ws.Paths().MetamodelPath, []byte(updatedMeta), 0o644)

	app.simulateReload([]repository.ChangeEvent{
		{Path: app.ws.Paths().MetamodelPath, Op: repository.OpModify},
	})

	if _, ok := app.Meta().GetEntityDef("component"); !ok {
		t.Error("expected 'component' entity in reloaded metamodel")
	}
}

func TestReloadConfigChange(t *testing.T) {
	app, fs := setupReloadTestApp(t)

	originalName := app.Cfg().App.Name

	// Write updated config with a different app name
	updatedConfig := `version: "1.0"
app:
  name: "Updated App"
lists: {}
forms: {}
navigation: []
`
	configPath := app.ws.Paths().Root + "/" + ConfigFile
	_ = fs.WriteFile(configPath, []byte(updatedConfig), 0o644)

	app.simulateReload([]repository.ChangeEvent{
		{Path: configPath, Op: repository.OpModify},
	})

	if app.Cfg().App.Name == originalName {
		t.Error("expected config app name to change after reload")
	}
	if app.Cfg().App.Name != "Updated App" {
		t.Errorf("expected 'Updated App', got %q", app.Cfg().App.Name)
	}
}

func TestReloadBadMetamodelKeepsPrevious(t *testing.T) {
	app, fs := setupReloadTestApp(t)

	original := app.Meta()

	// Write invalid metamodel
	_ = fs.WriteFile(app.ws.Paths().MetamodelPath, []byte(`not: valid: metamodel: {{{`), 0o644)

	app.simulateReload([]repository.ChangeEvent{
		{Path: app.ws.Paths().MetamodelPath, Op: repository.OpModify},
	})

	// Metamodel should be unchanged
	if app.Meta() != original {
		t.Error("expected metamodel to remain unchanged after bad reload")
	}
}

func TestReloadBadConfigKeepsPrevious(t *testing.T) {
	app, fs := setupReloadTestApp(t)

	originalName := app.Cfg().App.Name
	configPath := app.ws.Paths().Root + "/" + ConfigFile

	// Write invalid YAML config
	_ = fs.WriteFile(configPath, []byte(`not: valid: yaml: {{{`), 0o644)

	app.simulateReload([]repository.ChangeEvent{
		{Path: configPath, Op: repository.OpModify},
	})

	// Config should be unchanged
	if app.Cfg().App.Name != originalName {
		t.Errorf("expected config to remain unchanged, got %q", app.Cfg().App.Name)
	}
}

func TestReloadMixedEvents(t *testing.T) {
	app, fs := setupReloadTestApp(t)

	configPath := app.ws.Paths().Root + "/" + ConfigFile
	updatedConfig := `version: "1.0"
app:
  name: "Mixed Update"
lists: {}
forms: {}
navigation: []
`
	_ = fs.WriteFile(configPath, []byte(updatedConfig), 0o644)

	updatedMeta := `version: "1.0"
entities:
  ticket:
    label: Ticket
    plural: tickets
    id_prefix: "TKT-"
    id_type: sequential
    properties:
      title:
        type: string
        required: true
      status:
        type: string
      priority:
        type: string
relations:
  depends_on:
    label: depends on
    from: [ticket]
    to: [ticket]
`
	_ = fs.WriteFile(app.ws.Paths().MetamodelPath, []byte(updatedMeta), 0o644)

	// Reload with both config and metamodel changes at once
	app.simulateReload([]repository.ChangeEvent{
		{Path: configPath, Op: repository.OpModify},
		{Path: app.ws.Paths().MetamodelPath, Op: repository.OpModify},
	})

	if app.Cfg().App.Name != "Mixed Update" {
		t.Errorf("expected config name 'Mixed Update', got %q", app.Cfg().App.Name)
	}

	entDef, ok := app.Meta().GetEntityDef("ticket")
	if !ok {
		t.Fatal("expected 'ticket' in reloaded metamodel")
	}
	if _, hasPriority := entDef.Properties["priority"]; !hasPriority {
		t.Error("expected 'priority' property in reloaded metamodel")
	}
}

// --- handleSSE tests ---

// flusherRecorder wraps httptest.ResponseRecorder to implement http.Flusher.
type flusherRecorder struct {
	*httptest.ResponseRecorder
	flushed int
}

func (f *flusherRecorder) Flush() {
	f.flushed++
	f.ResponseRecorder.Flush()
}

func TestHandleSSEHeaders(t *testing.T) {
	app, _ := setupReloadTestApp(t)

	ctx, cancel := context.WithCancel(context.Background())
	r := httptest.NewRequest(http.MethodGet, "/api/events", http.NoBody).WithContext(ctx)
	w := &flusherRecorder{ResponseRecorder: httptest.NewRecorder()}

	// Run handler in goroutine since it blocks until context is cancelled
	done := make(chan struct{})
	go func() {
		app.handleSSE(w, r)
		close(done)
	}()

	// Give handler time to write headers and initial keepalive
	time.Sleep(50 * time.Millisecond)
	cancel()
	<-done

	if ct := w.Header().Get("Content-Type"); ct != "text/event-stream" {
		t.Errorf("expected Content-Type 'text/event-stream', got %q", ct)
	}
	if cc := w.Header().Get("Cache-Control"); cc != "no-cache" {
		t.Errorf("expected Cache-Control 'no-cache', got %q", cc)
	}
	// Regression: SSE must NOT reflect cross-origin requests via CORS headers.
	// The original bug let any website subscribe to live project events.
	if got := w.Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Errorf("SSE must not set Access-Control-Allow-Origin (security regression), got %q", got)
	}
	if got := w.Header().Get("Access-Control-Allow-Credentials"); got != "" {
		t.Errorf("SSE must not set Access-Control-Allow-Credentials (security regression), got %q", got)
	}
	body := w.Body.String()
	if !strings.Contains(body, ": connected") {
		t.Errorf("expected initial keepalive comment, got %q", body)
	}
}

func TestHandleSSEReceivesEvent(t *testing.T) {
	app, _ := setupReloadTestApp(t)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	r := httptest.NewRequest(http.MethodGet, "/api/events", http.NoBody).WithContext(ctx)
	w := &flusherRecorder{ResponseRecorder: httptest.NewRecorder()}

	done := make(chan struct{})
	go func() {
		app.handleSSE(w, r)
		close(done)
	}()

	// Wait for handler to subscribe
	time.Sleep(50 * time.Millisecond)

	// Broadcast an event
	app.broker.broadcast("refresh")

	// Wait for event to be written
	time.Sleep(50 * time.Millisecond)
	cancel()
	<-done

	body := w.Body.String()
	if !strings.Contains(body, "event: refresh") {
		t.Errorf("expected 'event: refresh' in body, got %q", body)
	}
	if !strings.Contains(body, "data: refresh") {
		t.Errorf("expected 'data: refresh' in body, got %q", body)
	}
}

func TestHandleSSENoFlusher(t *testing.T) {
	app, _ := setupReloadTestApp(t)

	r := httptest.NewRequest(http.MethodGet, "/api/events", http.NoBody)
	// Plain ResponseRecorder does not implement http.Flusher — but actually
	// httptest.ResponseRecorder does implement Flusher in Go 1.12+.
	// Use a custom writer that does NOT implement Flusher.
	w := &nonFlusherWriter{header: make(http.Header)}

	app.handleSSE(w, r)

	if w.code != http.StatusInternalServerError {
		t.Errorf("expected 500 for non-flusher writer, got %d", w.code)
	}
}

// nonFlusherWriter is an http.ResponseWriter that does NOT implement http.Flusher.
type nonFlusherWriter struct {
	header http.Header
	code   int
	body   strings.Builder
}

func (w *nonFlusherWriter) Header() http.Header         { return w.header }
func (w *nonFlusherWriter) WriteHeader(code int)        { w.code = code }
func (w *nonFlusherWriter) Write(b []byte) (int, error) { return w.body.Write(b) }

// --- noCacheMiddleware tests ---

func TestNoCacheMiddleware(t *testing.T) {
	app, _ := setupReloadTestApp(t)

	called := false
	inner := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	})

	handler := app.noCacheMiddleware(inner)
	r := httptest.NewRequest(http.MethodGet, "/", http.NoBody)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, r)

	if !called {
		t.Error("expected inner handler to be called")
	}
	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
}

func TestNoCacheMiddlewareSetsHeader(t *testing.T) {
	app, _ := setupReloadTestApp(t)

	inner := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	handler := app.noCacheMiddleware(inner)
	r := httptest.NewRequest(http.MethodGet, "/", http.NoBody)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, r)

	cc := w.Header().Get("Cache-Control")
	if cc != "no-cache, no-store, must-revalidate" {
		t.Errorf("expected Cache-Control header, got %q", cc)
	}
}

func TestConcurrentReadDuringOnReload(t *testing.T) {
	// With App.mu deleted, handlers never block on reload — they
	// observe either the pre-reload or post-reload snapshot.
	//
	// This test asserts the snapshot's *cross-field* invariants hold
	// at every observation: a reader that loads State() sees a Cfg,
	// Meta, Graph, StyleMap, etc. that all came from the same publish.
	// A regression that publishes one field independently of another
	// would break the StyleMap entry-count invariant below.
	app, _ := setupReloadTestApp(t)

	const readers = 8
	const duration = 200 * time.Millisecond
	stop := make(chan struct{})
	var wg sync.WaitGroup

	for i := 0; i < readers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-stop:
					return
				default:
				}
				s := app.State()
				if s == nil {
					t.Errorf("state.Load() returned nil")
					return
				}
				if s.Cfg == nil || s.Meta == nil {
					t.Errorf("state.Load() returned incomplete snapshot: cfg=%v meta=%v",
						s.Cfg != nil, s.Meta != nil)
					return
				}
				// Cross-field invariant: a published AppState always
				// has a StyleMap that covers every property type in
				// its metamodel. A torn publish would yield zero or
				// fewer entries.
				if len(s.StyleMap) < len(s.Meta.Types) {
					t.Errorf("torn snapshot: %d StyleMap entries vs %d metamodel types",
						len(s.StyleMap), len(s.Meta.Types))
					return
				}
			}
		}()
	}

	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			select {
			case <-stop:
				return
			default:
			}
			app.onReload(nil)
		}
	}()

	time.Sleep(duration)
	close(stop)
	wg.Wait()
}
