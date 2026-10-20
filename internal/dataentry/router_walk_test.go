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
// The SSE, CalDAV and MCP routes are listed but skipped (see skipWalk).
//
// POST /webhooks/idp is also excluded, but for a different reason: it is NOT an
// /api/ path, so this test's oracle (unregistered → stdlib 404) cannot see it —
// an unregistered non-/api/ path falls through to the SPA catch-all and returns
// 200 HTML, which reads as "reachable" here. That exact blind spot let BUG-F3ADZO
// ship. Its reachability is pinned by TestWebhook_ReachableThroughRouter, whose
// oracle is "not the SPA shell" instead.
//
// When registering a new route, add a probe here — the registration
// sites in router.go and api_v1.go carry face comments.
//
// CONSTRAINT: every probe must hit a path its handler actually serves
// for the fixture (or answers with a JSON error). Do NOT add probes
// that legitimately resolve to a handler-emitted http.NotFound — e.g.
// /api/help/<unknown-type> or an unregistered _-prefixed path — the
// oracle cannot distinguish those from an unregistered route and the
// test would fail with a misleading message.
func TestRouterWalk_AllAPIRoutesReachHandlers(t *testing.T) {
	for _, tc := range routerWalkProbes {
		t.Run(tc.method+" "+tc.path, func(t *testing.T) {
			if tc.skipWalk != "" {
				t.Skip(tc.skipWalk)
			}
			app := newHandlerTestApp(t)
			w := doRequest(t, app, tc.method, tc.path)

			if isStdlibNotFound(w.Code, w.Body.String()) {
				t.Fatalf("answered by the mux's stdlib 404 — route is not registered")
			}
			if tc.wantStatus != 0 && w.Code != tc.wantStatus {
				t.Errorf("status = %d, want %d (body: %.200s)", w.Code, tc.wantStatus, w.Body.String())
			}
		})
	}
}

// routerWalkProbe is one route probe of [TestRouterWalk_AllAPIRoutesReachHandlers].
type routerWalkProbe struct {
	method string
	// route is the pattern the handler serves, with {name} for a variable
	// segment and {name...} for the rest of the path. The golden corpus
	// keys its coverage on method plus route.
	route string
	// path is the concrete request the walk sends.
	path       string
	wantStatus int
	// skipWalk, when set, says why the walk cannot probe this route; the
	// route still counts for the golden corpus coverage guard.
	skipWalk string
}

// Reasons a route is listed but not walked.
const (
	skipSSE = "the SSE handler blocks until the client goes away; " +
		"TestNewRouterSSEEndpoint and TestHandleSSE* cover it"
	skipCalDAV = "the walk fixture wires no CalDAV aliases, so the route is not mounted; " +
		"TestCalDAV_RouteIsReachable covers it"
	skipAttachmentRead = "a missing attachment answers http.NotFound, which the oracle " +
		"cannot tell from an unregistered route; the POST probe covers the registration"
	skipMCP = "the walk fixture has no JWT gate, so the remote MCP route is not mounted; " +
		"TestMCPActions_ThroughTheRemoteMount and the golden corpus cover it"
)

// routerWalkProbes lists a probe for each route pattern the API serves. The
// golden corpus takes its route patterns from here: its coverage guard
// (TestGoldenParity_CoversEveryMethod) sends every method to each pattern and
// requires a corpus case or a reasoned exemption for each one that answers.
var routerWalkProbes = []routerWalkProbe{
	// NewRouter (router.go)
	{http.MethodGet, "/api/help/{type}", "/api/help/ticket", 0, ""},
	{http.MethodPost, "/api/command/{name}", "/api/command/nonexistent", 0, ""},
	{http.MethodPost, "/api/command-cancel/{run}", "/api/command-cancel/nonexistent", 0, ""},
	{http.MethodGet, "/api/command-file/{file}", "/api/command-file/nonexistent", 0, ""},
	// git probes are any-status: their codes depend on whether the
	// fixture wires gitOps, which is not this test's concern.
	{http.MethodGet, "/api/git/status", "/api/git/status", 0, ""},
	{http.MethodPost, "/api/git/sync", "/api/git/sync", 0, ""},
	{http.MethodGet, "/api/events", "/api/events", 0, skipSSE},
	{http.MethodGet, "/api/v1/_events", "/api/v1/_events", 0, skipSSE},

	// registerAPIV1Routes (api_v1.go) — system endpoints
	{http.MethodGet, "/api/v1/_schema", "/api/v1/_schema", http.StatusOK, ""},
	{http.MethodGet, "/api/v1/_schema/types", "/api/v1/_schema/types", http.StatusOK, ""},
	{http.MethodGet, "/api/v1/_schema/types/{type}", "/api/v1/_schema/types/ticket", http.StatusOK, ""},
	{http.MethodGet, "/api/v1/_schema/relations", "/api/v1/_schema/relations", http.StatusOK, ""},
	{http.MethodGet, "/api/v1/_schema/channels", "/api/v1/_schema/channels", http.StatusOK, ""},
	{http.MethodGet, "/api/v1/_config", "/api/v1/_config", http.StatusOK, ""},
	{http.MethodGet, "/api/v1/_me", "/api/v1/_me", http.StatusOK, ""},
	{http.MethodGet, "/api/v1/_configure", "/api/v1/_configure", http.StatusNotFound, ""},
	{http.MethodGet, "/api/v1/_configure/{key}", "/api/v1/_configure/x", http.StatusNotFound, ""},
	// registered; unknown feed → 404
	{http.MethodGet, "/api/v1/_feeds/{feed}", "/api/v1/_feeds/nope.ics", http.StatusNotFound, ""},
	{http.MethodGet, "/api/v1/_search", "/api/v1/_search?q=ticket", 0, ""},
	{http.MethodGet, "/api/v1/_position", "/api/v1/_position?type=ticket&id=TKT-001", 0, ""},
	{http.MethodGet, "/api/v1/_analyze", "/api/v1/_analyze", 0, ""},
	// no sources configured → empty suggestion
	{http.MethodGet, "/api/v1/_next_action", "/api/v1/_next_action", http.StatusOK, ""},
	{http.MethodGet, "/api/v1/_git/status", "/api/v1/_git/status", 0, ""},
	{http.MethodPost, "/api/v1/_git/sync", "/api/v1/_git/sync", 0, ""},
	{http.MethodGet, "/api/v1/_settings", "/api/v1/_settings", 0, ""},
	{http.MethodGet, "/api/v1/_palette", "/api/v1/_palette", 0, ""},
	{http.MethodGet, "/api/v1/_theme/logo", "/api/v1/_theme/logo", 0, ""},
	{http.MethodGet, "/api/v1/_theme/export", "/api/v1/_theme/export", 0, ""},
	{http.MethodPost, "/api/v1/_theme/import", "/api/v1/_theme/import", 0, ""},
	{http.MethodGet, "/api/v1/_sidepanel/{form}/{id}", "/api/v1/_sidepanel/ticket/TKT-001", 0, ""},
	{http.MethodGet, "/api/v1/_sidebar", "/api/v1/_sidebar", http.StatusOK, ""},
	{http.MethodGet, "/api/v1/_nav_status", "/api/v1/_nav_status", http.StatusOK, ""},
	{http.MethodGet, "/api/v1/_nav_items", "/api/v1/_nav_items", http.StatusOK, ""},
	{http.MethodGet, "/api/v1/_dashboard", "/api/v1/_dashboard", http.StatusOK, ""},
	// Comments: the fixture declares no `comments:` block, so the handler
	// answers 404 itself — a JSON error from a registered handler, not the
	// stdlib's unregistered-route 404.
	{http.MethodGet, "/api/v1/_comments/{type}/{id}", "/api/v1/_comments/ticket/TKT-001", http.StatusNotFound, ""},
	{http.MethodPost, "/api/v1/_comments/{type}/{id}", "/api/v1/_comments/ticket/TKT-001", http.StatusNotFound, ""},
	{http.MethodPatch, "/api/v1/_comments/{type}/{id}/{comment}",
		"/api/v1/_comments/ticket/TKT-001/c1", http.StatusNotFound, ""},
	{http.MethodPost, "/api/v1/_comments/{type}/{id}/{comment}/accept",
		"/api/v1/_comments/ticket/TKT-001/c1/accept", http.StatusNotFound, ""},
	// Piles: no service is wired in the fixture, so the handler answers
	// its own JSON 404.
	{http.MethodGet, "/api/v1/_piles", "/api/v1/_piles", http.StatusNotFound, ""},
	{http.MethodGet, "/api/v1/_piles/{id}", "/api/v1/_piles/PIL-ABCD", http.StatusNotFound, ""},
	{http.MethodPost, "/api/v1/_piles/{id}/items", "/api/v1/_piles/PIL-ABCD/items", http.StatusNotFound, ""},
	{http.MethodPost, "/api/v1/_piles/{id}/items/_remove", "/api/v1/_piles/PIL-ABCD/items/_remove",
		http.StatusNotFound, ""},
	{http.MethodGet, "/api/v1/_piles/{id}/_export", "/api/v1/_piles/PIL-ABCD/_export", http.StatusNotFound, ""},
	// Leases: no service is wired in the fixture, so the handler answers
	// its own JSON 404.
	{http.MethodGet, "/api/v1/_leases", "/api/v1/_leases", http.StatusNotFound, ""},
	{http.MethodPost, "/api/v1/_leases/renew", "/api/v1/_leases/renew", http.StatusNotFound, ""},
	{http.MethodPost, "/api/v1/_leases/release", "/api/v1/_leases/release", http.StatusNotFound, ""},
	// Live-view subscriptions: an unknown stream id is the handler's own
	// JSON 404.
	{http.MethodPut, "/api/v1/_events/{stream}/subscriptions", "/api/v1/_events/nope/subscriptions",
		http.StatusNotFound, ""},
	{http.MethodGet, "/api/v1/_conflicts", "/api/v1/_conflicts", 0, ""},
	{http.MethodGet, "/api/v1/_conflicts/{path...}", "/api/v1/_conflicts/some-id", 0, ""},
	{http.MethodPost, "/api/v1/_conflicts/resolve", "/api/v1/_conflicts/resolve", http.StatusBadRequest, ""},
	{http.MethodGet, "/api/v1/_documents/{doc}", "/api/v1/_documents/readme", 0, ""},
	{http.MethodGet, "/api/v1/_documents/{doc}/_export", "/api/v1/_documents/readme/_export?transform=copy", 0, ""},
	{http.MethodGet, "/api/v1/_history/{type}/{id}", "/api/v1/_history/ticket/TKT-001", 0, ""},
	{http.MethodGet, "/api/v1/_history/{type}/{id}/{version}", "/api/v1/_history/ticket/TKT-001/1", 0, ""},
	{http.MethodPost, "/api/v1/_history/{type}/{id}/{version}/restore",
		"/api/v1/_history/ticket/TKT-001/1/restore", 0, ""},
	{http.MethodGet, "/api/v1/_relation_history/{type}/{from}/{relation}/{to}",
		"/api/v1/_relation_history/ticket/TKT-001/implements/FEAT-001", 0, ""},
	{http.MethodGet, "/api/v1/_relation_history/{type}/{from}/{relation}/{to}/{version}",
		"/api/v1/_relation_history/ticket/TKT-001/implements/FEAT-001/1", 0, ""},
	{http.MethodGet, "/api/v1/_openapi.json", "/api/v1/_openapi.json", http.StatusOK, ""},
	{http.MethodGet, "/api/v1/_commands", "/api/v1/_commands", http.StatusOK, ""},
	{http.MethodGet, "/api/v1/_transforms", "/api/v1/_transforms", 0, ""},
	{http.MethodGet, "/api/v1/_templates/{type}", "/api/v1/_templates/ticket", 0, ""},
	{http.MethodGet, "/api/v1/_views/{type}/{id}", "/api/v1/_views/ticket/TKT-001", 0, ""},
	{http.MethodGet, "/api/v1/_gantts/{gantt}", "/api/v1/_gantts/plan", 0, ""},
	{http.MethodGet, "/api/v1/_copies", "/api/v1/_copies", 0, ""},
	{http.MethodPost, "/api/v1/_copies/{name}", "/api/v1/_copies/publish", 0, ""},
	{http.MethodPost, "/api/v1/_action/{name}", "/api/v1/_action/close", 0, ""},
	{http.MethodGet, "/api/v1/_apps/{app}/{path...}", "/api/v1/_apps/some-app/", 0, ""},
	{"REPORT", "/api/v1/_caldav/{path...}", "/api/v1/_caldav/principal/calendars/tickets/", 0, skipCalDAV},
	{http.MethodPost, "/api/v1/_mcp", "/api/v1/_mcp", 0, skipMCP},

	// Dynamic entity routes via the /api/v1/ catch-all
	{http.MethodGet, "/api/v1/{plural}/", "/api/v1/tickets/", http.StatusOK, ""},
	{http.MethodPost, "/api/v1/{plural}/", "/api/v1/tickets/", 0, ""},
	{http.MethodGet, "/api/v1/{plural}/_export", "/api/v1/tickets/_export?transform=copy", 0, ""},
	{http.MethodGet, "/api/v1/{plural}/{id}", "/api/v1/tickets/TKT-001", http.StatusOK, ""},
	{http.MethodPatch, "/api/v1/{plural}/{id}", "/api/v1/tickets/TKT-001", 0, ""},
	{http.MethodDelete, "/api/v1/{plural}/{id}", "/api/v1/tickets/TKT-001", 0, ""},
	{http.MethodGet, "/api/v1/{plural}/{id}/relations", "/api/v1/tickets/TKT-001/relations", 0, ""},
	{http.MethodGet, "/api/v1/{plural}/{id}/relations/{type}", "/api/v1/tickets/TKT-001/relations/implements", 0, ""},
	{http.MethodPost, "/api/v1/{plural}/{id}/relations/{type}", "/api/v1/tickets/TKT-001/relations/implements", 0, ""},
	{http.MethodGet, "/api/v1/{plural}/{id}/relations/{type}/{to}",
		"/api/v1/tickets/TKT-001/relations/implements/FEAT-001", 0, ""},
	{http.MethodDelete, "/api/v1/{plural}/{id}/relations/{type}/{to}",
		"/api/v1/tickets/TKT-001/relations/implements/FEAT-001", 0, ""},
	{http.MethodGet, "/api/v1/{plural}/{id}/_export", "/api/v1/tickets/TKT-001/_export?transform=copy", 0, ""},
	{http.MethodPost, "/api/v1/{plural}/{id}/restore", "/api/v1/tickets/TKT-001/restore", 0, ""},
	{http.MethodGet, "/api/v1/{plural}/{id}/mentioned-by", "/api/v1/tickets/TKT-001/mentioned-by", 0, ""},
	// A wrong method is the claim route's own 405, so a dropped dispatcher
	// case shows as "Resource not found" instead.
	{http.MethodGet, "/api/v1/{plural}/{id}/_lease", "/api/v1/tickets/TKT-001/_lease",
		http.StatusMethodNotAllowed, ""},
	{http.MethodPost, "/api/v1/{plural}/{id}/_lease", "/api/v1/tickets/TKT-001/_lease", http.StatusNotFound, ""},
	// Presence: the route's own 405 on a wrong method, and 200 on GET, so a
	// dropped dispatcher case shows as "Resource not found".
	{http.MethodPost, "/api/v1/{plural}/{id}/_presence", "/api/v1/tickets/TKT-001/_presence",
		http.StatusMethodNotAllowed, ""},
	{http.MethodGet, "/api/v1/{plural}/{id}/_presence", "/api/v1/tickets/TKT-001/_presence", http.StatusOK, ""},
	{http.MethodPost, "/api/v1/{plural}/{id}/_actions/{action}", "/api/v1/tickets/TKT-001/_actions/clone", 0, ""},
	// Attachment upload route. Its GET answers 405.
	{http.MethodPost, "/api/v1/{plural}/{id}/_attachments/{property}",
		"/api/v1/tickets/TKT-001/_attachments/screenshot", 0, ""},
	{http.MethodGet, "/api/v1/{plural}/{id}/_attachments/{property}/{file}",
		"/api/v1/tickets/TKT-001/_attachments/screenshot/shot.txt", 0, skipAttachmentRead},
}

// isStdlibNotFound reports whether a response is the stdlib ServeMux /
// http.NotFound page rather than a handler-produced error.
//
// Caveat: some registered handlers ALSO emit http.NotFound for unknown
// sub-resources (handleEntityHelp for an unknown type, the /api/v1/
// catch-all for unregistered _-paths). The probe table above therefore
// only contains paths the fixture serves — see the CONSTRAINT comment
// on the test. Within that constraint, this shape identifies "no route
// matched".
func isStdlibNotFound(code int, body string) bool {
	return code == http.StatusNotFound && strings.TrimSpace(body) == "404 page not found"
}
