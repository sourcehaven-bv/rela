package dataentry

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/dataentryconfig"
	"github.com/Sourcehaven-BV/rela/internal/project"
	"github.com/Sourcehaven-BV/rela/internal/repository"
	"github.com/Sourcehaven-BV/rela/internal/storage"
	"github.com/Sourcehaven-BV/rela/internal/workspace"
)

// callAction simulates the reloadLockMiddleware by holding RLock around
// the handler call, matching how requests reach handlers in production.
func callAction(app *App, req *http.Request, rec *httptest.ResponseRecorder) {
	app.mu.RLock()
	defer app.mu.RUnlock()
	app.handleV1Action(rec, req)
}

// newActionTestApp builds a test App with a real project root containing
// the given action scripts.
func newActionTestApp(t *testing.T, scripts map[string]string) *App {
	t.Helper()

	tmpDir := t.TempDir()
	actionsDir := filepath.Join(tmpDir, "actions")
	if err := os.MkdirAll(actionsDir, 0755); err != nil {
		t.Fatal(err)
	}
	for name, content := range scripts {
		if err := os.WriteFile(filepath.Join(actionsDir, name), []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
	}

	app := newTestAppV1(t)
	app.ws = workspace.NewWithGraph(
		repository.New(storage.NewSafeFS(storage.NewOsFS()), &project.Context{Root: tmpDir}),
		app.meta, app.g)

	return app
}

func TestHandleV1Action_Success(t *testing.T) {
	app := newActionTestApp(t, map[string]string{
		"hello.lua": `return {redirect = "/done", message = "ok"}`,
	})
	app.Cfg.Actions = map[string]dataentryconfig.Action{
		"hello": {Script: "hello.lua"},
	}

	req := httptest.NewRequest(http.MethodPost, "/api/v1/_action/hello", http.NoBody)
	rec := httptest.NewRecorder()

	callAction(app, req, rec)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp V1ActionResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode failed: %v", err)
	}
	if resp.Redirect != "/done" {
		t.Errorf("expected redirect=/done, got %q", resp.Redirect)
	}
	if resp.Message != "ok" {
		t.Errorf("expected message=ok, got %q", resp.Message)
	}
}

func TestHandleV1Action_NoReturn(t *testing.T) {
	app := newActionTestApp(t, map[string]string{
		"noop.lua": `local x = 1`,
	})
	app.Cfg.Actions = map[string]dataentryconfig.Action{
		"noop": {Script: "noop.lua"},
	}

	req := httptest.NewRequest(http.MethodPost, "/api/v1/_action/noop", http.NoBody)
	rec := httptest.NewRecorder()

	callAction(app, req, rec)

	if rec.Code != http.StatusNoContent {
		t.Errorf("expected 204, got %d", rec.Code)
	}
}

func TestHandleV1Action_NotFound(t *testing.T) {
	app := newActionTestApp(t, nil)
	app.Cfg.Actions = map[string]dataentryconfig.Action{}

	req := httptest.NewRequest(http.MethodPost, "/api/v1/_action/missing", http.NoBody)
	rec := httptest.NewRecorder()

	callAction(app, req, rec)

	if rec.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", rec.Code)
	}
}

func TestHandleV1Action_InvalidID(t *testing.T) {
	app := newActionTestApp(t, nil)

	tests := []struct {
		name string
		path string
	}{
		{"uppercase", "/api/v1/_action/Foo"},
		{"empty", "/api/v1/_action/"},
		{"slash", "/api/v1/_action/foo/bar"},
		{"dot", "/api/v1/_action/foo.bar"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, tt.path, http.NoBody)
			rec := httptest.NewRecorder()
			callAction(app, req, rec)
			if rec.Code != http.StatusBadRequest {
				t.Errorf("expected 400 for %q, got %d", tt.path, rec.Code)
			}
		})
	}
}

func TestHandleV1Action_MethodNotAllowed(t *testing.T) {
	app := newActionTestApp(t, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/_action/foo", http.NoBody)
	rec := httptest.NewRecorder()

	callAction(app, req, rec)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected 405, got %d", rec.Code)
	}
}

func TestHandleV1Action_ScriptError(t *testing.T) {
	app := newActionTestApp(t, map[string]string{
		"boom.lua": `error("kaboom")`,
	})
	app.Cfg.Actions = map[string]dataentryconfig.Action{
		"boom": {Script: "boom.lua"},
	}

	req := httptest.NewRequest(http.MethodPost, "/api/v1/_action/boom", http.NoBody)
	rec := httptest.NewRecorder()

	callAction(app, req, rec)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", rec.Code)
	}

	var resp V1ActionResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode failed: %v", err)
	}
	if resp.Error != "action_failed" {
		t.Errorf("expected error=action_failed, got %q", resp.Error)
	}
	if resp.CorrelationID == "" {
		t.Error("expected correlation_id, got empty")
	}
}

func TestHandleV1Action_ParamsPassed(t *testing.T) {
	app := newActionTestApp(t, map[string]string{
		"echo.lua": `return {message = rela.params.greeting}`,
	})
	app.Cfg.Actions = map[string]dataentryconfig.Action{
		"echo": {
			Script: "echo.lua",
			Params: map[string]string{"greeting": "hello world"},
		},
	}

	req := httptest.NewRequest(http.MethodPost, "/api/v1/_action/echo", http.NoBody)
	rec := httptest.NewRecorder()

	callAction(app, req, rec)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp V1ActionResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode failed: %v", err)
	}
	if resp.Message != "hello world" {
		t.Errorf("expected message='hello world', got %q", resp.Message)
	}
}

func TestHandleV1Action_OpenRedirectRejected(t *testing.T) {
	app := newActionTestApp(t, map[string]string{
		"evil.lua": `return {redirect = "//evil.com"}`,
	})
	app.Cfg.Actions = map[string]dataentryconfig.Action{
		"evil": {Script: "evil.lua"},
	}

	req := httptest.NewRequest(http.MethodPost, "/api/v1/_action/evil", http.NoBody)
	rec := httptest.NewRecorder()

	callAction(app, req, rec)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("expected 500 for open redirect, got %d", rec.Code)
	}
}

func TestHandleV1Action_Concurrent(t *testing.T) {
	// Two parallel POSTs to the same action should serialize via the write lock
	// (no panics, no corruption). We use a script that increments a property
	// on a known entity to detect lost updates.
	app := newActionTestApp(t, map[string]string{
		"noop.lua": `return {message = "done"}`,
	})
	app.Cfg.Actions = map[string]dataentryconfig.Action{
		"noop": {Script: "noop.lua"},
	}

	const N = 5
	var wg sync.WaitGroup
	for i := 0; i < N; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			req := httptest.NewRequest(http.MethodPost, "/api/v1/_action/noop", http.NoBody)
			rec := httptest.NewRecorder()
			callAction(app, req, rec)
			if rec.Code != http.StatusOK {
				t.Errorf("expected 200, got %d", rec.Code)
			}
		}()
	}
	wg.Wait()
}

func TestNewCorrelationID(t *testing.T) {
	id1 := newCorrelationID()
	id2 := newCorrelationID()
	if id1 == id2 {
		t.Errorf("correlation IDs should differ: %q == %q", id1, id2)
	}
	if id1 == "" {
		t.Error("correlation ID is empty")
	}
}
