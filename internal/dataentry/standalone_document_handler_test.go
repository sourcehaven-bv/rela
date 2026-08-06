package dataentry

import (
	"context"
	"maps"
	"net/http"
	"net/http/httptest"
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
//
// entities are seeded into the SERVICE's own store (newTestService builds a
// separate memstore from the App's). An entity-anchored render hashes its
// entry entity, so anything the test requests by id must be passed here as
// well as seeded into the app via seedEntity.
func withFakeDocRenderer(t *testing.T, app *App, entities ...*entity.Entity) *fakeScriptEngine {
	t.Helper()
	svc, fake := newTestService(t, entities...)
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
// `permission:` renders only for a holder, and the gate fires BEFORE the
// renderer (an unauthorized caller must not trigger an expensive aggregation).
//
// The deny is a 403 that NAMES the document and the required permission.
// Which documents exist is not a secret — they are keys in an operator's
// data-entry.yaml — so there is nothing to conceal and an actionable error
// beats a disguised 404. See "The configuration is not a secret; the data is"
// in the root CLAUDE.md.
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

	if rec.Code != http.StatusForbidden {
		t.Fatalf("bob (no permission): got %d, want 403; body=%s", rec.Code, rec.Body)
	}
	if fake.callCount() != 0 {
		t.Errorf("denied request invoked the renderer (%d calls)", fake.callCount())
	}

	// The 403 must be actionable: it names the document and the permission the
	// operator needs to grant. Deliberately NOT disguised as the unknown-doc
	// 404 — a config key is not a secret, and an opaque denial is
	// unsupportable at scale (the same reasoning acl.Decision records for
	// every deny).
	body := rec.Body.String()
	for _, want := range []string{"sales_review", "report:sales"} {
		if !strings.Contains(body, want) {
			t.Errorf("403 body should name %q so the operator can act on it, got: %s", want, body)
		}
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

// TestAnchoredDocument_PermissionGate covers `permission:` on an
// ENTITY-ANCHORED document — the other half of the gate, and the one where it
// composes with the per-entity read gate.
//
// The composition claim is "narrows, never widens": holding the permission
// must not grant access to an entity the principal cannot read.
func TestAnchoredDocument_PermissionGate(t *testing.T) {
	app := newTestAppV1(t)
	ticket := &entity.Entity{ID: "TKT-001", Type: "ticket", Properties: map[string]any{"title": "t"}}
	feature := &entity.Entity{ID: "FEA-001", Type: "feature", Properties: map[string]any{"title": "f"}}
	fake := withFakeDocRenderer(t, app, ticket, feature)
	seedEntity(app, ticket)
	seedEntity(app, feature)
	app.State().Cfg.Documents = map[string]dataentryconfig.DocumentConfig{
		"ticket_report": {EntityType: "ticket", Script: "docs/t.lua", Permission: "report:tickets"},
	}

	d := mustNewACL(t, &acl.Policy{
		Roles: map[string]acl.RoleDef{
			// alice: may read tickets AND holds the permission.
			"lead": {Read: []string{"ticket"}, Permissions: []string{"report:tickets"}},
			// bob: may read tickets but holds no permission.
			"viewer": {Read: []string{"ticket"}},
			// carol: holds the permission but may NOT read tickets.
			"auditor": {Read: []string{"feature"}, Permissions: []string{"report:tickets"}},
		},
		Assignments: map[string]string{
			"alice": "lead", "bob": "viewer", "carol": "auditor",
		},
	}, app.store)
	app.acl = d

	get := func(t *testing.T, ctx context.Context) *httptest.ResponseRecorder {
		t.Helper()
		rec := httptest.NewRecorder()
		app.handleV1Documents(rec, httptest.NewRequest(http.MethodGet,
			"/api/v1/_documents/ticket_report/TKT-001", http.NoBody).
			WithContext(gateCtxFor(ctx, t, d)))
		return rec
	}

	t.Run("holds permission and may read the entity", func(t *testing.T) {
		if rec := get(t, aliceCtx()); rec.Code != http.StatusOK {
			t.Fatalf("got %d, want 200; body=%s", rec.Code, rec.Body)
		}
	})

	t.Run("may read the entity but lacks the permission", func(t *testing.T) {
		before := fake.callCount()
		rec := get(t, principalCtx("bob"))
		// 403: the missing capability is a config key, safe to name.
		if rec.Code != http.StatusForbidden {
			t.Fatalf("got %d, want 403; body=%s", rec.Code, rec.Body)
		}
		if fake.callCount() != before {
			t.Errorf("denied request invoked the renderer")
		}
	})

	t.Run("holds the permission but may not read the entity", func(t *testing.T) {
		before := fake.callCount()
		rec := get(t, principalCtx("carol"))
		// 404, NOT 403: the entity gate fires first and whether TKT-001 exists
		// is a genuine secret. This is the case that proves the permission
		// narrows and never widens — carol holds report:tickets and still
		// cannot see a ticket she may not read.
		if rec.Code != http.StatusNotFound {
			t.Fatalf("got %d, want 404 (permission must not widen entity access); body=%s",
				rec.Code, rec.Body)
		}
		if fake.callCount() != before {
			t.Errorf("LEAK: entity-denied request invoked the renderer")
		}
	})
}

// TestAnchoredDocument_GateOrderingNoTypeOracle pins the gate ORDER in the
// anchored handler, which a comment there calls load-bearing but nothing
// previously enforced.
//
// The ENTITY read gate must fire before the entity_type-mismatch 400.
// Otherwise a principal who may not read an entity learns, from a 400 rather
// than a 404, that the id they guessed exists and is of some other type. That
// IS a real oracle: unlike a document name, whether an entity exists (and what
// type it is) is a genuine secret. TKT-M1AX6P inserted a second gate into that
// sequence, so the ordering is now easy to break by reordering two adjacent
// blocks.
func TestAnchoredDocument_GateOrderingNoTypeOracle(t *testing.T) {
	app := newTestAppV1(t)
	ticket := &entity.Entity{ID: "TKT-001", Type: "ticket", Properties: map[string]any{"title": "t"}}
	feature := &entity.Entity{ID: "FEA-001", Type: "feature", Properties: map[string]any{"title": "f"}}
	withFakeDocRenderer(t, app, ticket, feature)
	seedEntity(app, ticket)
	seedEntity(app, feature)
	app.State().Cfg.Documents = map[string]dataentryconfig.DocumentConfig{
		"ticket_report": {EntityType: "ticket", Script: "docs/t.lua"},
	}

	// bob may read NEITHER type, so the entity gate denies him whichever id he
	// aims at. That is the principal for whom the ordering matters.
	d := mustNewACL(t, &acl.Policy{
		Roles:       map[string]acl.RoleDef{"outsider": {}},
		Assignments: map[string]string{"bob": "outsider"},
	}, app.store)
	app.acl = d

	// Whether bob aims at a right-type or wrong-type id, the response must be
	// the same 404 — no type information either way.
	probe := func(t *testing.T, id string) *httptest.ResponseRecorder {
		t.Helper()
		rec := httptest.NewRecorder()
		app.handleV1Documents(rec, httptest.NewRequest(http.MethodGet,
			"/api/v1/_documents/ticket_report/"+id, http.NoBody).
			WithContext(gateCtxFor(principalCtx("bob"), t, d)))
		return rec
	}

	rightType, wrongType := probe(t, "TKT-001"), probe(t, "FEA-001")

	if rightType.Code != http.StatusNotFound || wrongType.Code != http.StatusNotFound {
		t.Fatalf("denied principal must get 404 for both ids, got %d (right-type) and %d (wrong-type)"+
			"\n  a 400 here means a gate moved below the type-mismatch check",
			rightType.Code, wrongType.Code)
	}

	// Compare every field except `instance`, which necessarily differs because
	// it echoes the requested URL — information the caller supplied, not
	// information about the entity.
	right, wrong := decodeProblem(t, rightType), decodeProblem(t, wrongType)
	delete(right, "instance")
	delete(wrong, "instance")
	if !maps.Equal(right, wrong) {
		t.Errorf("wrong-type probe distinguishable from right-type probe (type oracle):\n right=%v\n wrong=%v",
			right, wrong)
	}
}

// TestSidebarAndConfig_PrincipalIndependent pins the OPPOSITE of what an
// earlier draft of this feature did.
//
// Menu structure and config are served identically to every principal, gated
// document or not. docs/acl-security.md § "Sidebar menu structure is
// principal-independent" already recorded this decision — the metamodel is not
// a secret, and a divergent menu per principal complicates SPA caching for no
// confidentiality gain. `data-entry.yaml` is an operator-authored file in the
// repo, so its keys, script paths and permission names are already disclosed
// (see "The configuration is not a secret; the data is" in the root
// CLAUDE.md).
//
// The render endpoint still enforces `permission:` — see
// TestStandaloneDocument_PermissionGate. A user may therefore see a menu entry
// that 403s, which is the accepted trade: an actionable error beats a menu
// that silently differs per principal.
func TestSidebarAndConfig_PrincipalIndependent(t *testing.T) {
	app := newTestAppV1(t)
	app.State().Cfg.Documents = map[string]dataentryconfig.DocumentConfig{
		"sales_review": {Title: "Verkoop", Script: "docs/sales.lua", Permission: "report:sales"},
		"open_report":  {Title: "Open", Script: "docs/open.lua"},
	}
	app.Cfg().Navigation = []dataentryconfig.NavigationEntry{
		{Label: "Verkoop", Document: "sales_review"},
		{Label: "Open", Document: "open_report"},
	}

	d := mustNewACL(t, &acl.Policy{
		Roles: map[string]acl.RoleDef{
			"directie": {Read: []string{"ticket"}, Permissions: []string{"report:sales"}},
			"viewer":   {Read: []string{"ticket"}},
		},
		Assignments: map[string]string{"alice": "directie", "bob": "viewer"},
	}, app.store)
	app.acl = d

	bodyFor := func(t *testing.T, ctx context.Context, path string,
		h func(http.ResponseWriter, *http.Request),
	) string {
		t.Helper()
		rec := httptest.NewRecorder()
		h(rec, httptest.NewRequest(http.MethodGet, path, http.NoBody).
			WithContext(gateCtxFor(ctx, t, d)))
		if rec.Code != http.StatusOK {
			t.Fatalf("%s: got %d, want 200; body=%s", path, rec.Code, rec.Body)
		}
		return rec.Body.String()
	}

	t.Run("sidebar is byte-identical for a holder and a non-holder", func(t *testing.T) {
		alice := bodyFor(t, aliceCtx(), "/api/v1/_sidebar", app.views.handleV1Sidebar)
		bob := bodyFor(t, principalCtx("bob"), "/api/v1/_sidebar", app.views.handleV1Sidebar)
		if alice != bob {
			t.Errorf("sidebar differs by principal:\n alice=%s\n bob=%s", alice, bob)
		}
		if !strings.Contains(bob, "/document/sales_review") {
			t.Errorf("gated document missing from the non-holder's sidebar: %s", bob)
		}
	})

	t.Run("config is byte-identical for a holder and a non-holder", func(t *testing.T) {
		alice := bodyFor(t, aliceCtx(), "/api/v1/_config", app.handleV1Config)
		bob := bodyFor(t, principalCtx("bob"), "/api/v1/_config", app.handleV1Config)
		if alice != bob {
			t.Errorf("config differs by principal:\n alice=%s\n bob=%s", alice, bob)
		}
	})
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
