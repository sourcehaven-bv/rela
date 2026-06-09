package dataentry

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestNewRouterRegistersAPIRoutes(t *testing.T) {
	app := newHandlerTestApp(t)
	app.broker = newEventBroker()

	handler := app.NewRouter()

	// Test API routes (SPA routes depend on embedded frontend build)
	tests := []struct {
		path       string
		wantStatus int
	}{
		{"/api/v1/_schema", http.StatusOK},
		{"/api/v1/_config", http.StatusOK},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			r := httptest.NewRequest(http.MethodGet, tt.path, http.NoBody)
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, r)
			if w.Code != tt.wantStatus {
				t.Errorf("GET %s: expected %d, got %d", tt.path, tt.wantStatus, w.Code)
			}
		})
	}
}

func TestNewRouterStaticFiles(t *testing.T) {
	app := newHandlerTestApp(t)
	app.broker = newEventBroker()

	handler := app.NewRouter()

	// Request a known embedded static file (favicon is the only static file now)
	r := httptest.NewRequest(http.MethodGet, "/static/favicon.svg", http.NoBody)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200 for static file, got %d", w.Code)
	}
}

func TestNewRouterStaticFilesNoCacheHeader(t *testing.T) {
	app := newHandlerTestApp(t)
	app.broker = newEventBroker()
	handler := app.NewRouter()

	// Static files should NOT have no-cache header (they bypass the middleware)
	r := httptest.NewRequest(http.MethodGet, "/static/favicon.svg", http.NoBody)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)

	if cc := w.Header().Get("Cache-Control"); cc != "" {
		t.Errorf("static files should not have Cache-Control header, got %q", cc)
	}
}

func TestNewRouterAPIHasNoCacheHeader(t *testing.T) {
	app := newHandlerTestApp(t)
	app.broker = newEventBroker()
	handler := app.NewRouter()

	// API routes should have no-cache header
	r := httptest.NewRequest(http.MethodGet, "/api/v1/_schema", http.NoBody)
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, r)

	cc := w.Header().Get("Cache-Control")
	if cc != "no-cache, no-store, must-revalidate" {
		t.Errorf("expected Cache-Control header on API routes, got %q", cc)
	}
}

func TestNewRouterSSEEndpoint(t *testing.T) {
	app := newHandlerTestApp(t)
	app.broker = newEventBroker()

	handler := app.NewRouter()

	// SSE endpoint should respond with event-stream content type
	ctx, cancel := context.WithCancel(context.Background())
	r := httptest.NewRequest(http.MethodGet, "/api/events", http.NoBody).WithContext(ctx)
	w := newFlusherRecorder()

	done := make(chan struct{})
	go func() {
		handler.ServeHTTP(w, r)
		close(done)
	}()

	// Wait for the initial keepalive flush, then end the long-lived handler.
	w.awaitFlush(t)
	cancel()
	<-done

	if ct := w.Header().Get("Content-Type"); ct != "text/event-stream" {
		t.Errorf("expected Content-Type text/event-stream, got %q", ct)
	}
}
