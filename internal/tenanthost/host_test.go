package tenanthost_test

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/dataentry"
	"github.com/Sourcehaven-BV/rela/internal/tenant"
	"github.com/Sourcehaven-BV/rela/internal/tenanthost"
)

const header = "X-Auth-Assertion"

// fakeVerifier accepts "tok-<org>" as a subject in that org, "tok-noorg" as
// a subject with no org, and rejects everything else.
type fakeVerifier struct{ calls atomic.Int64 }

func (v *fakeVerifier) VerifyAssertion(_ context.Context, raw string) (dataentry.AssertedIdentity, error) {
	v.calls.Add(1)
	switch {
	case raw == "tok-noorg":
		return dataentry.AssertedIdentity{Subject: "alice"}, nil
	case strings.HasPrefix(raw, "tok-"):
		return dataentry.AssertedIdentity{Subject: "alice", OrgID: strings.TrimPrefix(raw, "tok-")}, nil
	}
	return dataentry.AssertedIdentity{}, errors.New("bad signature")
}

// fakeWebhook accepts "hook-<org>" bodies, with "hook-" meaning no org.
type fakeWebhook struct{}

func (fakeWebhook) VerifyWebhook(_ context.Context, raw string) (dataentry.WebhookClaims, error) {
	if !strings.HasPrefix(raw, "hook-") {
		return dataentry.WebhookClaims{}, errors.New("bad signature")
	}
	return dataentry.WebhookClaims{Event: "membership.created", OrgID: strings.TrimPrefix(raw, "hook-")}, nil
}

// registry is a fake tenant registry. Every tenant's handler writes its org
// and, when present, the request body, so a test can see where a request
// landed. It counts acquisitions and how many leases are outstanding.
type registry struct {
	known    map[string]bool
	fail     error
	calls    atomic.Int64 // every acquire call, including refused ones
	acquired atomic.Int64 // acquire calls that returned a tenant
	held     atomic.Int64

	mu       sync.Mutex
	verifier tenanthost.AssertionVerifier
	seen     []dataentry.AssertedIdentity
}

func (g *registry) acquire(_ context.Context, orgID string) (http.Handler, func(), error) {
	g.calls.Add(1)
	if g.fail != nil {
		return nil, nil, g.fail
	}
	if !g.known[orgID] {
		return nil, nil, fmt.Errorf("org %q: %w", orgID, tenant.ErrUnknownTenant)
	}
	g.acquired.Add(1)
	g.held.Add(1)
	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if g.verifier != nil {
			raw := r.Header.Get(header)
			id, err := g.verifier.VerifyAssertion(r.Context(), raw)
			if err == nil {
				g.mu.Lock()
				g.seen = append(g.seen, id)
				g.mu.Unlock()
			}
		}
		fmt.Fprintf(w, "tenant=%s held=%d", orgID, g.held.Load())
		if r.Body != nil {
			b, _ := io.ReadAll(r.Body)
			if len(b) > 0 {
				fmt.Fprintf(w, " body=%s", b)
			}
		}
	})
	var once sync.Once
	return h, func() { once.Do(func() { g.held.Add(-1) }) }, nil
}

var static = http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
	_, _ = io.WriteString(w, "static shell")
})

func newHost(t *testing.T, g *registry, v tenanthost.AssertionVerifier) *tenanthost.Host {
	t.Helper()
	h, err := tenanthost.New(tenanthost.Config{
		Verifier: v, HeaderName: header, Acquire: g.acquire, Static: static, Webhook: fakeWebhook{},
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return h
}

func do(h http.Handler, method, target, token, body string) *httptest.ResponseRecorder {
	var rd io.Reader
	if body != "" {
		rd = strings.NewReader(body)
	}
	r := httptest.NewRequest(method, target, rd)
	if token != "" {
		r.Header.Set(header, token)
	}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	return w
}

func TestNew_RejectsMissingCollaborators(t *testing.T) {
	t.Parallel()
	g := &registry{}
	full := tenanthost.Config{Verifier: &fakeVerifier{}, HeaderName: header, Acquire: g.acquire, Static: static}
	cases := map[string]func(c *tenanthost.Config){
		"verifier": func(c *tenanthost.Config) { c.Verifier = nil },
		"header":   func(c *tenanthost.Config) { c.HeaderName = "" },
		"acquire":  func(c *tenanthost.Config) { c.Acquire = nil },
		"static":   func(c *tenanthost.Config) { c.Static = nil },
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			c := full
			mutate(&c)
			if _, err := tenanthost.New(c); err == nil {
				t.Errorf("New accepted a config without %s", name)
			}
		})
	}
}

// TestHost_FailsClosed is one case per denial path. Each must answer its
// status and open no tenant.
func TestHost_FailsClosed(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name   string
		token  string
		fail   error
		path   string
		status int
		code   string
		// asked is whether the host may ask the registry at all: only a
		// verified org reaches it.
		asked bool
	}{
		{name: "no assertion", path: "/api/v1/tickets", status: http.StatusUnauthorized, code: "unauthenticated"},
		{name: "invalid assertion", token: "forged", path: "/api/v1/tickets",
			status: http.StatusUnauthorized, code: "unauthenticated"},
		{name: "verified without org", token: "tok-noorg", path: "/api/v1/tickets",
			status: http.StatusForbidden, code: "unknown_tenant"},
		{name: "unknown org", token: "tok-org-z", path: "/api/v1/tickets",
			status: http.StatusForbidden, code: "unknown_tenant", asked: true},
		{name: "open failure", token: "tok-org-a", fail: errors.New("connection refused"), path: "/api/v1/tickets",
			status: http.StatusServiceUnavailable, code: "tenant_unavailable", asked: true},
		{name: "registry at capacity", token: "tok-org-a", fail: tenant.ErrAtCapacity, path: "/api/v1/tickets",
			status: http.StatusServiceUnavailable, code: "tenant_unavailable", asked: true},
		{name: "declarative hook without assertion", path: "/hooks/deploy",
			status: http.StatusUnauthorized, code: "unauthenticated"},
		{name: "custom assets without assertion", path: "/_custom/custom.css",
			status: http.StatusUnauthorized, code: "unauthenticated"},
		{name: "bare /api without assertion", path: "/api", status: http.StatusUnauthorized, code: "unauthenticated"},
		{name: "dot segments reaching the api", path: "/static/../api/v1/tickets",
			status: http.StatusUnauthorized, code: "unauthenticated"},
		{name: "double slash reaching the api", path: "//api/v1/tickets",
			status: http.StatusUnauthorized, code: "unauthenticated"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			g := &registry{known: map[string]bool{"org-a": true}, fail: tc.fail}
			h := newHost(t, g, &fakeVerifier{})
			r := httptest.NewRequest(http.MethodGet, "/", http.NoBody)
			r.URL.Path = tc.path // set directly, so dot segments reach the host unresolved
			if tc.token != "" {
				r.Header.Set(header, tc.token)
			}
			w := httptest.NewRecorder()
			h.ServeHTTP(w, r)
			if w.Code != tc.status {
				t.Fatalf("status = %d, want %d (body %q)", w.Code, tc.status, w.Body.String())
			}
			if !strings.Contains(w.Body.String(), "/errors/"+tc.code) {
				t.Errorf("body %q does not name %s", w.Body.String(), tc.code)
			}
			if strings.Contains(w.Body.String(), "tenant=") {
				t.Error("a denied request reached a tenant")
			}
			if n := g.acquired.Load(); n != 0 {
				t.Errorf("a denial acquired %d tenants, want 0", n)
			}
			if !tc.asked && g.calls.Load() != 0 {
				t.Errorf("the registry was asked %d times without a verified org", g.calls.Load())
			}
		})
	}
}

// TestHost_RetryAfterOnUnavailable pins that the 503 tells a client when to
// retry.
func TestHost_RetryAfterOnUnavailable(t *testing.T) {
	t.Parallel()
	g := &registry{fail: errors.New("boom")}
	w := do(newHost(t, g, &fakeVerifier{}), http.MethodGet, "/api/v1/tickets", "tok-org-a", "")
	if w.Header().Get("Retry-After") == "" {
		t.Error("503 carries no Retry-After")
	}
}

func TestHost_WWWAuthenticateOnAuthorizationHeader(t *testing.T) {
	t.Parallel()
	g := &registry{}
	h, err := tenanthost.New(tenanthost.Config{
		Verifier: &fakeVerifier{}, HeaderName: "Authorization", Acquire: g.acquire, Static: static,
	})
	if err != nil {
		t.Fatal(err)
	}
	w := do(h, http.MethodGet, "/api/v1/tickets", "", "")
	if w.Header().Get("WWW-Authenticate") != "Bearer" {
		t.Errorf("WWW-Authenticate = %q, want Bearer", w.Header().Get("WWW-Authenticate"))
	}
}

// TestHost_ShellWithoutTenant pins the one tenant-free surface: the shell
// and its assets load without an assertion, and without a valid one, and
// open no tenant.
func TestHost_ShellWithoutTenant(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct{ path, token string }{
		{"/", ""}, {"/static/app.js", ""}, {"/tickets/TKT-1", ""}, {"/", "forged"}, {"/assets/x.css", "tok-noorg"}, {"/", "tok-org-z"},
	} {
		g := &registry{known: map[string]bool{"org-a": true}}
		w := do(newHost(t, g, &fakeVerifier{}), http.MethodGet, tc.path, tc.token, "")
		if w.Code != http.StatusOK || w.Body.String() != "static shell" {
			t.Errorf("%s (token %q): got %d %q, want the static shell", tc.path, tc.token, w.Code, w.Body.String())
		}
		if g.acquired.Load() != 0 {
			t.Errorf("%s: the shell acquired a tenant", tc.path)
		}
		if tc.token != "tok-org-z" && g.calls.Load() != 0 {
			t.Errorf("%s: the registry was asked without a verified org", tc.path)
		}
	}
}

// TestHost_RoutesByVerifiedOrg pins that each org reaches its own tenant and
// that the lease is held for the whole request and released after it.
func TestHost_RoutesByVerifiedOrg(t *testing.T) {
	t.Parallel()
	g := &registry{known: map[string]bool{"org-a": true, "org-b": true}}
	h := newHost(t, g, &fakeVerifier{})
	for _, org := range []string{"org-a", "org-b"} {
		w := do(h, http.MethodGet, "/api/v1/tickets", "Bearer tok-"+org, "")
		if w.Code != http.StatusOK {
			t.Fatalf("%s: status %d", org, w.Code)
		}
		if want := "tenant=" + org + " held=1"; w.Body.String() != want {
			t.Errorf("%s: body %q, want %q", org, w.Body.String(), want)
		}
	}
	if g.held.Load() != 0 {
		t.Errorf("%d leases still held after the requests ended", g.held.Load())
	}
}

// TestHost_TenantGateReusesVerification pins the memo: the tenant gate gets
// the host's identity without a second verification, and only for the exact
// token the host verified.
func TestHost_TenantGateReusesVerification(t *testing.T) {
	t.Parallel()
	v := &fakeVerifier{}
	g := &registry{known: map[string]bool{"org-a": true}, verifier: tenanthost.ReuseAssertion(v)}
	w := do(newHost(t, g, v), http.MethodGet, "/api/v1/tickets", "tok-org-a", "")
	if w.Code != http.StatusOK {
		t.Fatalf("status %d", w.Code)
	}
	if n := v.calls.Load(); n != 1 {
		t.Errorf("assertion verified %d times, want 1", n)
	}
	if len(g.seen) != 1 || g.seen[0].OrgID != "org-a" {
		t.Errorf("tenant gate saw %+v, want org-a", g.seen)
	}
}

// TestReuseAssertion_OnlyForTheVerifiedToken pins that the memo answers only
// for the token the host verified on this request: a different token under
// the same ctx is verified for real.
func TestReuseAssertion_OnlyForTheVerifiedToken(t *testing.T) {
	t.Parallel()
	v := &fakeVerifier{}
	reuse := tenanthost.ReuseAssertion(v)
	var got error
	inner := http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		_, got = reuse.VerifyAssertion(r.Context(), "forged")
	})
	h, err := tenanthost.New(tenanthost.Config{
		Verifier: v, HeaderName: header, Static: static,
		Acquire: func(context.Context, string) (http.Handler, func(), error) { return inner, func() {}, nil },
	})
	if err != nil {
		t.Fatal(err)
	}
	if w := do(h, http.MethodGet, "/api/v1/tickets", "tok-org-a", ""); w.Code != http.StatusOK {
		t.Fatalf("status %d", w.Code)
	}
	if got == nil {
		t.Error("ReuseAssertion accepted a token nobody verified, under a ctx carrying the memo")
	}
	if n := v.calls.Load(); n != 2 {
		t.Errorf("verifier called %d times, want 2 (the host's token, then the other one)", n)
	}
}

// TestForOrg_RefusesOtherOrgs pins the tenant's own check: a tenant app
// accepts only identities and webhooks of its own org, whatever routed them
// there.
func TestForOrg_RefusesOtherOrgs(t *testing.T) {
	t.Parallel()
	v := tenanthost.ForOrg(&fakeVerifier{}, "org-a")
	if _, err := v.VerifyAssertion(t.Context(), "tok-org-a"); err != nil {
		t.Errorf("own org refused: %v", err)
	}
	for _, raw := range []string{"tok-org-b", "tok-noorg"} {
		if _, err := v.VerifyAssertion(t.Context(), raw); err == nil {
			t.Errorf("%s accepted by org-a's tenant", raw)
		}
	}
	wv := tenanthost.WebhookForOrg(fakeWebhook{}, "org-a")
	if _, err := wv.VerifyWebhook(t.Context(), "hook-org-a"); err != nil {
		t.Errorf("own org's webhook refused: %v", err)
	}
	for _, raw := range []string{"hook-org-b", "hook-"} {
		if _, err := wv.VerifyWebhook(t.Context(), raw); err == nil {
			t.Errorf("webhook %q accepted by org-a's tenant", raw)
		}
	}
}

func TestHost_Webhook(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		body     string
		fail     error
		status   int
		want     string
		acquired int64
	}{
		{name: "known org is forwarded with its body", body: "hook-org-a", status: http.StatusOK,
			want: "tenant=org-a held=1 body=hook-org-a", acquired: 1},
		{name: "bad signature", body: "forged", status: http.StatusUnauthorized},
		{name: "no org", body: "hook-", status: http.StatusBadRequest},
		{name: "unknown org is acknowledged", body: "hook-org-z", status: http.StatusOK, want: ""},
		{name: "open failure", body: "hook-org-a", fail: errors.New("boom"), status: http.StatusBadGateway},
		// A suspended tenant's events must be delivered again, not dropped
		// (G-032): a role removal sent now applies when it is resumed.
		{name: "suspended tenant asks for a retry", body: "hook-org-a",
			fail:   fmt.Errorf("%w: %w", tenant.ErrUnknownTenant, tenant.ErrTenantInactive),
			status: http.StatusServiceUnavailable},
		{name: "oversized body", body: "hook-org-a" + strings.Repeat("x", 64<<10), status: http.StatusRequestEntityTooLarge},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			g := &registry{known: map[string]bool{"org-a": true}, fail: tc.fail}
			w := do(newHost(t, g, &fakeVerifier{}), http.MethodPost, tenanthost.WebhookPath, "", tc.body)
			if w.Code != tc.status {
				t.Fatalf("status %d, want %d (body %q)", w.Code, tc.status, w.Body.String())
			}
			if tc.status == http.StatusOK && w.Body.String() != tc.want {
				t.Errorf("body %q, want %q", w.Body.String(), tc.want)
			}
			if g.acquired.Load() != tc.acquired {
				t.Errorf("acquired %d tenants, want %d", g.acquired.Load(), tc.acquired)
			}
		})
	}
}
