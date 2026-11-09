package dataentry

import (
	"context"
	"errors"
	"log/slog"
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
	// when identity resolution fails.
	called := false
	next := http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) { called = true })
	handler := attachACLRequest(next, d, false)

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

// TestACLMiddleware_AnonymousIsUnauthenticated pins BUG-C8DHGF: a request
// the stamper ran on but no resolver named a user for (no proxy header, no
// RELA_DATAENTRY_USER) is a client without credentials. It gets the 401 the
// JWT gate and writeNoIdentity send, not a 500. The handler is still never
// reached. TestACLMiddleware_FailLoudOnApi keeps the 500 for a ctx the
// stamper never ran on.
func TestACLMiddleware_AnonymousIsUnauthenticated(t *testing.T) {
	tests := []struct {
		name   string
		header string // X-Remote-User value; empty sends no header
	}{
		{"no header", ""},
		{"placeholder user", principal.Unknown},
		{"header sanitized to empty", "\x01\x02"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			d := mustNewACL(t, &acl.Policy{}, newTestAppV1(t).store)
			called := false
			next := http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) { called = true })
			handler := stampAuditPrincipal(attachACLRequest(next, d, false), ChainResolvers(
				HeaderPrincipalResolver("X-Remote-User")))

			req := httptest.NewRequest(http.MethodGet, "/api/v1/tickets", http.NoBody)
			if tc.header != "" {
				req.Header.Set("X-Remote-User", tc.header)
			}
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)

			if rec.Code != http.StatusUnauthorized {
				t.Errorf("API path without an identity: got %d, want 401: %s", rec.Code, rec.Body)
			}
			if !strings.Contains(rec.Body.String(), "unauthenticated") {
				t.Errorf("missing unauthenticated code: %s", rec.Body)
			}
			// TKT-QR1VTP: the detail names the settings that supply an identity.
			for _, want := range []string{EnvDataEntryUserVar, "--principal-header", "--jwt-*"} {
				if !strings.Contains(rec.Body.String(), want) {
					t.Errorf("detail does not mention %s: %s", want, rec.Body)
				}
			}
			if called {
				t.Error("next handler was called; the middleware must refuse the request")
			}
		})
	}
}

// TestACLMiddleware_NamedUserUnknownToolStays500 pins the other side of
// BUG-C8DHGF: a stamped user with an unknown tool is a stamper fault, not a
// client without credentials, so it keeps the 500.
func TestACLMiddleware_NamedUserUnknownToolStays500(t *testing.T) {
	d := mustNewACL(t, &acl.Policy{}, newTestAppV1(t).store)
	called := false
	next := http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) { called = true })
	handler := attachACLRequest(next, d, false)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/tickets", http.NoBody)
	req = req.WithContext(principal.With(req.Context(),
		principal.Principal{User: "alice", Tool: principal.Unknown}))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("named user, unknown tool: got %d, want 500: %s", rec.Code, rec.Body)
	}
	if !strings.Contains(rec.Body.String(), "acl_unstamped_principal") {
		t.Errorf("missing acl_unstamped_principal code: %s", rec.Body)
	}
	if called {
		t.Error("next handler was called; the middleware must refuse the request")
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
	handler := attachACLRequest(next, d, false)

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
	setTestACL(t, app, d)

	// Install a resolver that returns a stamped principal for every
	// request. With the CORRECT wrap order (stamp outermost → ACL
	// inner), identity resolution sees alice and succeeds. With the BROKEN
	// order (ACL outermost → stamp inner), identity resolution sees the
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
	handler := attachACLRequest(next, d, false)

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

// downMembership fails every graph read, so the member-of walk fails.
type downMembership struct{ acl.NullGraph }

func (downMembership) OutgoingRelations(context.Context, string, string) ([]string, error) {
	return nil, errors.New("graph down: secret backend detail")
}

// TestACLMiddleware_WalkFailureFailsTheRequest: when the member-of walk
// fails, the request edge does not continue with a partial or missing
// identity. It answers 500 without the backend error text, and the handler
// never runs.
func TestACLMiddleware_WalkFailureFailsTheRequest(t *testing.T) {
	d, err := acl.NewDeclarative(&acl.Policy{}, downMembership{}, acl.NullGraphQueryer{})
	if err != nil {
		t.Fatal(err)
	}
	called := false
	handler := attachACLRequest(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { called = true }), d, false)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/tickets", http.NoBody)
	req = req.WithContext(principal.With(req.Context(),
		principal.Principal{User: "alice", Tool: principal.ToolDataEntry}))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError || !strings.Contains(rec.Body.String(), "acl_identity_unresolved") {
		t.Errorf("got %d %s, want 500 acl_identity_unresolved", rec.Code, rec.Body)
	}
	if strings.Contains(rec.Body.String(), "secret backend detail") {
		t.Errorf("response leaks the backend error: %s", rec.Body)
	}
	if called {
		t.Error("next handler ran without a resolved identity")
	}
}

// TestReadGateFromContext_DeniesUnderPolicyWithoutGate pins that a request
// served under a Declarative policy that reaches a read with no gate attached
// (a route outside /api/, such as a webhook that forgot to attach one) is
// denied every read. Allow-all is only for the wiring with no policy.
func TestReadGateFromContext_DeniesUnderPolicyWithoutGate(t *testing.T) {
	d := mustNewACL(t, &acl.Policy{}, newTestAppV1(t).store)

	var got readGate
	next := http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		got = readGateFromContext(r.Context())
	})
	attachACLRequest(next, d, false).ServeHTTP(httptest.NewRecorder(),
		httptest.NewRequest(http.MethodPost, "/hooks/example", http.NoBody))

	ctx := context.Background()
	if ok, err := got.PermitsRead(ctx, "ticket", "TKT-1"); ok || err != nil {
		t.Errorf("PermitsRead = %v, %v; want false", ok, err)
	}
	if rq := got.ReadQuery(ctx, "ticket"); !rq.DenyAll {
		t.Errorf("ReadQuery = %+v; want DenyAll", rq)
	}
	if scope := got.SearchScope(ctx, []string{"ticket"}); len(scope) != 0 {
		t.Errorf("SearchScope = %v; want empty", scope)
	}
	if got.HoldsPermission(ctx, acl.PermHistoryRead) {
		t.Error("HoldsPermission = true; want false")
	}
	if ok, _ := got.PermitsWorld(ctx, "draft"); ok {
		t.Error("PermitsWorld = true; want false")
	}
	if v, _ := got.ReadableFacesMany(ctx, "ticket", []string{"TKT-1"}); !v.For("TKT-1").None() {
		t.Errorf("ReadableFacesMany admits TKT-1")
	}

	if _, nop := readGateFromContext(context.Background()).(nopReadGate); !nop {
		t.Error("a context with no policy mark must keep the nop gate")
	}
}

// TestACLMiddleware_CanceledWalkStaysSilent: a member-of walk that fails
// because the client went away answers nothing and logs no warning
// (TKT-T2I7P2). The walk failure above, on a live request, still answers
// 500.
func TestACLMiddleware_CanceledWalkStaysSilent(t *testing.T) {
	appLog := withLogLevel(t, slog.LevelInfo)
	d, err := acl.NewDeclarative(&acl.Policy{}, downMembership{}, acl.NullGraphQueryer{})
	if err != nil {
		t.Fatal(err)
	}
	called := false
	handler := attachACLRequest(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { called = true }), d, false)

	ctx, cancel := context.WithCancel(principal.With(context.Background(),
		principal.Principal{User: "alice", Tool: principal.ToolDataEntry}))
	cancel()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/tickets", http.NoBody).WithContext(ctx)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Body.Len() != 0 {
		t.Errorf("a canceled request must get no answer, got %d %s", rec.Code, rec.Body)
	}
	if strings.Contains(appLog.String(), "level=WARN") {
		t.Errorf("a canceled request must not log a warning, got %q", appLog.String())
	}
	if called {
		t.Error("next handler ran without a resolved identity")
	}
}
