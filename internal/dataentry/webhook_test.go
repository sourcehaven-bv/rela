package dataentry

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/dataentryconfig"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/state"
	"github.com/Sourcehaven-BV/rela/internal/storage"
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
// 502 and does not record the id, so a retry (same id) is NOT deduped and runs again.
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
		t.Fatalf("retry after failure status = %d, want 502 (a failed id must not be recorded)", rec.Code)
	}
}

// TestSeenSet_TTLExpiry: a done id past the TTL is treated as new again.
func TestSeenSet_TTLExpiry(t *testing.T) {
	s := newSeenSet(time.Minute, nil)
	base := time.Unix(1_700_000_000, 0)
	s.now = func() time.Time { return base }

	if !mustHandle(t, s, "a") {
		t.Fatal("first delivery should be new")
	}
	if mustHandle(t, s, "a") {
		t.Fatal("an immediate redelivery should be a duplicate")
	}
	// Advance past the TTL.
	s.now = func() time.Time { return base.Add(2 * time.Minute) }
	if !mustHandle(t, s, "a") {
		t.Fatal("a redelivery after the TTL should be new again")
	}
}

// TestSeenSet_ClaimStates pins the claim: a claimed id is in flight to
// every other caller until it is finished (done) or released (new again).
func TestSeenSet_ClaimStates(t *testing.T) {
	ctx := context.Background()
	s := newSeenSet(time.Minute, nil)
	mustClaim(t, s, "a", seenNew)
	mustClaim(t, s, "a", seenInFlight)
	if err := s.release(ctx, "a"); err != nil {
		t.Fatal(err)
	}
	mustClaim(t, s, "a", seenNew)
	if err := s.finish(ctx, "a"); err != nil {
		t.Fatal(err)
	}
	mustClaim(t, s, "a", seenDone)
	// A release never undoes a finished id.
	if err := s.release(ctx, "a"); err != nil {
		t.Fatal(err)
	}
	mustClaim(t, s, "a", seenDone)
}

// TestSeenSet_ClaimLapses pins that a claim whose release was never saved
// lapses after the claim TTL, well before a done id would, so the IdP's
// later retry runs and the event is not lost.
func TestSeenSet_ClaimLapses(t *testing.T) {
	s := newSeenSet(time.Hour, nil)
	base := time.Unix(1_700_000_000, 0)
	s.now = func() time.Time { return base }
	mustClaim(t, s, "a", seenNew)

	s.now = func() time.Time { return base.Add(webhookClaimTTL - time.Second) }
	mustClaim(t, s, "a", seenInFlight)
	s.now = func() time.Time { return base.Add(webhookClaimTTL + time.Second) }
	mustClaim(t, s, "a", seenNew)
}

// TestSeenSet_SurvivesEviction pins that the set lives in state.KV: a new
// set over the same kv, as a reopened tenant App builds, still dedups an id
// the old one handled, and sees its claims (BUG-ZO33J9M).
func TestSeenSet_SurvivesEviction(t *testing.T) {
	kv := newMemKV(t)

	before := newSeenSet(time.Minute, kv)
	if !mustHandle(t, before, "a") {
		t.Fatal("first delivery should be new")
	}
	mustClaim(t, before, "b", seenNew)

	after := newSeenSet(time.Minute, kv)
	mustClaim(t, after, "a", seenDone)
	mustClaim(t, after, "b", seenInFlight)
	if !mustHandle(t, after, "c") {
		t.Error("a new id was deduped")
	}
	// The first set sees what the second recorded.
	mustClaim(t, before, "c", seenDone)
}

func newMemKV(t *testing.T) state.KV {
	t.Helper()
	root, err := storage.NewRootedFS(storage.NewMemFS(), t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	return state.NewFSKV(root)
}

func mustClaim(t *testing.T, s *seenSet, id string, want seenState) {
	t.Helper()
	got, err := s.claim(context.Background(), id)
	if err != nil {
		t.Fatalf("claim(%q): %v", id, err)
	}
	if got != want {
		t.Fatalf("claim(%q) = %d, want %d", id, got, want)
	}
}

// mustHandle claims id and, when the claim is new, finishes it, as a
// successful delivery does. It reports whether the delivery was new.
func mustHandle(t *testing.T, s *seenSet, id string) bool {
	t.Helper()
	got, err := s.claim(context.Background(), id)
	if err != nil {
		t.Fatalf("claim(%q): %v", id, err)
	}
	if got != seenNew {
		return false
	}
	if err := s.finish(context.Background(), id); err != nil {
		t.Fatalf("finish(%q): %v", id, err)
	}
	return true
}

// brokenKV fails every read.
type brokenKV struct{ state.KV }

func (brokenKV) Get(context.Context, string) ([]byte, error) { return nil, errors.New("db down") }

// putFailKV reads as empty and fails every write.
type putFailKV struct{ state.KV }

func (putFailKV) Get(context.Context, string) ([]byte, error) { return nil, os.ErrNotExist }
func (putFailKV) Put(context.Context, string, []byte) error   { return errors.New("db down") }

// failAfterKV passes the first n writes to the wrapped kv and fails the
// rest, so a claim saves and the finish or release after it does not.
type failAfterKV struct {
	state.KV
	n int
}

func (k *failAfterKV) Put(ctx context.Context, key string, data []byte) error {
	if k.n == 0 {
		return errors.New("db down")
	}
	k.n--
	return k.KV.Put(ctx, key, data)
}

// TestWebhook_DedupUnavailable pins that the receiver answers 503 with
// Retry-After when the stored id set cannot be read or the claim cannot be
// saved (the handler does not run), or the handled id cannot be marked done
// (the IdP retries; the handler is idempotent), so no webhook is accepted
// without replay protection.
func TestWebhook_DedupUnavailable(t *testing.T) {
	claims := WebhookClaims{Event: "organization.created", OrgID: "org_1", ID: "evt_kv"}
	for _, tc := range []struct {
		name string
		// kv wraps a working in-memory kv.
		kv    func(mem state.KV) state.KV
		calls int
	}{
		{"read fails", func(state.KV) state.KV { return brokenKV{} }, 0},
		{"claim write fails", func(state.KV) state.KV { return putFailKV{} }, 0},
		{"done write fails", func(mem state.KV) state.KV { return &failAfterKV{KV: mem, n: 1} }, 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h := &recordingOrgHandler{}
			app := newOrgWebhookTestApp(t, claims, h)
			app.webhook.seen = newSeenSet(time.Minute, tc.kv(newMemKV(t)))
			rec := postWebhook(app, "b")
			if rec.Code != http.StatusServiceUnavailable || rec.Header().Get("Retry-After") == "" {
				t.Fatalf("status = %d, Retry-After = %q; want 503 with Retry-After",
					rec.Code, rec.Header().Get("Retry-After"))
			}
			if len(h.got) != tc.calls {
				t.Errorf("org handler calls = %d, want %d", len(h.got), tc.calls)
			}
		})
	}
}

// blockingOrgHandler blocks each event until release is closed, after
// sending on entered.
type blockingOrgHandler struct {
	entered chan struct{}
	release chan struct{}
	calls   atomic.Int32
}

func (h *blockingOrgHandler) HandleOrgEvent(context.Context, WebhookClaims) error {
	h.calls.Add(1)
	h.entered <- struct{}{}
	<-h.release
	return nil
}

// TestWebhook_ConcurrentDuplicateInFlight pins the race the claim closes: a
// copy of an id that arrives while the first delivery still runs does not
// run; it gets 503 with Retry-After so it is not lost. Once the first
// finishes, the redelivery is deduped with 200.
func TestWebhook_ConcurrentDuplicateInFlight(t *testing.T) {
	claims := WebhookClaims{Event: "organization.created", OrgID: "org_1", ID: "evt_race"}
	h := &blockingOrgHandler{entered: make(chan struct{}, 1), release: make(chan struct{})}
	app := newOrgWebhookTestApp(t, claims, h)

	first := make(chan *httptest.ResponseRecorder, 1)
	go func() { first <- postWebhook(app, "b") }()
	<-h.entered

	rec := postWebhook(app, "b")
	if rec.Code != http.StatusServiceUnavailable || rec.Header().Get("Retry-After") == "" {
		t.Fatalf("in-flight copy: status = %d, Retry-After = %q; want 503 with Retry-After",
			rec.Code, rec.Header().Get("Retry-After"))
	}
	close(h.release)
	if got := (<-first).Code; got != http.StatusOK {
		t.Fatalf("first delivery status = %d, want 200", got)
	}
	if rec := postWebhook(app, "b"); rec.Code != http.StatusOK {
		t.Fatalf("redelivery after success: status = %d, want 200", rec.Code)
	}
	if n := h.calls.Load(); n != 1 {
		t.Errorf("org handler calls = %d, want 1", n)
	}
}

// TestWebhook_FailedReleaseLapses pins that a failure whose release cannot
// be saved still lets a later retry run: the claim lapses after the claim
// TTL.
func TestWebhook_FailedReleaseLapses(t *testing.T) {
	claims := WebhookClaims{Event: "organization.created", OrgID: "org_1", ID: "evt_rel"}
	h := &recordingOrgHandler{err: errors.New("boom")}
	app := newOrgWebhookTestApp(t, claims, h)
	kv := &failAfterKV{KV: newMemKV(t), n: 1}
	seen := newSeenSet(time.Hour, kv)
	base := time.Unix(1_700_000_000, 0)
	seen.now = func() time.Time { return base }
	app.webhook.seen = seen

	if rec := postWebhook(app, "b"); rec.Code != http.StatusBadGateway {
		t.Fatalf("failing delivery status = %d, want 502", rec.Code)
	}
	// The claim stayed: a retry right away is in flight.
	kv.n = 100
	if rec := postWebhook(app, "b"); rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("retry within the claim TTL: status = %d, want 503", rec.Code)
	}
	seen.now = func() time.Time { return base.Add(webhookClaimTTL + time.Second) }
	h.err = nil
	if rec := postWebhook(app, "b"); rec.Code != http.StatusOK {
		t.Fatalf("retry after the claim TTL: status = %d, want 200", rec.Code)
	}
	if len(h.got) != 2 {
		t.Errorf("org handler calls = %d, want 2", len(h.got))
	}
}

// TestSeenSet_FailsClosed pins that an unreadable stored set is an error,
// never a fresh id: a replay could otherwise pass.
func TestSeenSet_FailsClosed(t *testing.T) {
	s := newSeenSet(time.Minute, brokenKV{})
	if _, err := s.claim(context.Background(), "a"); err == nil {
		t.Fatal("claim over a failing kv succeeded; want an error")
	}
}

// TestSeenSet_Bounded pins the ceiling on the persisted set.
func TestSeenSet_Bounded(t *testing.T) {
	s := newSeenSet(time.Hour, nil)
	base := time.Unix(1_700_000_000, 0)
	n := 0
	s.now = func() time.Time { n++; return base.Add(time.Duration(n) * time.Millisecond) }
	for i := range webhookSeenMax + 10 {
		mustHandle(t, s, fmt.Sprintf("id-%d", i))
	}
	if len(s.at) > webhookSeenMax {
		t.Fatalf("set holds %d ids, want at most %d", len(s.at), webhookSeenMax)
	}
	if _, ok := s.at["id-0"]; ok {
		t.Error("the oldest id was kept over the ceiling")
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

// TestWebhook_RefusesElevatedAction: the IdP webhook never elevates, so an
// action that declares allow_acl_bypass is refused there before its script
// runs (TKT-JA5SABG).
func TestWebhook_RefusesElevatedAction(t *testing.T) {
	script := `rela.create_entity("ticket", { title = "should-not-run" })
	           return { message = "ok" }`
	v := stubWebhookVerifier{claims: WebhookClaims{
		Event: "membership.created", UserID: "usr_1", OrgID: "org_1", ID: "evt_elevated",
	}}
	app := newWebhookTestApp(t, script, v)
	app.Cfg().Actions["idp-sync"] = dataentryconfig.Action{
		Script:          "idp-sync.lua",
		Permission:      "sync",
		AllowACLBypass:  metamodel.ACLBypassWrite,
		ElevatedUpdates: map[string][]string{"ticket": {"status"}},
	}

	if rec := postWebhook(app, "e1"); rec.Code != http.StatusBadGateway {
		t.Fatalf("status = %d, want 502 (%s)", rec.Code, rec.Body.String())
	}
	if n := countTickets(t, app); n != 0 {
		t.Fatalf("elevated action ran from the webhook (%d entities)", n)
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
// jti dedup, and a handler failure answers 502 and records no id, so the
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
			t.Fatalf("org handler calls = %d, want 2 (a failed id must not be recorded)", len(h.got))
		}
	})
}
