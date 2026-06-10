package dataentry

import (
	"net/http"
	"strings"
	"testing"
)

// TestRouterWalk_AllAPIRoutesReachHandlers drives every API route
// registered in NewRouter and registerAPIV1Routes through the
// production router. The handler-level suites pin endpoint behavior;
// this test pins that each route is actually registered and dispatches
// to a handler (TKT-TLQ94B) — a regression class the handler tests
// cannot see because they bypass the mux.
//
// Oracle: an unregistered /api path falls through to a ServeMux and is
// answered by the stdlib's plain-text "404 page not found" page. Every
// registered API handler answers with JSON (or at minimum a non-stdlib
// body), so a stdlib 404 means the route is gone. wantStatus pins the
// exact status where the fixture makes it deterministic; 0 means "any
// status, as long as a handler answered".
//
// The SSE routes (/api/events, /api/v1/_events) are excluded — their
// handlers block until context cancellation and have dedicated tests
// (TestNewRouterSSEEndpoint, TestHandleSSE*).
//
// When registering a new route, add a probe here — the registration
// sites in router.go and api_v1.go carry pointer comments.
func TestRouterWalk_AllAPIRoutesReachHandlers(t *testing.T) {
	routes := []struct {
		method     string
		path       string
		wantStatus int
	}{
		// NewRouter (router.go)
		{http.MethodGet, "/api/help/ticket", 0},
		{http.MethodPost, "/api/command/nonexistent", 0},
		{http.MethodPost, "/api/command-cancel/nonexistent", 0},
		{http.MethodPost, "/api/open-file", 0},
		{http.MethodGet, "/api/git/status", http.StatusOK},
		{http.MethodPost, "/api/git/sync", 0},

		// registerAPIV1Routes (api_v1.go) — system endpoints
		{http.MethodGet, "/api/v1/_schema", http.StatusOK},
		{http.MethodGet, "/api/v1/_schema/ticket", 0},
		{http.MethodGet, "/api/v1/_config", http.StatusOK},
		{http.MethodGet, "/api/v1/_search?q=ticket", 0},
		{http.MethodGet, "/api/v1/_position?type=ticket&id=TKT-001", 0},
		{http.MethodGet, "/api/v1/_analyze", 0},
		{http.MethodGet, "/api/v1/_git/status", http.StatusOK},
		{http.MethodPost, "/api/v1/_git/sync", 0},
		{http.MethodGet, "/api/v1/_settings", 0},
		{http.MethodGet, "/api/v1/_palette", 0},
		{http.MethodGet, "/api/v1/_theme/logo", 0},
		{http.MethodGet, "/api/v1/_theme/export", 0},
		{http.MethodPost, "/api/v1/_theme/import", 0},
		{http.MethodGet, "/api/v1/_sidepanel/ticket/TKT-001", 0},
		{http.MethodGet, "/api/v1/_sidebar", http.StatusOK},
		{http.MethodGet, "/api/v1/_conflicts", 0},
		{http.MethodGet, "/api/v1/_conflicts/some-id", 0},
		{http.MethodGet, "/api/v1/_documents/readme", 0},
		{http.MethodGet, "/api/v1/_openapi.json", http.StatusOK},
		{http.MethodGet, "/api/v1/_commands", http.StatusOK},
		{http.MethodGet, "/api/v1/_templates/ticket", 0},
		{http.MethodGet, "/api/v1/_views/ticket_detail?id=TKT-001", 0},
		{http.MethodPost, "/api/v1/_action/ticket/TKT-001/transition", 0},

		// Dynamic entity routes via the /api/v1/ catch-all
		{http.MethodGet, "/api/v1/tickets/", http.StatusOK},
		{http.MethodGet, "/api/v1/tickets/TKT-001", http.StatusOK},
		{http.MethodGet, "/api/v1/tickets/TKT-001/relations", 0},
	}

	for _, tc := range routes {
		t.Run(tc.method+" "+tc.path, func(t *testing.T) {
			app := newHandlerTestApp(t)
			w := doRequest(t, app, tc.method, tc.path, nil)

			if isStdlibNotFound(w.Code, w.Body.String()) {
				t.Fatalf("answered by the mux's stdlib 404 — route is not registered")
			}
			if tc.wantStatus != 0 && w.Code != tc.wantStatus {
				t.Errorf("status = %d, want %d (body: %.200s)", w.Code, tc.wantStatus, w.Body.String())
			}
		})
	}
}

// isStdlibNotFound reports whether a response is the stdlib ServeMux /
// http.NotFound page rather than a handler-produced error. Registered
// API handlers answer with JSON bodies, so this shape identifies "no
// route matched".
func isStdlibNotFound(code int, body string) bool {
	return code == http.StatusNotFound && strings.TrimSpace(body) == "404 page not found"
}
