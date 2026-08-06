package dataentry

import (
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/principal"
)

// End-to-end: a verified assertion drives an authorization decision and lands
// in the audit log (TKT-RP3X3Q).
//
// Unit tests on the resolver and on acl.computeGlobals both pass even when a
// field is silently DROPPED between them — and there is a real dropper in the
// path: resolvePrincipalEntity rebuilds the Principal field by field, so a
// plain composite literal there loses org and roles while every unit test
// stays green. These tests run the whole chain: HTTP request → resolver →
// principal_property re-stamp → ACL decision → audit record.

// e2eAssertionApp wires an App whose JWT resolver returns the given claims for
// the token "good", with an ACL policy granting `editor` via an asserted claim.
func e2eAssertionApp(t *testing.T, id AssertedIdentity, policy *acl.Policy) *App {
	t.Helper()
	app := newTestAppV1(t)
	seedEntity(app, &entity.Entity{
		ID: "TKT-001", Type: "ticket", Properties: map[string]any{"title": "T1"},
	})

	app.acl = mustNewACL(t, policy, app.store)

	v := stubVerifier{
		validToken: "good",
		subject:    id.Subject,
		orgID:      id.OrgID,
		orgSlug:    id.OrgSlug,
		roles:      id.Roles,
	}
	app.SetPrincipalResolver(ChainResolvers(JWTPrincipalResolver(v, assertionHeader)))
	return app
}

func assertedGetTicket(t *testing.T, app *App, token string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/tickets/TKT-001", http.NoBody)
	req.Header.Set(assertionHeader, token)
	rec := httptest.NewRecorder()
	app.NewRouter().ServeHTTP(rec, req)
	return rec
}

func TestE2E_AssertedClaimAuthorizesRequest(t *testing.T) {
	app := e2eAssertionApp(t,
		AssertedIdentity{
			Subject: "usr_1", OrgID: "org_acme", OrgSlug: "acme",
			Roles: []string{"admin"},
		},
		&acl.Policy{
			Roles:         map[string]acl.RoleDef{"viewer": {Read: []string{"ticket"}}},
			AssertedRoles: map[string]acl.RoleList{"admin": {"viewer"}},
		})

	rec := assertedGetTicket(t, app, "good")

	if strings.Contains(rec.Body.String(), "acl_unstamped_principal") {
		t.Fatalf("principal was not stamped: %d %s", rec.Code, rec.Body)
	}
	if rec.Code != http.StatusOK {
		t.Errorf("got %d, want 200 — the asserted claim should grant read; body=%s",
			rec.Code, rec.Body)
	}
}

func TestE2E_WithoutTheClaimTheRequestIsDenied(t *testing.T) {
	// The negative half: same policy, a token whose claim maps to nothing.
	// Without this, the test above could pass for the wrong reason (e.g. the
	// policy accidentally granting read to everyone).
	app := e2eAssertionApp(t,
		AssertedIdentity{Subject: "usr_1", Roles: []string{"not-mapped"}},
		&acl.Policy{
			Roles:         map[string]acl.RoleDef{"viewer": {Read: []string{"ticket"}}},
			AssertedRoles: map[string]acl.RoleList{"admin": {"viewer"}},
		})

	rec := assertedGetTicket(t, app, "good")

	if rec.Code == http.StatusOK {
		t.Errorf("got 200 for a principal holding no mapped claim; body=%s", rec.Body)
	}
}

func TestE2E_AssertedClaimsReachTheWritePath(t *testing.T) {
	// The Principal stamped on the request context is what the entitymanager
	// hands to the audit sink, so asserting on it here covers the audit path
	// without reaching into the manager's internally-wired sink.
	app := e2eAssertionApp(t,
		AssertedIdentity{
			Subject: "usr_1", OrgID: "org_acme", OrgSlug: "acme",
			Roles: []string{"admin"},
		},
		&acl.Policy{
			Roles: map[string]acl.RoleDef{"editor": {
				Read: []string{"ticket"}, Update: []string{"ticket"},
			}},
			AssertedRoles: map[string]acl.RoleList{"admin": {"editor"}},
		})

	req := httptest.NewRequest(http.MethodPatch, "/api/v1/tickets/TKT-001",
		strings.NewReader(`{"properties":{"title":"updated"}}`))
	req.Header.Set(assertionHeader, "good")
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	app.NewRouter().ServeHTTP(rec, req)

	if rec.Code < 200 || rec.Code >= 300 {
		t.Fatalf("write failed: %d %s", rec.Code, rec.Body)
	}

	got := stampedPrincipal(t, app, req)

	if got.User != "usr_1" {
		t.Errorf("User = %q, want usr_1", got.User)
	}
	if got.OrgID() != "org_acme" {
		t.Errorf("OrgID = %q, want org_acme — org was dropped before the write path",
			got.OrgID())
	}
	if want := []string{"admin"}; !slices.Equal(got.Roles(), want) {
		t.Errorf("Roles = %v, want %v — roles were dropped in the chain",
			got.Roles(), want)
	}
}

func TestE2E_ClaimsSurvivePrincipalPropertyReStamp(t *testing.T) {
	// The specific regression this file exists for. With principal_property
	// enabled AND a matching entity, resolvePrincipalEntity rebuilds the
	// Principal — replacing User with the entity ID. Everything it does not
	// explicitly carry over is lost, silently.
	app := newTestAppV1(t)
	seedEntity(app, &entity.Entity{
		ID: "TKT-001", Type: "ticket", Properties: map[string]any{"title": "T1"},
	})
	seedEntity(app, &entity.Entity{
		ID: "PERS-alice", Type: "person",
		Properties: map[string]any{"email": "alice@example.com"},
	})

	d, err := acl.NewDeclarative(
		&acl.Policy{
			UserEntityType:    "person",
			PrincipalProperty: "email",
			Roles: map[string]acl.RoleDef{"editor": {
				Read: []string{"ticket"}, Update: []string{"ticket"},
			}},
			AssertedRoles: map[string]acl.RoleList{"admin": {"editor"}},
		},
		acl.NewStoreGraph(app.store), app.store,
		acl.WithPrincipalLookup(acl.NewStorePrincipalLookup(app.store)),
	)
	if err != nil {
		t.Fatalf("NewDeclarative: %v", err)
	}
	app.acl = d

	// Drive the resolver + the ACL middleware, which is where the re-stamp runs,
	// and capture the principal the downstream handler would see.
	verified := principal.Verified("alice@example.com", principal.ToolDataEntry,
		"org_acme", "", []string{"admin"})

	var got principal.Principal
	handler := attachACLRequest(
		http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
			got = principal.From(r.Context())
		}), d, false)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/tickets/TKT-001", http.NoBody)
	handler.ServeHTTP(httptest.NewRecorder(),
		req.WithContext(principal.With(req.Context(), verified)))

	// The re-stamp happened: User is now the entity ID, RawUser the email.
	if got.User != "PERS-alice" {
		t.Fatalf("User = %q, want the resolved entity ID PERS-alice — the re-stamp "+
			"did not run, so this test is not exercising the drop path", got.User)
	}
	if got.RawUser != "alice@example.com" {
		t.Errorf("RawUser = %q, want the pre-resolution email", got.RawUser)
	}
	// ...and the assertion claims survived it.
	if want := []string{"admin"}; !slices.Equal(got.Roles(), want) {
		t.Errorf("Roles = %v, want %v — resolvePrincipalEntity dropped the claims "+
			"when it rebuilt the Principal", got.Roles(), want)
	}
	if got.OrgID() != "org_acme" {
		t.Errorf("OrgID = %q, want org_acme — dropped by the re-stamp", got.OrgID())
	}
}

func TestE2E_ForgedAssertionGrantsNothing(t *testing.T) {
	// A token the verifier rejects must fall through to the unstamped default,
	// carrying no claims — not authorize with the claims it merely asserted.
	app := e2eAssertionApp(t,
		AssertedIdentity{Subject: "usr_1", Roles: []string{"admin"}},
		&acl.Policy{
			Roles:         map[string]acl.RoleDef{"viewer": {Read: []string{"ticket"}}},
			AssertedRoles: map[string]acl.RoleList{"admin": {"viewer"}},
		})

	rec := assertedGetTicket(t, app, "forged")

	if rec.Code == http.StatusOK {
		t.Errorf("a forged token authorized the request: %d %s", rec.Code, rec.Body)
	}
}

func TestE2E_UnmatchedPrincipalStillCarriesClaims(t *testing.T) {
	// AC10 end-to-end: principal_property is enabled but NO person entity
	// matches. The request must still succeed on its asserted role — this is
	// the SSO-provisioned user's very first request.
	app := newTestAppV1(t)
	seedEntity(app, &entity.Entity{
		ID: "TKT-001", Type: "ticket", Properties: map[string]any{"title": "T1"},
	})

	d, err := acl.NewDeclarative(
		&acl.Policy{
			UserEntityType:    "person",
			PrincipalProperty: "email",
			Roles:             map[string]acl.RoleDef{"viewer": {Read: []string{"ticket"}}},
			AssertedRoles:     map[string]acl.RoleList{"admin": {"viewer"}},
		},
		acl.NewStoreGraph(app.store), app.store,
		acl.WithPrincipalLookup(acl.NewStorePrincipalLookup(app.store)),
	)
	if err != nil {
		t.Fatalf("NewDeclarative: %v", err)
	}
	app.acl = d

	v := stubVerifier{
		validToken: "good",
		subject:    "newcomer@example.com", // no matching person entity
		roles:      []string{"admin"},
	}
	app.SetPrincipalResolver(ChainResolvers(JWTPrincipalResolver(v, assertionHeader)))

	rec := assertedGetTicket(t, app, "good")

	if rec.Code != http.StatusOK {
		t.Errorf("got %d, want 200 — a verified principal with no user entity must "+
			"keep its asserted grants (TKT-0C3II2 tracks provisioning); body=%s",
			rec.Code, rec.Body)
	}
}

// TestE2E_HeaderModeUnaffected pins AC3 at the integration level: with the JWT
// resolver disabled and a plain trusted header, behavior is exactly as before
// this feature — and no roles appear anywhere.
func TestE2E_HeaderModeUnaffected(t *testing.T) {
	app := newTestAppV1(t)
	seedEntity(app, &entity.Entity{
		ID: "TKT-001", Type: "ticket", Properties: map[string]any{"title": "T1"},
	})
	app.acl = mustNewACL(t, &acl.Policy{
		Roles:       map[string]acl.RoleDef{"viewer": {Read: []string{"ticket"}}},
		Assignments: map[string]string{"alice": "viewer"},
	}, app.store)
	app.SetPrincipalResolver(ChainResolvers(HeaderPrincipalResolver("X-Forwarded-User")))

	req := httptest.NewRequest(http.MethodGet, "/api/v1/tickets/TKT-001", http.NoBody)
	req.Header.Set("X-Forwarded-User", "alice")
	// A header that looks like a roles claim must be inert.
	req.Header.Set("X-Asserted-Roles", "superuser")
	rec := httptest.NewRecorder()
	app.NewRouter().ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("header mode broke: %d %s", rec.Code, rec.Body)
	}

	p := stampedPrincipal(t, app, req)
	if len(p.Roles()) != 0 {
		t.Errorf("header-mode principal carried roles %v", p.Roles())
	}
}

// stampedPrincipal runs just the resolver chain for req and returns the
// Principal it stamps, so a test can inspect it directly.
func stampedPrincipal(t *testing.T, app *App, req *http.Request) principal.Principal {
	t.Helper()
	var got principal.Principal
	h := stampAuditPrincipal(
		http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
			got = principal.From(r.Context())
		}),
		app.principalResolver,
	)
	h.ServeHTTP(httptest.NewRecorder(), req)
	return got
}
