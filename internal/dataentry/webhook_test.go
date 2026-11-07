package dataentry

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/dataentryconfig"
)

// errStubReject stands in for a verification failure in the shim test — the
// handler only branches on err != nil, so the concrete error doesn't matter.
var errStubReject = errors.New("stub: rejected")

// stubWebhookVerifier returns a fixed result, so the shim test exercises the
// dispatch/dedup/principal mechanics without a JWKS server (real verification is
// covered in internal/jwtauth).
type stubWebhookVerifier struct {
	claims WebhookClaims
	err    error
}

func (s stubWebhookVerifier) VerifyWebhook(context.Context, string) (WebhookClaims, error) {
	return s.claims, s.err
}

// newWebhookTestApp wires an App with the given action script + a webhook
// receiver backed by the stub verifier dispatching to action id "idp-sync".
func newWebhookTestApp(t *testing.T, script string, v webhookVerifier) *App {
	t.Helper()
	app := newActionTestApp(t, map[string]string{"idp-sync.lua": script})
	app.Cfg().Actions = map[string]dataentryconfig.Action{
		"idp-sync": {Script: "idp-sync.lua"},
	}
	app.SetWebhookReceiver(v, "idp-sync", nil)
	return app
}

func postWebhook(app *App, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, "/webhooks/idp", strings.NewReader(body))
	rec := httptest.NewRecorder()
	// Drive the receiver's handler with the same dispatch closure production wires
	// in registerWebhookRoutes.
	r := app.webhook
	r.handle(rec, req, func(ctx context.Context, claims WebhookClaims) error {
		return dispatchWebhookAction(ctx, app.write, r, claims)
	})
	return rec
}

// TestWebhook_DispatchesActionWithClaims: a verified webhook runs the action with
// event/user_id/org_id as params. The action echoes user_id back in its message.
func TestWebhook_DispatchesActionWithClaims(t *testing.T) {
	script := `return { message = "synced " .. rela.params["user_id"] .. " in " .. rela.params["org_id"] }`
	v := stubWebhookVerifier{claims: WebhookClaims{
		Event: "membership.created", UserID: "usr_1", OrgID: "org_1", ID: "evt_1",
	}}
	app := newWebhookTestApp(t, script, v)

	rec := postWebhook(app, "any.jwt.body")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body %q)", rec.Code, rec.Body.String())
	}
}

// TestWebhook_DedupsByID: a redelivered webhook (same id) does not re-run the
// action. The action increments a counter entity; after two deliveries the count
// must be 1.
func TestWebhook_DedupsByID(t *testing.T) {
	// The action creates a ticket on each run; we count runs via list length.
	script := `rela.create_entity("ticket", { title = "run" })
	           return { message = "ok" }`
	v := stubWebhookVerifier{claims: WebhookClaims{
		Event: "membership.created", UserID: "usr_1", OrgID: "org_1", ID: "evt_dup",
	}}
	app := newWebhookTestApp(t, script, v)

	if rec := postWebhook(app, "b1"); rec.Code != http.StatusOK {
		t.Fatalf("first delivery status = %d (%s)", rec.Code, rec.Body.String())
	}
	if rec := postWebhook(app, "b2"); rec.Code != http.StatusOK {
		t.Fatalf("redelivery status = %d (%s)", rec.Code, rec.Body.String())
	}

	got := countTickets(t, app)
	if got != 1 {
		t.Fatalf("action ran %d times; want 1 (redelivery must be deduped)", got)
	}
}

// TestWebhook_RejectsBadSignature: a verification failure → 401 and the action
// never runs.
func TestWebhook_RejectsBadSignature(t *testing.T) {
	script := `rela.create_entity("ticket", { title = "should-not-run" })
	           return { message = "ok" }`
	v := stubWebhookVerifier{err: errStubReject}
	app := newWebhookTestApp(t, script, v)

	rec := postWebhook(app, "forged")
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rec.Code)
	}
	if n := countTickets(t, app); n != 0 {
		t.Fatalf("action ran on a rejected webhook (%d entities)", n)
	}
}

// TestWebhook_MissingUserID: a verified webhook with no user_id → 400 (nothing to
// provision), action not run.
func TestWebhook_MissingUserID(t *testing.T) {
	script := `rela.create_entity("ticket", { title = "x" }); return {}`
	v := stubWebhookVerifier{claims: WebhookClaims{Event: "membership.created", ID: "evt_x"}}
	app := newWebhookTestApp(t, script, v)

	rec := postWebhook(app, "b")
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
	if n := countTickets(t, app); n != 0 {
		t.Fatalf("action ran despite missing user_id (%d entities)", n)
	}
}

// TestWebhook_ActionFailureRetryable: when the action fails, the receiver answers
// 502 AND forgets the id, so a retry (same id) is NOT deduped and runs again.
func TestWebhook_ActionFailureRetryable(t *testing.T) {
	// The action always fails. Both deliveries (same id) must reach it and 502 —
	// proving a failed id is not remembered, so the IdP's retry runs again.
	script := `error("boom")`
	v := stubWebhookVerifier{claims: WebhookClaims{
		Event: "membership.created", UserID: "usr_1", OrgID: "org_1", ID: "evt_retry",
	}}
	app := newWebhookTestApp(t, script, v)

	if rec := postWebhook(app, "b1"); rec.Code != http.StatusBadGateway {
		t.Fatalf("first failure status = %d, want 502 (%s)", rec.Code, rec.Body.String())
	}
	// If the failed id had been remembered, this would 200 (deduped). It must 502
	// again — the retry reaches the action.
	if rec := postWebhook(app, "b2"); rec.Code != http.StatusBadGateway {
		t.Fatalf("retry after failure status = %d, want 502 (id must be forgotten on failure)", rec.Code)
	}
}

// TestSeenSet_TTLExpiry: an id past the TTL is treated as new again.
func TestSeenSet_TTLExpiry(t *testing.T) {
	s := newSeenSet(time.Minute)
	base := time.Unix(1_700_000_000, 0)
	s.now = func() time.Time { return base }

	if !s.add("a") {
		t.Fatal("first add should be new")
	}
	if s.add("a") {
		t.Fatal("immediate re-add should be a duplicate")
	}
	// Advance past the TTL.
	s.now = func() time.Time { return base.Add(2 * time.Minute) }
	if !s.add("a") {
		t.Fatal("re-add after TTL should be new again")
	}
}

// countTickets returns how many tickets exist in the app's store. The webhook
// test scripts create one ticket per run, so this counts action runs.
func countTickets(t *testing.T, app *App) int {
	t.Helper()
	return len(entitiesByType(app, "ticket"))
}

// TestWebhook_ReachableThroughRouter drives POST /webhooks/idp through the REAL
// production router (NewRouter), not the handler directly. This is the gap that
// let BUG-F3ADZO ship: every other webhook test calls rec.handle(...) straight,
// so none of them route through the mux — and the route was registered on the
// `inner` sub-mux, which is only mounted under /api/. A path that does not carry
// the /api/ prefix therefore fell through to the SPA catch-all and returned
// 200 HTML, so the webhook handler never ran.
//
// Oracle: the webhook handler answers a bodyless malformed request with a 4xx
// (it cannot verify an empty JWT). The SPA catch-all answers 200 with an HTML
// body. So "not 200, and not an HTML body" means the handler actually ran.
func TestWebhook_ReachableThroughRouter(t *testing.T) {
	app := newWebhookTestApp(t, "return true", stubWebhookVerifier{err: errStubReject})

	req := httptest.NewRequest(http.MethodPost, "/webhooks/idp", strings.NewReader("not-a-jwt"))
	// Clear the host allowlist (the test app binds 127.0.0.1:8080). Without a
	// matching Host the security layer 403s before routing, which would mask the
	// routing bug this test exists to catch.
	req.Host = "127.0.0.1:8080"
	rec := httptest.NewRecorder()
	app.NewRouter().ServeHTTP(rec, req)

	if rec.Code == http.StatusOK {
		t.Fatalf("POST /webhooks/idp returned 200 — it fell through to the SPA "+
			"catch-all instead of reaching the webhook handler (BUG-F3ADZO). Body: %.80q",
			rec.Body.String())
	}
	if strings.Contains(rec.Body.String(), "<!") || strings.Contains(rec.Body.String(), "<html") {
		t.Fatalf("POST /webhooks/idp returned an HTML body — the SPA shell answered, "+
			"not the webhook handler. Status %d", rec.Code)
	}
}

// TestWebhook_RefusesEntityBoundAction: an action with available_on needs an
// entity and a user, which the IdP webhook supplies neither of, so it never
// runs there (TKT-VVS16W).
func TestWebhook_RefusesEntityBoundAction(t *testing.T) {
	script := `rela.create_entity("ticket", { title = "should-not-run" })
	           return { message = "ok" }`
	v := stubWebhookVerifier{claims: WebhookClaims{
		Event: "membership.created", UserID: "usr_1", OrgID: "org_1", ID: "evt_bound",
	}}
	app := newWebhookTestApp(t, script, v)
	app.Cfg().Actions["idp-sync"] = dataentryconfig.Action{
		Script:      "idp-sync.lua",
		AvailableOn: &dataentryconfig.ActionScope{EntityTypes: []string{"ticket"}},
	}

	if rec := postWebhook(app, "b1"); rec.Code != http.StatusBadGateway {
		t.Fatalf("status = %d, want 502 (%s)", rec.Code, rec.Body.String())
	}
	if n := countTickets(t, app); n != 0 {
		t.Fatalf("entity-bound action ran from the webhook (%d entities)", n)
	}
}

// recordingOrgHandler records the organization events it receives and
// returns err from each call.
type recordingOrgHandler struct {
	got []WebhookClaims
	err error
}

func (h *recordingOrgHandler) HandleOrgEvent(_ context.Context, claims WebhookClaims) error {
	h.got = append(h.got, claims)
	return h.err
}

// newOrgWebhookTestApp wires a receiver whose action creates a ticket on each
// run, and whose organization handler is h.
func newOrgWebhookTestApp(t *testing.T, claims WebhookClaims, h webhookOrgHandler) *App {
	t.Helper()
	script := `rela.create_entity("ticket", { title = "run" })
	           return { message = "ok" }`
	app := newWebhookTestApp(t, script, stubWebhookVerifier{claims: claims})
	app.webhook.org = h
	return app
}

// TestWebhook_OrganizationEventReachesOrgHandler pins acceptance criterion 1
// of TKT-2R2RW5H: a verified organization event with org_id and no user_id is
// acknowledged and reaches the organization handler, not the action.
func TestWebhook_OrganizationEventReachesOrgHandler(t *testing.T) {
	for _, event := range []string{"organization.created", "organization.updated", "organization.deleted"} {
		t.Run(event, func(t *testing.T) {
			claims := WebhookClaims{Event: event, OrgID: "org_1", ID: "evt_" + event}
			h := &recordingOrgHandler{}
			app := newOrgWebhookTestApp(t, claims, h)

			rec := postWebhook(app, "b")
			if rec.Code != http.StatusOK {
				t.Fatalf("status = %d, want 200 (%s)", rec.Code, rec.Body.String())
			}
			if len(h.got) != 1 || h.got[0] != claims {
				t.Fatalf("org handler got %+v, want one call with %+v", h.got, claims)
			}
			if n := countTickets(t, app); n != 0 {
				t.Fatalf("action ran for an organization event (%d entities)", n)
			}
		})
	}
}

// TestWebhook_OrganizationEventWithUserIDStillRoutesToOrgHandler: the family
// decides the route, not which claims happen to be present.
func TestWebhook_OrganizationEventWithUserIDStillRoutesToOrgHandler(t *testing.T) {
	claims := WebhookClaims{Event: "organization.updated", UserID: "usr_1", OrgID: "org_1", ID: "evt_o"}
	h := &recordingOrgHandler{}
	app := newOrgWebhookTestApp(t, claims, h)

	if rec := postWebhook(app, "b"); rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (%s)", rec.Code, rec.Body.String())
	}
	if len(h.got) != 1 {
		t.Fatalf("org handler calls = %d, want 1", len(h.got))
	}
	if n := countTickets(t, app); n != 0 {
		t.Fatalf("action ran for an organization event (%d entities)", n)
	}
}

// TestWebhook_DefaultOrgHandlerAcknowledges: until provisioning (TKT-TNPRV8)
// and erasure (TKT-TNERAS) fill the seam, the receiver as wired by
// SetWebhookReceiver acknowledges organization events.
func TestWebhook_DefaultOrgHandlerAcknowledges(t *testing.T) {
	script := `rela.create_entity("ticket", { title = "run" }); return {}`
	v := stubWebhookVerifier{claims: WebhookClaims{Event: "organization.created", OrgID: "org_1", ID: "evt_d"}}
	app := newWebhookTestApp(t, script, v)
	if _, ok := app.webhook.org.(logOrgEvents); !ok {
		t.Fatalf("default org handler = %T, want logOrgEvents", app.webhook.org)
	}

	if rec := postWebhook(app, "b"); rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (%s)", rec.Code, rec.Body.String())
	}
	if n := countTickets(t, app); n != 0 {
		t.Fatalf("action ran for an organization event (%d entities)", n)
	}
}

// TestClassifyWebhookEvent records the matching rules: an exact,
// case-sensitive prefix up to and including the dot.
func TestClassifyWebhookEvent(t *testing.T) {
	cases := []struct {
		event string
		want  webhookFamily
	}{
		{"organization.created", familyOrganization},
		{"organization.suspended", familyOrganization},
		{"organization.", familyOrganization},
		{"membership.deleted", familyMembership},
		{"Organization.created", familyOther},
		{"organization", familyOther},
		{"organizations.created", familyOther},
		{"session.revoked", familyOther},
		{"", familyOther},
	}
	for _, tc := range cases {
		if got := classifyWebhookEvent(tc.event); got != tc.want {
			t.Errorf("classifyWebhookEvent(%q) = %v, want %v", tc.event, got, tc.want)
		}
	}
}

// TestWebhook_RequiredClaimsPerFamily pins acceptance criteria 2 and 3 of
// TKT-2R2RW5H, and the rule for events outside both families.
func TestWebhook_RequiredClaimsPerFamily(t *testing.T) {
	cases := []struct {
		name        string
		claims      WebhookClaims
		wantMissing string
	}{
		{"membership without user_id", WebhookClaims{Event: "membership.created", OrgID: "org_1"}, "user_id"},
		{"membership without org_id", WebhookClaims{Event: "membership.deleted", UserID: "usr_1"}, "org_id"},
		{"membership without either", WebhookClaims{Event: "membership.updated"}, "user_id"},
		{"organization without org_id", WebhookClaims{Event: "organization.created"}, "org_id"},
		{"organization with only user_id", WebhookClaims{Event: "organization.deleted", UserID: "usr_1"}, "org_id"},
		{"other event without user_id", WebhookClaims{Event: "user.updated", OrgID: "org_1"}, "user_id"},
		{"empty event without user_id", WebhookClaims{OrgID: "org_1"}, "user_id"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tc.claims.ID = "evt_bad"
			h := &recordingOrgHandler{}
			app := newOrgWebhookTestApp(t, tc.claims, h)

			rec := postWebhook(app, "b")
			if rec.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want 400 (%s)", rec.Code, rec.Body.String())
			}
			if want := "missing " + tc.wantMissing; !strings.Contains(rec.Body.String(), want) {
				t.Fatalf("body = %q, want it to contain %q", rec.Body.String(), want)
			}
			if len(h.got) != 0 {
				t.Fatalf("org handler ran on a rejected event")
			}
			if n := countTickets(t, app); n != 0 {
				t.Fatalf("action ran on a rejected event (%d entities)", n)
			}
		})
	}
}

// TestWebhook_OtherEventsReachAction: an event outside the membership and
// organization families still runs the action when it names a user. The
// action is the provider-specific layer and may ignore the event, so the
// receiver does not keep a list of event names.
func TestWebhook_OtherEventsReachAction(t *testing.T) {
	claims := WebhookClaims{Event: "session.revoked", UserID: "usr_1", ID: "evt_s"}
	h := &recordingOrgHandler{}
	app := newOrgWebhookTestApp(t, claims, h)

	if rec := postWebhook(app, "b"); rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (%s)", rec.Code, rec.Body.String())
	}
	if n := countTickets(t, app); n != 1 {
		t.Fatalf("action ran %d times, want 1", n)
	}
	if len(h.got) != 0 {
		t.Fatalf("org handler ran for a non-organization event")
	}
}

// TestWebhook_OrganizationEventDedupAndRetry: organization events share the
// jti dedup, and a handler failure answers 502 and forgets the id so the
// retry runs again.
func TestWebhook_OrganizationEventDedupAndRetry(t *testing.T) {
	claims := WebhookClaims{Event: "organization.created", OrgID: "org_1", ID: "evt_od"}

	t.Run("dedup", func(t *testing.T) {
		h := &recordingOrgHandler{}
		app := newOrgWebhookTestApp(t, claims, h)
		for i := range 2 {
			if rec := postWebhook(app, "b"); rec.Code != http.StatusOK {
				t.Fatalf("delivery %d status = %d (%s)", i, rec.Code, rec.Body.String())
			}
		}
		if len(h.got) != 1 {
			t.Fatalf("org handler calls = %d, want 1 (redelivery must be deduped)", len(h.got))
		}
	})

	t.Run("failure is retryable", func(t *testing.T) {
		h := &recordingOrgHandler{err: errors.New("boom")}
		app := newOrgWebhookTestApp(t, claims, h)
		for i := range 2 {
			if rec := postWebhook(app, "b"); rec.Code != http.StatusBadGateway {
				t.Fatalf("delivery %d status = %d, want 502", i, rec.Code)
			}
		}
		if len(h.got) != 2 {
			t.Fatalf("org handler calls = %d, want 2 (failed id must be forgotten)", len(h.got))
		}
	})
}
