package dataentry

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/entitymanager/entitymanagertest"
	"github.com/Sourcehaven-BV/rela/internal/lua"
	"github.com/Sourcehaven-BV/rela/internal/script"
	"github.com/Sourcehaven-BV/rela/internal/state"
	"github.com/Sourcehaven-BV/rela/internal/storage"
	"github.com/Sourcehaven-BV/rela/internal/store/memstore"
	"github.com/Sourcehaven-BV/rela/internal/tracer"
)

// fakeScriptEngine is a test double for documentScriptEngine. Each call
// writes the fake's `stdout` string and records the invocation args so
// tests can assert what the service passed through.
type fakeScriptEngine struct {
	mu     sync.Mutex
	calls  []fakeScriptCall
	stdout func(args fakeScriptCall) string // produces the per-call output
	delay  time.Duration                    // per-call sleep (tests concurrency)
	err    error
}

// callCount returns the number of ExecuteDocument invocations so far.
// Lock the mutex to match the write side in ExecuteDocument; callers
// that drain through a channel already happen-before this read, but
// tests that poll still need the lock.
func (f *fakeScriptEngine) callCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.calls)
}

type fakeScriptCall struct {
	path       string
	documentID string
	entryID    string
	timeout    time.Duration
}

func (f *fakeScriptEngine) ExecuteDocument(path string, _ lua.WriteDeps, stdout io.Writer,
	documentID, entryID string, timeout time.Duration) error {
	call := fakeScriptCall{path: path, documentID: documentID, entryID: entryID, timeout: timeout}
	f.mu.Lock()
	f.calls = append(f.calls, call)
	f.mu.Unlock()

	if f.delay > 0 {
		time.Sleep(f.delay)
	}
	if f.err != nil {
		return f.err
	}
	if f.stdout != nil {
		_, _ = io.WriteString(stdout, f.stdout(call))
	}
	return nil
}

// newTestService builds a documentService wired to an in-memory store
// and KV. The caller supplies any entities the test needs to exist; the
// fakeScriptEngine is returned so tests can inspect/configure it.
func newTestService(t *testing.T, entities ...*entity.Entity) (*documentService, *fakeScriptEngine) {
	t.Helper()

	st := memstore.New()
	for _, e := range entities {
		if err := st.CreateEntity(context.Background(), e); err != nil {
			t.Fatalf("seed entity %q: %v", e.ID, err)
		}
	}

	fs := storage.NewMemFS()
	if err := fs.MkdirAll("/p/.rela", 0o755); err != nil {
		t.Fatalf("mkdir .rela: %v", err)
	}
	kvRoot, err := storage.NewRootedFS(fs, "/p/.rela")
	if err != nil {
		t.Fatalf("NewRootedFS: %v", err)
	}
	kv := state.NewFSKV(kvRoot)

	fake := &fakeScriptEngine{}
	// Deps are never actually read by the fake; the zero value is fine.
	deps := func() lua.WriteDeps { return lua.WriteDeps{} }

	return newDocumentService(st, kv, "/p", fake, deps), fake
}

// reqEntity is a convenience factory for the tests below, all of which
// render the same REQ-001 entity. No test reads the title back, so it's
// a fixed placeholder. If a future test needs a different ID, construct
// an entity.Entity directly.
func reqEntity() *entity.Entity {
	return &entity.Entity{
		ID:         "REQ-001",
		Type:       "requirement",
		Properties: map[string]interface{}{"title": "a req"},
	}
}

// TestDocumentService_ScriptRender_CapturesMarkdown verifies the Lua
// renderer dispatch (AC1): when cfg.Script is set, the service routes
// to scriptEngine.ExecuteDocument, uses captured stdout as markdown,
// and returns HTML through the shared goldmark pipeline.
func TestDocumentService_ScriptRender_CapturesMarkdown(t *testing.T) {
	s, fake := newTestService(t, reqEntity())
	fake.stdout = func(_ fakeScriptCall) string { return "# Heading\n\nbody" }

	result, err := s.Render(context.Background(), "REQ-001", documentRenderConfig{
		ConfigID: "notes",
		Script:   "docs/notes.lua",
		Timeout:  5 * time.Second,
	})
	if err != nil {
		t.Fatalf("Render failed: %v", err)
	}
	if !strings.Contains(result.HTML, "<h1") {
		t.Errorf("expected <h1 in HTML, got %q", result.HTML)
	}
	if !strings.Contains(result.HTML, "body") {
		t.Errorf("expected body text in HTML, got %q", result.HTML)
	}

	if len(fake.calls) != 1 {
		t.Fatalf("expected 1 script call, got %d", len(fake.calls))
	}
	call := fake.calls[0]
	if call.path != "docs/notes.lua" {
		t.Errorf("path: got %q, want %q", call.path, "docs/notes.lua")
	}
	if call.documentID != "notes" {
		t.Errorf("documentID: got %q, want %q", call.documentID, "notes")
	}
	if call.entryID != "REQ-001" {
		t.Errorf("entryID: got %q, want %q", call.entryID, "REQ-001")
	}
	if call.timeout != 5*time.Second {
		t.Errorf("timeout: got %v, want %v", call.timeout, 5*time.Second)
	}
}

// TestDocumentService_ScriptRender_NoDiskCacheWrite verifies AC10:
// script: renders do not populate the disk cache. The frontend hits
// GetCached first; if script renders wrote to disk their old output
// could be served to a later command: config on the same entry.
func TestDocumentService_ScriptRender_NoDiskCacheWrite(t *testing.T) {
	s, fake := newTestService(t, reqEntity())
	fake.stdout = func(_ fakeScriptCall) string { return "lua output" }

	if _, err := s.Render(context.Background(), "REQ-001", documentRenderConfig{
		ConfigID: "notes",
		Script:   "docs/notes.lua",
	}); err != nil {
		t.Fatalf("Render failed: %v", err)
	}

	// After render, GetCached should return nil — nothing was written.
	if result := s.GetCached(context.Background(), "REQ-001"); result != nil {
		t.Errorf("expected no cache entry after script render, got %q", result.HTML)
	}
}

// TestDocumentService_ScriptRender_StaleCommandCacheIgnored verifies
// the second half of AC10: a pre-existing command:-era cache file at
// the same key must not be served for a script: render. The service
// always re-renders for script: configs.
func TestDocumentService_ScriptRender_StaleCommandCacheIgnored(t *testing.T) {
	// Note: the HTTP handler is what skips GetCached for script: docs;
	// Render itself will re-render regardless (doRender for script
	// always runs). This test proves the Render side: a script render
	// returns fresh content even when GetCached would have returned
	// something stale. It complements the handler-level test which
	// exercises the skip.
	s, fake := newTestService(t, reqEntity())
	fake.stdout = func(_ fakeScriptCall) string { return "fresh lua output" }

	// Manually seed a stale command-era cache entry.
	_, hash, err := s.computeDocumentHash(context.Background(), "REQ-001")
	if err != nil {
		t.Fatalf("computeDocumentHash: %v", err)
	}
	cacheFile := docCacheSubdir + "/REQ-001-" + hash + ".html"
	if putErr := s.state.Put(context.Background(), cacheFile, []byte("<p>stale</p>")); putErr != nil {
		t.Fatalf("seed stale cache: %v", putErr)
	}

	result, err := s.Render(context.Background(), "REQ-001", documentRenderConfig{
		ConfigID: "notes",
		Script:   "docs/notes.lua",
	})
	if err != nil {
		t.Fatalf("Render failed: %v", err)
	}
	if strings.Contains(result.HTML, "stale") {
		t.Errorf("script render returned stale cache contents: %q", result.HTML)
	}
	if !strings.Contains(result.HTML, "fresh lua output") {
		t.Errorf("expected fresh output, got %q", result.HTML)
	}
	if fake.callCount() != 1 {
		t.Errorf("expected script engine to run once, got %d calls", fake.callCount())
	}
}

// TestDocumentService_SingleflightNoCollapseAcrossConfigs verifies
// RR-4QSBN / AC8: two concurrent Render calls for the same entry but
// different ConfigIDs must both run the script — singleflight must
// NOT collapse them. Previous key (entryID alone) would have served
// the first caller's output to both.
func TestDocumentService_SingleflightNoCollapseAcrossConfigs(t *testing.T) {
	s, fake := newTestService(t, reqEntity())

	// Slow-down so the two goroutines overlap in singleflight's key
	// window: without the key fix, the second caller would block on
	// the first and receive its output.
	fake.delay = 50 * time.Millisecond
	fake.stdout = func(args fakeScriptCall) string {
		return "output for " + args.documentID
	}

	type renderResult struct {
		docID string
		html  string
		err   error
	}
	resultsCh := make(chan renderResult, 2)
	for _, docID := range []string{"docA", "docB"} {
		go func(d string) {
			r, err := s.Render(context.Background(), "REQ-001", documentRenderConfig{
				ConfigID: d,
				Script:   "docs/" + d + ".lua",
			})
			if err != nil {
				resultsCh <- renderResult{docID: d, err: err}
				return
			}
			resultsCh <- renderResult{docID: d, html: r.HTML}
		}(docID)
	}

	got := map[string]string{}
	for range 2 {
		r := <-resultsCh
		if r.err != nil {
			t.Fatalf("render %s: %v", r.docID, r.err)
		}
		got[r.docID] = r.html
	}

	if !strings.Contains(got["docA"], "output for docA") {
		t.Errorf("docA got wrong output (singleflight collapsed?): %q", got["docA"])
	}
	if !strings.Contains(got["docB"], "output for docB") {
		t.Errorf("docB got wrong output (singleflight collapsed?): %q", got["docB"])
	}
	if fake.callCount() != 2 {
		t.Errorf("expected 2 script executions, got %d", fake.callCount())
	}
}

// TestDocumentService_SingleflightCollapsesSameConfig is the positive
// complement: two concurrent renders of the SAME (entryID, configID)
// should still collapse onto a single execution.
func TestDocumentService_SingleflightCollapsesSameConfig(t *testing.T) {
	s, fake := newTestService(t, reqEntity())
	fake.delay = 50 * time.Millisecond
	fake.stdout = func(_ fakeScriptCall) string { return "same doc" }

	var wg sync.WaitGroup
	for range 2 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _ = s.Render(context.Background(), "REQ-001", documentRenderConfig{
				ConfigID: "same",
				Script:   "docs/same.lua",
			})
		}()
	}
	wg.Wait()

	if fake.callCount() != 1 {
		t.Errorf("expected 1 script execution (singleflight dedupe), got %d", fake.callCount())
	}
}

// TestDocumentService_ScriptError surfaces Lua errors to the caller.
func TestDocumentService_ScriptError(t *testing.T) {
	s, fake := newTestService(t, reqEntity())
	fake.err = errors.New("lua blew up")

	_, err := s.Render(context.Background(), "REQ-001", documentRenderConfig{
		ConfigID: "notes",
		Script:   "docs/notes.lua",
	})
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "lua blew up") {
		t.Errorf("expected Lua error to surface, got %v", err)
	}
}

// TestDocumentService_CacheMemoizeAcrossRenders verifies AC6: Lua
// scripts using rela.cache.memoize share state across successive
// render calls within the same process. The compute fn writes a marker
// line to output/counter.log via rela.write_file; after two renders the
// file should contain exactly one line.
//
// Uses a real script.Engine (not the fake) so we exercise the actual
// rela.cache plumbing end-to-end.
func TestDocumentService_CacheMemoizeAcrossRenders(t *testing.T) {
	projectRoot := t.TempDir()
	// Write the lua script.
	scriptsDir := filepath.Join(projectRoot, "scripts", "docs")
	if err := os.MkdirAll(scriptsDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	scriptBody := `
-- Memoized "compute" writes a line to output/counter.log on first run
-- and returns a cached string on subsequent runs. After two document
-- renders the file should contain exactly one line: proof that the fn
-- only executed once across the two renders.
local got = rela.cache.memoize("counter-key", function()
  rela.write_file("counter.log", "ran\n", {ensure_newline = true})
  return "computed"
end)
print("# doc")
print(got)
`
	if err := os.WriteFile(filepath.Join(scriptsDir, "memo.lua"), []byte(scriptBody), 0o644); err != nil {
		t.Fatalf("write script: %v", err)
	}

	// Build a documentService wired to a real script.Engine + real deps
	// pointing at the tempdir project.
	st := memstore.New()
	if err := st.CreateEntity(context.Background(), reqEntity()); err != nil {
		t.Fatalf("seed: %v", err)
	}
	fs := storage.NewMemFS()
	_ = fs.MkdirAll("/p/.rela", 0o755)
	kvRoot, err := storage.NewRootedFS(fs, "/p/.rela")
	if err != nil {
		t.Fatalf("NewRootedFS: %v", err)
	}
	kv := state.NewFSKV(kvRoot)

	engine := script.NewEngine()
	deps := func() lua.WriteDeps {
		return lua.WriteDeps{
			ReadDeps: lua.ReadDeps{
				Store:       st,
				Tracer:      tracer.New(st),
				ProjectRoot: projectRoot,
			},
			EntityManager: entitymanagertest.PanicOnUse{},
		}
	}
	s := newDocumentService(st, kv, projectRoot, engine, deps)

	cfg := documentRenderConfig{
		ConfigID: "notes",
		Script:   "docs/memo.lua",
		Timeout:  5 * time.Second,
	}

	for i := range 2 {
		if _, renderErr := s.Render(context.Background(), "REQ-001", cfg); renderErr != nil {
			t.Fatalf("render %d: %v", i, renderErr)
		}
	}

	counterFile := filepath.Join(projectRoot, "output", "counter.log")
	data, err := os.ReadFile(counterFile)
	if err != nil {
		t.Fatalf("read counter.log: %v", err)
	}
	// ensure_newline=true on rela.write_file guarantees a trailing
	// newline, so counting '\n' gives the number of lines written.
	lines := strings.Count(string(data), "\n")
	if lines != 1 {
		t.Errorf("expected 1 line in counter.log (memoize ran once), got %d lines: %q",
			lines, string(data))
	}
}
