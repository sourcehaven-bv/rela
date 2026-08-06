package dataentry

import (
	"context"
	"maps"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/dataentryconfig"
	"github.com/Sourcehaven-BV/rela/internal/entity"
)

// withFakeDocRenderer swaps the app's document service for one backed by a
// fakeScriptEngine, so tests can assert not just the status code but whether
// the renderer ran at all. "The renderer never executed" is the property that
// matters on a deny path — a 404 alone would still pass if the expensive Lua
// aggregation had already run.
func withFakeDocRenderer(t *testing.T, app *App) *fakeScriptEngine {
	t.Helper()
	svc, fake := newTestService(t)
	fake.stdout = func(fakeScriptCall) string { return "# Rendered" }
	app.documents = svc
	return fake
}

// standaloneDocReq drives handleV1Documents through the ENTITY-LESS path
// shape, exercising the real routing split rather than calling the standalone
// handler directly.
func standaloneDocReq(ctx context.Context, docName string) *http.Request {
	return httptest.NewRequest(http.MethodGet,
		"/api/v1/_documents/"+docName, http.NoBody).WithContext(ctx)
}

// TestStandaloneDocument_Renders is the happy path (TKT-M1AX6P AC5): a
// document declared without an entity_type renders at /_documents/{name}.
func TestStandaloneDocument_Renders(t *testing.T) {
	app := newTestAppV1(t)
	fake := withFakeDocRenderer(t, app)
	app.State().Cfg.Documents = map[string]dataentryconfig.DocumentConfig{
		"sales_review": {Title: "Verkooprapportage", Script: "docs/sales.lua"},
	}

	rec := httptest.NewRecorder()
	app.handleV1Documents(rec, standaloneDocReq(t.Context(), "sales_review"))

	if rec.Code != http.StatusOK {
		t.Fatalf("got %d, want 200; body=%s", rec.Code, rec.Body)
	}
	if !strings.Contains(rec.Body.String(), "Rendered") {
		t.Errorf("expected rendered HTML in body, got: %s", rec.Body)
	}
	if fake.callCount() != 1 {
		t.Fatalf("expected exactly 1 render call, got %d", fake.callCount())
	}
	// The document identity reaches the script; the entry id stays absent.
	call := fake.calls[0]
	if call.documentID != "sales_review" {
		t.Errorf("documentID = %q, want sales_review", call.documentID)
	}
	if call.entryID != "" {
		t.Errorf("entryID = %q, want empty (a standalone document has no entry entity)", call.entryID)
	}
}

// TestStandaloneDocument_KindMismatch pins that each path shape refuses the
// other's document kind (AC6) rather than guessing a missing entry id or
// ignoring a supplied one.
func TestStandaloneDocument_KindMismatch(t *testing.T) {
	app := newTestAppV1(t)
	fake := withFakeDocRenderer(t, app)
	seedEntity(app, &entity.Entity{
		ID: "TKT-001", Type: "ticket", Properties: map[string]any{"title": "t"},
	})
	app.State().Cfg.Documents = map[string]dataentryconfig.DocumentConfig{
		"standalone": {Script: "docs/sales.lua"},
		"anchored":   {EntityType: "ticket", Script: "docs/ticket.lua"},
	}

	t.Run("entity-anchored doc requested without an entity id", func(t *testing.T) {
		rec := httptest.NewRecorder()
		app.handleV1Documents(rec, standaloneDocReq(t.Context(), "anchored"))

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("got %d, want 400; body=%s", rec.Code, rec.Body)
		}
		if !strings.Contains(rec.Body.String(), "entityId") {
			t.Errorf("error should point at the correct path shape, got: %s", rec.Body)
		}
	})

	t.Run("standalone doc requested with an entity id", func(t *testing.T) {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet,
			"/api/v1/_documents/standalone/TKT-001", http.NoBody).WithContext(t.Context())
		app.handleV1Documents(rec, req)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("got %d, want 400; body=%s", rec.Code, rec.Body)
		}
	})

	if fake.callCount() != 0 {
		t.Errorf("a kind mismatch must not invoke the renderer, got %d calls", fake.callCount())
	}
}

// TestStandaloneDocument_UnknownAndMalformed covers the negative paths.
func TestStandaloneDocument_UnknownAndMalformed(t *testing.T) {
	app := newTestAppV1(t)
	fake := withFakeDocRenderer(t, app)
	app.State().Cfg.Documents = map[string]dataentryconfig.DocumentConfig{
		"sales_review": {Script: "docs/sales.lua"},
	}

	cases := []struct {
		name string
		path string
		want int
	}{
		{"unknown document", "/api/v1/_documents/nope", http.StatusNotFound},
		{"empty document name", "/api/v1/_documents/", http.StatusBadRequest},
		{"traversal in document name", "/api/v1/_documents/..", http.StatusBadRequest},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, tc.path, http.NoBody).WithContext(t.Context())
			app.handleV1Documents(rec, req)
			if rec.Code != tc.want {
				t.Errorf("got %d, want %d; body=%s", rec.Code, tc.want, rec.Body)
			}
		})
	}

	t.Run("method not allowed", func(t *testing.T) {
		rec := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/api/v1/_documents/sales_review", http.NoBody)
		app.handleV1Documents(rec, req.WithContext(t.Context()))
		if rec.Code != http.StatusMethodNotAllowed {
			t.Errorf("got %d, want 405", rec.Code)
		}
	})

	if fake.callCount() != 0 {
		t.Errorf("no rejected request may invoke the renderer, got %d calls", fake.callCount())
	}
}

// TestStandaloneDocument_UngatedByDefault pins the deliberate default (AC9): a
// document without a `permission:` renders for any principal.
//
// This is safe because document content is bounded by the ACL-gated reader the
// render's Lua uses, not by this endpoint — a principal who cannot read the
// underlying entities gets an empty report either way. Requiring an explicit
// gate on every standalone document would be ceremony; this test is what keeps
// someone from "hardening" it into one.
func TestStandaloneDocument_UngatedByDefault(t *testing.T) {
	app := newTestAppV1(t)
	fake := withFakeDocRenderer(t, app)
	app.State().Cfg.Documents = map[string]dataentryconfig.DocumentConfig{
		"sales_review": {Script: "docs/sales.lua"},
	}

	d := mustNewACL(t, &acl.Policy{
		Roles:       map[string]acl.RoleDef{"viewer": {Read: []string{"ticket"}}},
		Assignments: map[string]string{"alice": "viewer"},
	}, app.store)
	app.acl = d

	// bob holds no roles and no permissions at all.
	rec := httptest.NewRecorder()
	app.handleV1Documents(rec, standaloneDocReq(gateCtxFor(principalCtx("bob"), t, d), "sales_review"))

	if rec.Code != http.StatusOK {
		t.Fatalf("an ungated document must render for any principal: got %d; body=%s", rec.Code, rec.Body)
	}
	if fake.callCount() != 1 {
		t.Errorf("expected the render to run, got %d calls", fake.callCount())
	}
}

// TestStandaloneDocument_PermissionGate pins AC8: a document declaring a
// `permission:` renders only for a holder. The deny is a 404 identical to an
// unknown document (so names stay non-enumerable) and fires BEFORE the
// renderer.
func TestStandaloneDocument_PermissionGate(t *testing.T) {
	app := newTestAppV1(t)
	fake := withFakeDocRenderer(t, app)
	app.State().Cfg.Documents = map[string]dataentryconfig.DocumentConfig{
		"sales_review": {Script: "docs/sales.lua", Permission: "report:sales"},
	}

	d := mustNewACL(t, &acl.Policy{
		Roles: map[string]acl.RoleDef{
			"directie": {Read: []string{"ticket"}, Permissions: []string{"report:sales"}},
			"viewer":   {Read: []string{"ticket"}},
		},
		Assignments: map[string]string{"alice": "directie", "bob": "viewer"},
	}, app.store)
	app.acl = d

	// Denied: bob's role grants no report:sales.
	rec := httptest.NewRecorder()
	app.handleV1Documents(rec, standaloneDocReq(gateCtxFor(principalCtx("bob"), t, d), "sales_review"))

	if rec.Code != http.StatusNotFound {
		t.Fatalf("bob (no permission): got %d, want 404; body=%s", rec.Code, rec.Body)
	}
	if fake.callCount() != 0 {
		t.Errorf("LEAK: denied request invoked the renderer (%d calls)", fake.callCount())
	}

	// The deny must be indistinguishable from an unknown document, or the
	// difference tells an unauthorized caller which documents exist. Compared
	// field-by-field except `instance`, which echoes the request path the
	// caller already supplied and so carries no information back to them.
	unknownRec := httptest.NewRecorder()
	app.handleV1Documents(unknownRec,
		standaloneDocReq(gateCtxFor(principalCtx("bob"), t, d), "no_such_document"))

	deny, unknown := decodeProblem(t, rec), decodeProblem(t, unknownRec)
	delete(deny, "instance")
	delete(unknown, "instance")
	if !maps.Equal(deny, unknown) {
		t.Errorf("deny response distinguishable from unknown-document response:\n deny=%v\n unknown=%v",
			deny, unknown)
	}
	if unknownRec.Code != rec.Code {
		t.Errorf("deny status %d != unknown-document status %d", rec.Code, unknownRec.Code)
	}

	// Permitted: alice's role grants report:sales.
	rec = httptest.NewRecorder()
	app.handleV1Documents(rec, standaloneDocReq(gateCtxFor(aliceCtx(), t, d), "sales_review"))
	if rec.Code != http.StatusOK {
		t.Fatalf("alice (holds report:sales): got %d, want 200; body=%s", rec.Code, rec.Body)
	}
	if fake.callCount() != 1 {
		t.Errorf("expected exactly 1 render for the permitted principal, got %d", fake.callCount())
	}
}

// TestSidebar_HidesGatedDocument pins AC10's ACL half: a document entry the
// principal cannot render is omitted from the sidebar entirely, and a group
// left empty by that filtering is dropped rather than rendered as a bare
// heading.
//
// The endpoint re-checks the same permission (TestStandaloneDocument_PermissionGate),
// so this is a UX affordance, not the boundary — asserted here so nobody
// "simplifies" the sidebar into the authorization decision.
func TestSidebar_HidesGatedDocument(t *testing.T) {
	app := newTestAppV1(t)
	app.State().Cfg.Documents = map[string]dataentryconfig.DocumentConfig{
		"sales_review": {Script: "docs/sales.lua", Permission: "report:sales"},
		"open_report":  {Script: "docs/open.lua"},
	}
	app.Cfg().Navigation = []dataentryconfig.NavigationEntry{
		{Group: "Reports", Items: []dataentryconfig.NavigationEntry{
			{Label: "Verkooprapportage", Document: "sales_review"},
		}},
		{Label: "Open Report", Document: "open_report"},
	}

	d := mustNewACL(t, &acl.Policy{
		Roles: map[string]acl.RoleDef{
			"directie": {Read: []string{"ticket"}, Permissions: []string{"report:sales"}},
			"viewer":   {Read: []string{"ticket"}},
		},
		Assignments: map[string]string{"alice": "directie", "bob": "viewer"},
	}, app.store)
	app.acl = d

	hrefsFor := func(t *testing.T, ctx context.Context) []string {
		t.Helper()
		req := httptest.NewRequest(http.MethodGet, "/api/v1/_sidebar", http.NoBody)
		rec := httptest.NewRecorder()
		app.views.handleV1Sidebar(rec, req.WithContext(gateCtxFor(ctx, t, d)))
		if rec.Code != http.StatusOK {
			t.Fatalf("sidebar: got %d, want 200; body=%s", rec.Code, rec.Body)
		}
		var resp struct {
			Navigation []struct {
				Group string `json:"group"`
				Items []struct {
					Href string `json:"href"`
				} `json:"items"`
			} `json:"navigation"`
		}
		if err := decodeJSON(rec.Body, &resp); err != nil {
			t.Fatalf("decode sidebar: %v", err)
		}
		var hrefs []string
		for _, g := range resp.Navigation {
			if len(g.Items) == 0 {
				t.Errorf("group %q rendered with no items; empty groups must be dropped", g.Group)
			}
			for _, it := range g.Items {
				hrefs = append(hrefs, it.Href)
			}
		}
		return hrefs
	}

	alice := hrefsFor(t, aliceCtx())
	if !slices.Contains(alice, "/document/sales_review") {
		t.Errorf("alice holds report:sales but the entry is missing: %v", alice)
	}
	if !slices.Contains(alice, "/document/open_report") {
		t.Errorf("ungated document missing for alice: %v", alice)
	}

	bob := hrefsFor(t, principalCtx("bob"))
	if slices.Contains(bob, "/document/sales_review") {
		t.Errorf("bob lacks report:sales but the entry is present: %v", bob)
	}
	if !slices.Contains(bob, "/document/open_report") {
		t.Errorf("ungated document must stay visible to bob: %v", bob)
	}
}

// decodeProblem decodes an RFC-7807 error body into a comparable map.
func decodeProblem(t *testing.T, rec *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var m map[string]any
	if err := decodeJSON(rec.Body, &m); err != nil {
		t.Fatalf("decode problem body: %v", err)
	}
	return m
}
