package dataentry

import (
	"crypto/rand"
	"encoding/json"
	"errors"
	"log/slog"
	"maps"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/peerkey"
	"github.com/Sourcehaven-BV/rela/internal/principal"
	"github.com/Sourcehaven-BV/rela/internal/reconwire"
	"github.com/Sourcehaven-BV/rela/internal/testutil/logtest"

	"golang.org/x/time/rate"
)

// The peer key edge (TKT-H91N91L, DEC-81161C3): a sync peer authenticates
// with a peer key, runs as peer:<name> under the source's ACL, and may only
// read.

// peerFixture is a router with an ACL, the reconcile endpoint and one peer
// key for peer:acme, which may read tickets only.
type peerFixture struct {
	app  *App
	key  string
	keys *mutableKeys
}

// mutableKeys is a peer key list a test can change.
type mutableKeys struct{ entries map[string]string }

func (m *mutableKeys) load() (map[string]string, error) { return m.entries, nil }

func newPeerKey(t *testing.T) (key, digest string) {
	t.Helper()
	key, digest, err := peerkey.Generate(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	return key, digest
}

func newPeerFixture(t *testing.T, resolver PrincipalResolver, clock func() time.Time) *peerFixture {
	t.Helper()
	app := newTestAppV1(t)
	for _, e := range []*entity.Entity{
		{ID: "TKT-001", Type: "ticket", Properties: map[string]any{"title": "one"}},
		{ID: "TKT-002", Type: "ticket", Properties: map[string]any{"title": "two"}},
		{ID: "FEAT-001", Type: "feature", Properties: map[string]any{"title": "f1"}},
	} {
		seedEntity(app, e)
	}
	d := mustNewACL(t, &acl.Policy{
		Roles: map[string]acl.RoleDef{
			"tickets-reader": {Read: []string{"ticket"}, Create: []string{"ticket"}, Update: []string{"ticket"}},
			"admin":          {Read: []string{"*"}},
		},
		Assignments: map[string]string{"peer:acme": "tickets-reader", "alice": "admin"},
	}, app.store)
	setTestACL(t, app, d)
	if err := SetReconcile(app, localElements(t, app, d)); err != nil {
		t.Fatal(err)
	}
	if resolver != nil {
		app.SetPrincipalResolver(resolver)
	}
	key, digest := newPeerKey(t)
	keys := &mutableKeys{entries: map[string]string{"acme": digest}}
	if clock == nil {
		clock = time.Now
	}
	src, err := peerkey.NewSource(keys.load, peerkey.ReloadInterval, clock, nil)
	if err != nil {
		t.Fatal(err)
	}
	SetPeerKeys(app, src)
	return &peerFixture{app: app, key: key, keys: keys}
}

func (f *peerFixture) do(t *testing.T, h http.Handler, method, path, body string,
	hdr map[string]string,
) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	for k, v := range hdr {
		req.Header.Set(k, v)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func bearer(key string) map[string]string { return map[string]string{"Authorization": "Bearer " + key} }

func reconcileCount(t *testing.T, rec *httptest.ResponseRecorder) int {
	t.Helper()
	if rec.Code != http.StatusOK {
		t.Fatalf("reconcile: %d %s", rec.Code, rec.Body)
	}
	var out reconwire.Response
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	return int(*out.Ranges[0].Count)
}

func fingerprintBody(t *testing.T, types ...string) string {
	t.Helper()
	b, err := json.Marshal(reconwire.Request{Ranges: []reconwire.Range{fpRange(types...)}})
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// TestPeerKey_ReadsOnlyWhatItsRoleAllows is AC1: through the list endpoints
// and _reconcile, the peer sees tickets and no feature.
func TestPeerKey_ReadsOnlyWhatItsRoleAllows(t *testing.T) {
	f := newPeerFixture(t, nil, nil)
	h := f.app.NewRouter()

	if rec := f.do(t, h, http.MethodGet, "/api/v1/tickets/TKT-001", "", bearer(f.key)); rec.Code != http.StatusOK {
		t.Errorf("GET ticket: %d %s", rec.Code, rec.Body)
	}
	if rec := f.do(t, h, http.MethodGet, "/api/v1/features/FEAT-001", "", bearer(f.key)); rec.Code != http.StatusNotFound {
		t.Errorf("GET feature: %d %s, want 404", rec.Code, rec.Body)
	}
	rec := f.do(t, h, http.MethodGet, "/api/v1/features", "", bearer(f.key))
	if strings.Contains(rec.Body.String(), "FEAT-001") {
		t.Errorf("feature list leaked a row: %d %s", rec.Code, rec.Body)
	}
	rec = f.do(t, h, http.MethodPost, reconcilePath, fingerprintBody(t, "ticket", "feature"), bearer(f.key))
	if got := reconcileCount(t, rec); got != 2 {
		t.Errorf("reconcile count = %d, want 2 (the tickets)", got)
	}
	if v := rec.Header().Values("Vary"); !containsFold(v, "Authorization") {
		t.Errorf("Vary = %v, want Authorization", v)
	}
	if rec := f.do(t, h, http.MethodGet, schemaPath, "", bearer(f.key)); rec.Code != http.StatusOK {
		t.Errorf("GET schema: %d", rec.Code)
	}
}

func containsFold(values []string, want string) bool {
	for _, v := range values {
		for part := range strings.SplitSeq(v, ",") {
			if strings.EqualFold(strings.TrimSpace(part), want) {
				return true
			}
		}
	}
	return false
}

// TestPeerKey_BadKeysAnswer401 is AC2: a wrong, malformed or revoked key is
// 401 and never falls through to the header identity, which here would be
// an admin.
func TestPeerKey_BadKeysAnswer401(t *testing.T) {
	now := time.Unix(1000, 0)
	clock := func() time.Time { return now }
	f := newPeerFixture(t, ChainResolvers(HeaderPrincipalResolver("X-Forwarded-User")), clock)
	h := f.app.NewRouter()
	wrong, _ := newPeerKey(t)

	for _, tc := range []struct {
		name string
		hdr  map[string]string
	}{
		{"unknown key", bearer(wrong)},
		{"malformed key", bearer(peerkey.Prefix + "short")},
		{"lower-case scheme", map[string]string{"Authorization": "bearer " + peerkey.Prefix + "x"}},
		{"bare key", map[string]string{"Authorization": peerkey.Prefix + "x"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			hdr := map[string]string{"X-Forwarded-User": "alice"}
			maps.Copy(hdr, tc.hdr)
			rec := f.do(t, h, http.MethodGet, "/api/v1/tickets/TKT-001", "", hdr)
			if rec.Code != http.StatusUnauthorized {
				t.Errorf("%d %s, want 401", rec.Code, rec.Body)
			}
			if rec.Header().Get("WWW-Authenticate") != "Bearer" {
				t.Errorf("WWW-Authenticate = %q", rec.Header().Get("WWW-Authenticate"))
			}
		})
	}

	// Two Authorization values, one of them the valid key: ambiguous.
	req := httptest.NewRequest(http.MethodGet, "/api/v1/tickets/TKT-001", http.NoBody)
	req.Header.Add("Authorization", "Bearer "+f.key)
	req.Header.Add("Authorization", "Bearer other")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("two Authorization values: %d, want 401", rec.Code)
	}

	// Revoked: removing the digest stops the key after the reload interval.
	if rec := f.do(t, h, http.MethodGet, "/api/v1/tickets/TKT-001", "", bearer(f.key)); rec.Code != http.StatusOK {
		t.Fatalf("valid key: %d", rec.Code)
	}
	f.keys.entries = map[string]string{}
	now = now.Add(peerkey.ReloadInterval)
	if rec := f.do(t, h, http.MethodGet, "/api/v1/tickets/TKT-001", "", bearer(f.key)); rec.Code != http.StatusUnauthorized {
		t.Errorf("revoked key: %d, want 401", rec.Code)
	}

	// A credential that is not a peer key takes the normal chain.
	hdr := map[string]string{"X-Forwarded-User": "alice", "Authorization": "Bearer eyJhbGciOi"}
	if rec := f.do(t, h, http.MethodGet, "/api/v1/features/FEAT-001", "", hdr); rec.Code != http.StatusOK {
		t.Errorf("non-peer bearer: %d %s", rec.Code, rec.Body)
	}
}

// TestPeerKey_CannotWrite is AC3: the role may create and update tickets,
// but a peer key only reads.
func TestPeerKey_CannotWrite(t *testing.T) {
	f := newPeerFixture(t, nil, nil)
	h := f.app.NewRouter()
	for _, tc := range []struct{ method, path, body string }{
		{http.MethodPost, "/api/v1/tickets", `{"properties":{"title":"x"}}`},
		{http.MethodPatch, "/api/v1/tickets/TKT-001", `{"properties":{"title":"x"}}`},
		{http.MethodPut, "/api/v1/tickets/TKT-001", `{"properties":{"title":"x"}}`},
		{http.MethodDelete, "/api/v1/tickets/TKT-001", ""},
		{http.MethodGet, CollabPathPrefix + "ticket/TKT-001", ""},
		{http.MethodGet, "/api/v1/_events", ""},
		{http.MethodGet, "/api/v1/_config", ""},
		{http.MethodPost, "/api/v1/_batch", `{}`},
		{http.MethodGet, "/api/command/x", ""},
	} {
		t.Run(tc.method+" "+tc.path, func(t *testing.T) {
			rec := f.do(t, h, tc.method, tc.path, tc.body, bearer(f.key))
			if rec.Code != http.StatusForbidden || !strings.Contains(rec.Body.String(), "peer_route_refused") {
				t.Errorf("%d %s, want 403 peer_route_refused", rec.Code, rec.Body)
			}
		})
	}
	rec := f.do(t, h, http.MethodGet, "/api/v1/tickets/TKT-001", "", bearer(f.key))
	if rec.Code != http.StatusOK || strings.Contains(rec.Body.String(), `"title":"x"`) {
		t.Errorf("after the refused writes: %d %s", rec.Code, rec.Body)
	}
}

// TestPeerKey_NameCannotBeAsserted is AC4 for the header and the environment
// user: `peer:` is reserved.
func TestPeerKey_NameCannotBeAsserted(t *testing.T) {
	f := newPeerFixture(t, ChainResolvers(HeaderPrincipalResolver("X-Forwarded-User")), nil)
	h := f.app.NewRouter()
	rec := f.do(t, h, http.MethodGet, "/api/v1/tickets/TKT-001", "",
		map[string]string{"X-Forwarded-User": "peer:acme"})
	if rec.Code != http.StatusForbidden || !strings.Contains(rec.Body.String(), "reserved_principal") {
		t.Errorf("asserted peer name: %d %s, want 403 reserved_principal", rec.Code, rec.Body)
	}
	if !strings.Contains(rec.Body.String(), principal.PeerPrefix) {
		t.Errorf("the refusal does not name the peer prefix: %s", rec.Body)
	}

	t.Setenv(envDataEntryUser, "peer:acme")
	f = newPeerFixture(t, ChainResolvers(EnvPrincipalResolver()), nil)
	rec = f.do(t, f.app.NewRouter(), http.MethodGet, "/api/v1/tickets/TKT-001", "", nil)
	if rec.Code != http.StatusForbidden || !strings.Contains(rec.Body.String(), "reserved_principal") {
		t.Errorf("peer name from the environment: %d %s, want 403 reserved_principal", rec.Code, rec.Body)
	}
}

// TestPeerKey_JWTMode: with the JWT gate installed, a peer key still
// authenticates on its own, a bad key is 401, and a JWT subject in the peer
// namespace is refused.
func TestPeerKey_JWTMode(t *testing.T) {
	f := newPeerFixture(t, nil, nil)
	if err := f.app.SetJWTGate(JWTGateConfig{
		Verifier:   gateVerifier{validToken: "good-jwt", subject: "peer:acme"},
		HeaderName: "Authorization",
	}); err != nil {
		t.Fatal(err)
	}
	h := f.app.NewRouter()
	rec := f.do(t, h, http.MethodPost, reconcilePath, fingerprintBody(t, "ticket", "feature"), bearer(f.key))
	if got := reconcileCount(t, rec); got != 2 {
		t.Errorf("reconcile count = %d, want 2", got)
	}
	wrong, _ := newPeerKey(t)
	if rec := f.do(t, h, http.MethodGet, "/api/v1/tickets/TKT-001", "", bearer(wrong)); rec.Code != http.StatusUnauthorized {
		t.Errorf("wrong key: %d", rec.Code)
	}
	if rec := f.do(t, h, http.MethodGet, "/api/v1/tickets/TKT-001", "", bearer("good-jwt")); rec.Code != http.StatusUnauthorized {
		t.Errorf("JWT subject peer:acme: %d, want 401", rec.Code)
	}
}

// TestPeerKey_NoKeyValueInLogsOrBody is AC5.
func TestPeerKey_NoKeyValueInLogsOrBody(t *testing.T) {
	logs := logtest.Capture(t, slog.LevelDebug)

	f := newPeerFixture(t, nil, nil)
	h := f.app.NewRouter()
	wrong, _ := newPeerKey(t)
	rec := f.do(t, h, http.MethodGet, "/api/v1/tickets/TKT-001", "", bearer(wrong))
	ok := f.do(t, h, http.MethodGet, "/api/v1/tickets/TKT-001", "", bearer(f.key))
	for _, s := range []string{logs(), rec.Body.String(), ok.Body.String()} {
		for _, k := range []string{wrong, f.key, strings.TrimPrefix(wrong, peerkey.Prefix)} {
			if strings.Contains(s, k) {
				t.Errorf("a key value leaked: %q", s)
			}
		}
	}
	if !strings.Contains(logs(), "peer key refused") {
		t.Errorf("the refusal was not logged: %q", logs())
	}
}

// TestPeerKey_ReconcileGateKeyedOnPeer resolves RR-KT1PVVG: each peer has
// its own admission key, the peer principal.
func TestPeerKey_ReconcileGateKeyedOnPeer(t *testing.T) {
	f := newPeerFixture(t, nil, nil)
	// One token per principal and no refill: a second call by the same
	// principal is refused, a call by another is not (RR-KT1PVVG).
	gate := newReconcileGate(reconcileGateConfig{Rate: rate.Every(time.Hour), Burst: 1})
	var seen string
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen = principal.From(r.Context()).User
		release, refusal := gate.acquire(seen)
		if refusal != nil {
			w.WriteHeader(refusal.status)

			return
		}
		release()
	})
	h := stampAuditPrincipal(next, defaultPrincipalResolver, f.app.peerKeys)
	if rec := f.do(t, h, http.MethodPost, reconcilePath, "{}", bearer(f.key)); rec.Code != http.StatusOK ||
		seen != "peer:acme" {

		t.Fatalf("peer: %d as %q, want 200 as peer:acme", rec.Code, seen)
	}
	if rec := f.do(t, h, http.MethodPost, reconcilePath, "{}", bearer(f.key)); rec.Code == http.StatusOK {
		t.Fatal("the peer's second call was admitted with an empty bucket")
	}
	// Header mode with no identity header: the caller is unknown, which
	// has its own bucket, not the peer's.
	if rec := f.do(t, h, http.MethodPost, reconcilePath, "{}", nil); rec.Code != http.StatusOK ||
		seen == "peer:acme" {

		t.Fatalf("other caller: %d as %q", rec.Code, seen)
	}
}

func TestPeerRouteAllowed(t *testing.T) {
	for _, tc := range []struct {
		method, path string
		want         bool
	}{
		{http.MethodGet, "/api/v1/tickets", true},
		{http.MethodHead, "/api/v1/tickets/TKT-1", true},
		{http.MethodGet, "/api/v1/tickets/TKT-1/relations", true},
		{http.MethodGet, "/api/v1/tickets/TKT-1/relations/affects/FEAT-1", true},
		{http.MethodGet, "/api/v1/tickets/TKT-1/_history", false},
		{http.MethodGet, "/api/v1/tickets/_export", false},
		{http.MethodGet, "/api/v1/tickets/TKT-1/_export", false},
		{http.MethodGet, "/api/v1/tickets/TKT-1/_presence", false},
		{http.MethodGet, "/api/v1/tickets/TKT-1/_channels/chat/main/messages", false},
		{http.MethodGet, "/api/v1/tickets/TKT-1/_actions/close", false},
		{http.MethodGet, "/api/v1/tickets/TKT-1/_attachments/a.pdf", false},
		{http.MethodGet, "/api/v1/tickets/TKT-1/_lease", false},
		{http.MethodGet, "/api/v1/tickets/", false},
		{http.MethodGet, "/api/v1/_schema", true},
		{http.MethodGet, "/api/v1/_openapi.json", true},
		{http.MethodPost, "/api/v1/_reconcile", true},
		{http.MethodGet, "/api/v1/_reconcile", false},
		{http.MethodGet, "/api/v1/", false},
		{http.MethodGet, "/api/v1//x", false},
		{http.MethodGet, "/api/v1/_search", false},
		{http.MethodGet, "/api/v2/tickets", false},
		{http.MethodOptions, "/api/v1/tickets", false},
	} {
		if got := peerRouteAllowed(tc.method, tc.path); got != tc.want {
			t.Errorf("peerRouteAllowed(%s %s) = %v, want %v", tc.method, tc.path, got, tc.want)
		}
	}
}

// stubPeerKeys is a PeerKeys that fails verification with err.
type stubPeerKeys struct{ err error }

func (stubPeerKeys) IsPeerKey(c string) bool         { return strings.HasPrefix(c, "p_") }
func (s stubPeerKeys) Verify(string) (string, error) { return "", s.err }

// TestStampAuditPrincipal_PeerKeysOff: without peer keys a peer-shaped credential
// is just another header, and non-API paths never consult the keys.
func TestStampAuditPrincipal_PeerKeysOff(t *testing.T) {
	var seen string
	next := http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		seen = principal.From(r.Context()).User
	})
	h := stampAuditPrincipal(next, defaultPrincipalResolver, nil)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/tickets", http.NoBody)
	req.Header.Set("Authorization", "Bearer "+peerkey.Prefix+"x")
	h.ServeHTTP(httptest.NewRecorder(), req)
	if seen != principal.Unknown {
		t.Errorf("seen = %q, want %q", seen, principal.Unknown)
	}

	h = stampAuditPrincipal(next, defaultPrincipalResolver, stubPeerKeys{err: errors.New("no")})
	req = httptest.NewRequest(http.MethodGet, "/", http.NoBody)
	req.Header.Set("Authorization", "Bearer p_x")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || seen != principal.Unknown {
		t.Errorf("non-API path: %d, user %q", rec.Code, seen)
	}
}

// TestPeerKey_NoACLRefusesEveryKey: without an acl.yaml nothing limits what
// a peer reads, so a valid key is refused rather than reading everything.
func TestPeerKey_NoACLRefusesEveryKey(t *testing.T) {
	app := newTestAppV1(t)
	key, digest := newPeerKey(t)
	keys := &mutableKeys{entries: map[string]string{"acme": digest}}
	src, err := peerkey.NewSource(keys.load, peerkey.ReloadInterval, time.Now, nil)
	if err != nil {
		t.Fatal(err)
	}
	SetPeerKeys(app, src)
	f := &peerFixture{app: app, key: key, keys: keys}
	rec := f.do(t, app.NewRouter(), http.MethodGet, "/api/v1/tickets", "", bearer(key))
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("valid key without acl.yaml: %d %s, want 401", rec.Code, rec.Body)
	}
}

// TestPeerKey_MisplacedInIdentityHeader: a peer key sent as the principal
// header is refused on the API and never becomes a user name in the logs.
func TestPeerKey_MisplacedInIdentityHeader(t *testing.T) {
	logs := logtest.Capture(t, slog.LevelDebug)
	f := newPeerFixture(t, ChainResolvers(HeaderPrincipalResolver("X-Forwarded-User")), nil)
	h := f.app.NewRouter()
	rec := f.do(t, h, http.MethodGet, "/api/v1/tickets", "", map[string]string{"X-Forwarded-User": f.key})
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("key in the identity header: %d %s, want 401", rec.Code, rec.Body)
	}
	if strings.Contains(logs(), f.key) || strings.Contains(rec.Body.String(), f.key) {
		t.Error("the misplaced key was logged or echoed")
	}
}
