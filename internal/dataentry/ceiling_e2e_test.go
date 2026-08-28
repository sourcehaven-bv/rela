package dataentry

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/entity"
)

// End-to-end client attenuation over real HTTP (TKT-IAC8TX).
//
// The unit tests in internal/acl prove the ceiling computes correctly. These
// prove it SURVIVES THE REQUEST PATH — a separate question, because the path
// rebuilds the Principal twice (verifiedPrincipal, then
// resolvePrincipalEntity) and either rebuild can silently drop a claim. A
// dropped principal_type matches no baseline, so the client is UNRESTRICTED:
// the failure is invisible and fails open.
//
// READS ONLY. The write gate lives in entitymanager, and `newTestAppV1` does
// not wire its entitymanager with the same ACL as the router (the same
// limitation noted in acl_write_test.go), so a write test here would exercise a
// NopACL manager and pass no matter what the ceiling said — worse than no test.
// Write-path attenuation is covered at the gate itself, in
// internal/entitymanager/ceiling_test.go.

// ceilingRequest issues an authenticated GET as the given client.
func ceilingRequest(t *testing.T, app *App, path string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, path, http.NoBody)
	req.Header.Set(assertionHeader, "good")
	rec := httptest.NewRecorder()
	app.NewRouter().ServeHTTP(rec, req)
	return rec
}

// denyReadApp wires an App where app-type clients cannot read tickets at all,
// while the acting user is a full editor.
func denyReadApp(t *testing.T) *App {
	t.Helper()
	app := newTestAppV1(t)
	seedEntity(app, &entity.Entity{
		ID: "TKT-001", Type: "ticket", Properties: map[string]any{"title": "T1"},
	})
	policy, err := acl.LoadPolicyBytes([]byte(`
roles:
  editor:
    read: [ticket]
    update: [ticket]
assignments:
  usr_alice: editor
client_baselines:
  apps:
    applies_to: [app]
    deny_read: [ticket]
`))
	if err != nil {
		t.Fatalf("LoadPolicyBytes: %v", err)
	}
	app.acl = mustNewACL(t, policy, app.store)
	app.SetPrincipalResolver(ChainResolvers(JWTPrincipalResolver(stubVerifier{
		validToken: "good", subject: "usr_alice", principalType: "app",
	}, assertionHeader)))
	return app
}

// TestCeilingE2E_DenyReadIsIndistinguishableFrom404 pins AC9, the one place
// client attenuation touches a secrecy boundary.
//
// Row-level denial must make the entity NONEXISTENT, not forbidden: a 403 on a
// denied read is an existence oracle, telling the caller the id is real. The
// two responses must be byte-identical.
func TestCeilingE2E_DenyReadIsIndistinguishableFrom404(t *testing.T) {
	app := denyReadApp(t)

	denied := ceilingRequest(t, app, "/api/v1/tickets/TKT-001")
	absent := ceilingRequest(t, app, "/api/v1/tickets/TKT-NOPE")

	if denied.Code != http.StatusNotFound {
		t.Errorf("denied read = %d, want 404 (a 403 is an existence oracle)\n%s",
			denied.Code, denied.Body)
	}
	if absent.Code != http.StatusNotFound {
		t.Fatalf("missing entity = %d, want 404", absent.Code)
	}
	// Compare the bodies with the echoed request path normalized away: the
	// `instance` field legitimately differs (it is the URL the caller just
	// typed, so it reveals nothing they did not already know). Everything else
	// — status, type, title — must be identical, since any difference there is
	// the oracle.
	normalize := func(rec *httptest.ResponseRecorder, id string) string {
		return strings.ReplaceAll(rec.Body.String(), id, "<id>")
	}
	if got, want := normalize(denied, "TKT-001"), normalize(absent, "TKT-NOPE"); got != want {
		t.Errorf("denied and missing responses differ beyond the echoed path — "+
			"the difference IS the oracle:\n denied:  %s\n missing: %s", got, want)
	}
}

// TestCeilingE2E_DenyReadPrunesTheList: the row must also vanish from list
// responses, or the 404 above is undone by the collection endpoint.
func TestCeilingE2E_DenyReadPrunesTheList(t *testing.T) {
	app := denyReadApp(t)

	rec := ceilingRequest(t, app, "/api/v1/tickets")
	if rec.Code != http.StatusOK {
		t.Fatalf("GET list = %d, want 200\n%s", rec.Code, rec.Body)
	}
	var payload struct {
		Entities []json.RawMessage `json:"entities"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode list: %v\n%s", err, rec.Body)
	}
	if len(payload.Entities) != 0 {
		t.Errorf("list returned %d entities under deny_read; want none pruned in\n%s",
			len(payload.Entities), rec.Body)
	}
	if strings.Contains(rec.Body.String(), "TKT-001") {
		t.Errorf("denied id leaked into the list body:\n%s", rec.Body)
	}
}

// TestCeilingE2E_RESTSurfacesInheritTheCeiling probes REST routes beyond the
// two the AC tests cover, confirming the ceiling reaches them all rather than
// just the entity GET and list.
//
// It is a survey rather than a per-route assertion on purpose: the ceiling
// rides on the acl.Request that attachACLRequest attaches to every /api/ path
// (isAPIPath), so coverage is structural. What this pins is that no probed
// surface leaks a denied id into its body.
func TestCeilingE2E_RESTSurfacesInheritTheCeiling(t *testing.T) {
	app := denyReadApp(t)

	// Collection-shaped surfaces only. A per-entity GET is excluded because its
	// 404 body echoes the requested path — which contains the id the caller
	// just typed, so it is not a leak. That case is covered precisely by
	// TestCeilingE2E_DenyReadIsIndistinguishableFrom404, which normalizes the
	// echoed path away and compares everything else.
	for _, path := range []string{
		"/api/v1/tickets",
		"/api/v1/_search?q=T1",
		"/api/v1/_analyze",
		"/api/v1/_sidebar",
		"/api/v1/_schema",
	} {
		t.Run(path, func(t *testing.T) {
			rec := ceilingRequest(t, app, path)
			if rec.Code >= 500 {
				t.Fatalf("%s = %d (server error)\n%s", path, rec.Code, rec.Body)
			}
			if strings.Contains(rec.Body.String(), "TKT-001") {
				t.Errorf("%s leaked the denied entity id:\n%s", path, rec.Body)
			}
		})
	}
}
