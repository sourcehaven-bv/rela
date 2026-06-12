package dataentry

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/principal"
)

// TestACLMiddleware_FailLoudOnApi pins AC7 + RR-875A: with ACL
// configured and the principal stamper failing (unstamped principal),
// `/api/` requests MUST return 500 with the `acl_unstamped_principal`
// code. Silent fall-through would be fail-open — every read becomes
// AllowAll because no Request is attached.
func TestACLMiddleware_FailLoudOnApi(t *testing.T) {
	d := mustNewACL(t, &acl.Policy{}, newTestAppV1(t).store)

	// Wrap a sentinel "next" handler that records whether it was
	// reached — the middleware MUST short-circuit before calling next
	// when ForPrincipal fails.
	called := false
	next := http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) { called = true })
	handler := attachACLRequest(next, d)

	// Construct a request whose ctx has NO principal (the stamper bug
	// case): principal.From returns the unknown/unknown default, which
	// is what triggers ErrUnstampedPrincipal.
	req := httptest.NewRequest(http.MethodGet, "/api/v1/tickets", http.NoBody)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("API path under unstamped principal: got %d, want 500", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "acl_unstamped_principal") {
		t.Errorf("missing acl_unstamped_principal code: %s", rec.Body)
	}
	if called {
		t.Error("next handler was called; middleware must short-circuit on fail-loud")
	}
}

// TestACLMiddleware_NonAPIPathsBypass pins RR-T15E: ACL configured +
// unstamped principal must NOT take down the SPA shell or static
// assets. Operators need the UI reachable to recover from a stamper
// misconfig — locking them out of the recovery surface is the
// failure mode this guard prevents.
func TestACLMiddleware_NonAPIPathsBypass(t *testing.T) {
	d := mustNewACL(t, &acl.Policy{}, newTestAppV1(t).store)

	called := false
	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	})
	handler := attachACLRequest(next, d)

	for _, path := range []string{"/", "/index.html", "/static/app.js", "/assets/main.css"} {
		t.Run(path, func(t *testing.T) {
			called = false
			req := httptest.NewRequest(http.MethodGet, path, http.NoBody)
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)
			if rec.Code != http.StatusOK {
				t.Errorf("%s: got %d, want 200 (non-/api/ paths bypass ACL)", path, rec.Code)
			}
			if !called {
				t.Errorf("%s: next handler not called; middleware should pass through", path)
			}
		})
	}
}

// TestACLMiddleware_RouterChainOrder pins the CRIT-1 regression: when
// the router wraps `attachACLRequest` OUTSIDE `stampAuditPrincipal`
// (the previous broken order), every `/api/` request to an ACL-
// configured app returns 500 acl_unstamped_principal because the
// principal isn't stamped yet at the time the ACL middleware reads
// it. This test asserts the WHOLE chain via NewRouter(), not just the
// individual middleware — the bug only existed at the composition
// site.
func TestACLMiddleware_RouterChainOrder(t *testing.T) {
	app := newTestAppV1(t)
	seedEntity(app, &entity.Entity{ID: "TKT-001", Type: "ticket", Properties: map[string]any{"title": "T1"}})

	// Configure ACL with alice as a viewer of tickets — so a real
	// stamped principal would resolve to a non-empty Request.
	d := mustNewACL(t, &acl.Policy{
		Roles:       map[string]acl.RoleDef{"viewer": {Read: []string{"ticket"}}},
		Assignments: map[string]string{"alice": "viewer"},
	}, app.store)
	app.acl = d

	// Install a resolver that returns a stamped principal for every
	// request. With the CORRECT wrap order (stamp outermost → ACL
	// inner), ForPrincipal sees alice and succeeds. With the BROKEN
	// order (ACL outermost → stamp inner), ForPrincipal sees the
	// default unstamped principal and 500s.
	app.SetPrincipalResolver(func(*http.Request) principal.Principal {
		return principal.Principal{User: "alice", Tool: principal.ToolDataEntry}
	})

	handler := app.NewRouter()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/tickets/TKT-001", http.NoBody)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	// With the bug present, code would be 500 + body containing
	// "acl_unstamped_principal". With the fix, code should be 200
	// (alice is a viewer of tickets).
	if strings.Contains(rec.Body.String(), "acl_unstamped_principal") {
		t.Fatalf("CRIT-1 regression: router wrap order put ACL outside stamp; "+
			"got %d %s — fix attachACLRequest to wrap before stampAuditPrincipal",
			rec.Code, rec.Body)
	}
	if rec.Code != http.StatusOK {
		t.Errorf("GET /api/v1/tickets/TKT-001 under stamped alice: got %d, want 200; body=%s",
			rec.Code, rec.Body)
	}
}

// TestACLMiddleware_StampedPrincipalAttachesGate pins the happy path
// *behaviourally* (RR-CAFF): under a policy that allows ticket reads
// but denies document reads, the attached gate MUST return true for
// a ticket and false for a document. Type-equality assertions (the
// previous shape) pinned implementation, not behavior — a future
// broken aclReadGate would still pass.
func TestACLMiddleware_StampedPrincipalAttachesGate(t *testing.T) {
	app := newTestAppV1(t)
	seedEntity(app, &entity.Entity{ID: "TKT-001", Type: "ticket", Properties: map[string]any{"title": "t"}})
	seedEntity(app, &entity.Entity{ID: "DOC-001", Type: "document", Properties: map[string]any{"title": "d"}})

	d := mustNewACL(t, &acl.Policy{
		Roles:       map[string]acl.RoleDef{"viewer": {Read: []string{"ticket"}}},
		Assignments: map[string]string{"alice": "viewer"},
	}, app.store)

	var sawRequest bool
	var permitsTicket, permitsDoc bool
	next := http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		sawRequest = acl.FromContext(r.Context()) != nil
		gate := readGateFromContext(r.Context())
		permitsTicket, _ = gate.PermitsRead(r.Context(), "ticket", "TKT-001")
		permitsDoc, _ = gate.PermitsRead(r.Context(), "document", "DOC-001")
	})
	handler := attachACLRequest(next, d)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/tickets", http.NoBody)
	req = req.WithContext(principal.With(req.Context(),
		principal.Principal{User: "alice", Tool: principal.ToolDataEntry}))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if !sawRequest {
		t.Error("acl.FromContext returned nil; Request not attached")
	}
	if !permitsTicket {
		t.Error("gate.PermitsRead(ticket): false; want true (viewer has read:[ticket])")
	}
	if permitsDoc {
		t.Error("gate.PermitsRead(document): true; want false (no read grant on document)")
	}
}
