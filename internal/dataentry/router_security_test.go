package dataentry

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// newSecuredTestApp returns an app with the security middlewares wired up
// against the loopback bind address used by httptest (127.0.0.1:8080).
func newSecuredTestApp(t *testing.T) *App {
	t.Helper()
	app := newHandlerTestApp(t)
	app.broker = newEventBroker()
	if err := app.SetSecurityConfig(SecurityConfig{
		BindAddress: "127.0.0.1:8080",
	}); err != nil {
		t.Fatalf("SetSecurityConfig: %v", err)
	}
	return app
}

func sendSameOrigin(t *testing.T, h http.Handler, method, path string) *httptest.ResponseRecorder {
	t.Helper()
	r := httptest.NewRequest(method, path, http.NoBody)
	r.Host = "127.0.0.1:8080"
	r.Header.Set("Origin", "http://127.0.0.1:8080")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	return w
}

func TestSecuredRouter_RejectsCrossOriginAPI(t *testing.T) {
	app := newSecuredTestApp(t)
	h := app.NewRouter()

	r := httptest.NewRequest(http.MethodGet, "/api/v1/_config", http.NoBody)
	r.Host = "127.0.0.1:8080"
	r.Header.Set("Origin", "https://evil.example")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)

	if w.Code != http.StatusForbidden {
		t.Fatalf("cross-origin API call should be 403, got %d (body=%s)", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "origin_not_allowed") {
		t.Fatalf("expected origin_not_allowed, got body=%q", w.Body.String())
	}
}

func TestSecuredRouter_RejectsHostSpoof(t *testing.T) {
	app := newSecuredTestApp(t)
	h := app.NewRouter()

	r := httptest.NewRequest(http.MethodGet, "/api/v1/_config", http.NoBody)
	r.Host = "evil.example"
	r.Header.Set("Origin", "http://127.0.0.1:8080") // valid origin still rejected
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)

	if w.Code != http.StatusForbidden {
		t.Fatalf("spoofed host should be 403, got %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), "host_not_allowed") {
		t.Fatalf("expected host_not_allowed, got body=%q", w.Body.String())
	}
}

func TestSecuredRouter_AllowsSameOriginAPI(t *testing.T) {
	app := newSecuredTestApp(t)
	h := app.NewRouter()

	w := sendSameOrigin(t, h, http.MethodGet, "/api/v1/_config")
	if w.Code != http.StatusOK {
		t.Fatalf("same-origin API call should be 200, got %d (body=%s)", w.Code, w.Body.String())
	}
}

func TestSecuredRouter_StaticFilesBypassOriginCheck(t *testing.T) {
	// Static assets need to remain fetchable cross-origin (they leak nothing
	// sensitive) so the SPA can still load resources via asset URLs that may
	// pass through proxies or absolute origins.
	app := newSecuredTestApp(t)
	h := app.NewRouter()

	r := httptest.NewRequest(http.MethodGet, "/static/favicon.svg", http.NoBody)
	r.Host = "127.0.0.1:8080"
	r.Header.Set("Origin", "https://example.com") // cross-origin
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("static asset should bypass origin check, got %d", w.Code)
	}
}

func TestSecuredRouter_RejectsCrossOriginCommandGET(t *testing.T) {
	// Critical regression test: previously /api/command/X accepted GET, so
	// `<img src=/api/command/X>` from any tab triggered RCE.
	app := newSecuredTestApp(t)
	h := app.NewRouter()

	r := httptest.NewRequest(http.MethodGet, "/api/command/anything", http.NoBody)
	r.Host = "127.0.0.1:8080"
	r.Header.Set("Origin", "https://evil.example")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)

	if w.Code != http.StatusForbidden {
		t.Fatalf("cross-origin GET to /api/command should be 403, got %d", w.Code)
	}
}

func TestSecuredRouter_AllowsSameOriginSSE(t *testing.T) {
	// Regression: removing the CORS reflection on SSE must not break the
	// SPA's own subscription. A same-origin EventSource-style GET should
	// pass through requireSameOrigin and reach the handler.
	app := newSecuredTestApp(t)
	h := app.NewRouter()

	r := httptest.NewRequest(http.MethodGet, "/api/events", http.NoBody)
	r.Host = "127.0.0.1:8080"
	r.Header.Set("Origin", "http://127.0.0.1:8080")
	// Cancel the context quickly so the long-lived handler returns.
	ctx, cancel := context.WithCancel(r.Context())
	r = r.WithContext(ctx)
	w := &flusherRecorder{ResponseRecorder: httptest.NewRecorder()}

	done := make(chan struct{})
	go func() {
		h.ServeHTTP(w, r)
		close(done)
	}()
	time.Sleep(20 * time.Millisecond)
	cancel()
	<-done

	if w.Code != http.StatusOK {
		t.Fatalf("same-origin SSE should be 200, got %d (body=%s)", w.Code, w.Body.String())
	}
	if ct := w.Header().Get("Content-Type"); ct != "text/event-stream" {
		t.Errorf("expected text/event-stream, got %q", ct)
	}
}

func TestSecuredRouter_BlockedResponseFormat(t *testing.T) {
	app := newSecuredTestApp(t)
	h := app.NewRouter()

	r := httptest.NewRequest(http.MethodPost, "/api/v1/tickets/", http.NoBody)
	r.Host = "127.0.0.1:8080"
	r.Header.Set("Origin", "https://evil.example")
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)

	if got := w.Header().Get("Content-Type"); got != "application/json" {
		t.Errorf("expected JSON content type, got %q", got)
	}
	body := w.Body.String()
	if !strings.Contains(body, `"error":"forbidden"`) {
		t.Errorf("body missing error field: %q", body)
	}
	if !strings.Contains(body, `"reason":"origin_not_allowed"`) {
		t.Errorf("body missing reason field: %q", body)
	}
}
