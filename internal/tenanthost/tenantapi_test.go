package tenanthost_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/tenant"
	"github.com/Sourcehaven-BV/rela/internal/tenanthost"
)

// apiVerifier accepts "api:<org>" tokens and names <org> in the claims.
type apiVerifier struct{}

func (apiVerifier) VerifyTenantAPI(_ context.Context, raw string) (tenanthost.TenantAPIClaims, error) {
	org, ok := strings.CutPrefix(raw, "api:")
	if !ok {
		return tenanthost.TenantAPIClaims{}, errors.New("bad signature")
	}
	return tenanthost.TenantAPIClaims{OrgID: org, ID: "jti-api"}, nil
}

func newAPIHost(t *testing.T, c *controlTable, p tenanthost.Provisioner, mode tenanthost.ProvisionMode) *tenanthost.Host {
	t.Helper()
	h, err := tenanthost.New(tenanthost.Config{
		Verifier: &fakeVerifier{}, HeaderName: header, Acquire: c.acquire, Static: static,
		Provision: tenanthost.ProvisionConfig{Mode: mode, Provisioner: p},
		TenantAPI: apiVerifier{},
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	t.Cleanup(h.Close)
	return h
}

func callAPI(h http.Handler, method, org, token string) *httptest.ResponseRecorder {
	r := httptest.NewRequest(method, tenanthost.TenantsPath+org, http.NoBody)
	if token != "" {
		r.Header.Set("Authorization", "Bearer "+token)
	}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	return w
}

// TestTenantAPI_Auth pins who may call the tenant API: only a verified
// token that names the org in the path. An end user's assertion is not a
// tenant API token (TKT-84CD0PY).
func TestTenantAPI_Auth(t *testing.T) {
	t.Parallel()
	c := newControl(nil)
	p := &stubProvisioner{control: c}
	h := newAPIHost(t, c, p, tenanthost.ProvisionWebhook)

	cases := []struct {
		name, method, org, token string
		code                     int
	}{
		{"no token", http.MethodPut, "org-a", "", http.StatusUnauthorized},
		{"bad token", http.MethodPut, "org-a", "forged", http.StatusUnauthorized},
		{"user assertion", http.MethodPut, "org-a", "org-a|user", http.StatusUnauthorized},
		{"other org", http.MethodPut, "org-a", "api:org-b", http.StatusForbidden},
		{"other org read", http.MethodGet, "org-a", "api:org-b", http.StatusForbidden},
		// The bare prefix is not the API, so it needs an assertion.
		{"no org", http.MethodGet, "", "api:", http.StatusUnauthorized},
		{"nested path", http.MethodGet, "org-a/x", "api:org-a/x", http.StatusNotFound},
		{"method", http.MethodDelete, "org-a", "api:org-a", http.StatusMethodNotAllowed},
	}
	for _, tc := range cases {
		w := callAPI(h, tc.method, tc.org, tc.token)
		if w.Code != tc.code {
			t.Errorf("%s: %d %s, want %d", tc.name, w.Code, w.Body, tc.code)
		}
	}
	if n := p.calls.Load(); n != 0 {
		t.Errorf("refused calls ran the provisioner %d times", n)
	}
	if got := callAPI(h, http.MethodPut, "org-a", "").Header().Get("WWW-Authenticate"); got != "Bearer" {
		t.Errorf("WWW-Authenticate = %q, want Bearer", got)
	}
}

// TestTenantAPI_CreateThenRead creates a tenant and reads it back.
func TestTenantAPI_CreateThenRead(t *testing.T) {
	t.Parallel()
	c := newControl(nil)
	p := &stubProvisioner{control: c}
	h := newAPIHost(t, c, p, tenanthost.ProvisionWebhook)

	if w := callAPI(h, http.MethodGet, "org-a", "api:org-a"); w.Code != http.StatusNotFound {
		t.Fatalf("GET before create: %d %s, want 404", w.Code, w.Body)
	}
	w := callAPI(h, http.MethodPut, "org-a", "api:org-a")
	if w.Code != http.StatusCreated || statusOf(t, w.Body.String()) != "active" {
		t.Fatalf("PUT: %d %s, want 201 active", w.Code, w.Body)
	}
	if !strings.Contains(w.Body.String(), `"org_id":"org-a"`) {
		t.Errorf("PUT body %s lacks the org id", w.Body)
	}
	w = callAPI(h, http.MethodPut, "org-a", "api:org-a")
	if w.Code != http.StatusOK || statusOf(t, w.Body.String()) != "active" {
		t.Errorf("second PUT: %d %s, want 200 active", w.Code, w.Body)
	}
	w = callAPI(h, http.MethodGet, "org-a", "api:org-a")
	if w.Code != http.StatusOK || statusOf(t, w.Body.String()) != "active" {
		t.Errorf("GET: %d %s, want 200 active", w.Code, w.Body)
	}
	if n := p.calls.Load(); n != 1 {
		t.Errorf("provisioner ran %d times, want 1", n)
	}
}

// TestTenantAPI_SlowCreateIsAccepted answers 202 with a Location while the
// create runs, and the status reads provisioning until it ends.
func TestTenantAPI_SlowCreateIsAccepted(t *testing.T) {
	t.Parallel()
	c := newControl(nil)
	p := &stubProvisioner{control: c, gate: make(chan struct{})}
	h, err := tenanthost.New(tenanthost.Config{
		Verifier: &fakeVerifier{}, HeaderName: header, Acquire: c.acquire, Static: static,
		Provision: tenanthost.ProvisionConfig{
			Mode: tenanthost.ProvisionWebhook, Provisioner: p, WebhookWait: 1,
		},
		TenantAPI: apiVerifier{},
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(h.Close)

	w := callAPI(h, http.MethodPut, "org-a", "api:org-a")
	if w.Code != http.StatusAccepted || statusOf(t, w.Body.String()) != "provisioning" {
		t.Fatalf("PUT: %d %s, want 202 provisioning", w.Code, w.Body)
	}
	if loc := w.Header().Get("Location"); loc != tenanthost.TenantsPath+"org-a" {
		t.Errorf("Location = %q", loc)
	}
	if w.Header().Get("Retry-After") == "" {
		t.Error("202 without Retry-After")
	}
	w = callAPI(h, http.MethodGet, "org-a", "api:org-a")
	if w.Code != http.StatusOK || statusOf(t, w.Body.String()) != "provisioning" {
		t.Errorf("GET while running: %d %s, want provisioning", w.Code, w.Body)
	}
	close(p.gate)
	waitFor(t, func() bool {
		return statusOf(t, callAPI(h, http.MethodGet, "org-a", "api:org-a").Body.String()) == "active"
	})
}

// TestTenantAPI_States reports and refuses by the tenant's status.
func TestTenantAPI_States(t *testing.T) {
	t.Parallel()
	c := newControl(map[string]tenant.Status{
		"org-s": tenant.StatusSuspended,
		"org-e": tenant.StatusErased,
		"org-p": tenant.StatusProvisioning,
	})
	p := &stubProvisioner{control: c}
	h := newAPIHost(t, c, p, tenanthost.ProvisionLazy)

	reads := map[string]string{"org-s": "suspended", "org-e": "erased", "org-p": "provisioning"}
	for org, want := range reads {
		w := callAPI(h, http.MethodGet, org, "api:"+org)
		if w.Code != http.StatusOK || statusOf(t, w.Body.String()) != want {
			t.Errorf("GET %s: %d %s, want %s", org, w.Code, w.Body, want)
		}
	}
	for org, code := range map[string]string{"org-s": "tenant_suspended", "org-e": "tenant_erased"} {
		w := callAPI(h, http.MethodPut, org, "api:"+org)
		if w.Code != http.StatusConflict || !strings.Contains(w.Body.String(), code) {
			t.Errorf("PUT %s: %d %s, want 409 %s", org, w.Code, w.Body, code)
		}
	}
	// A create that stopped part way is finished.
	if w := callAPI(h, http.MethodPut, "org-p", "api:org-p"); w.Code != http.StatusCreated {
		t.Errorf("PUT org-p: %d %s, want 201", w.Code, w.Body)
	}
	if n := p.calls.Load(); n != 1 {
		t.Errorf("provisioner ran %d times, want 1 (org-p only)", n)
	}
}

// TestTenantAPI_Failures maps a failed or refused create to a problem.
func TestTenantAPI_Failures(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		err  error
		code int
		want string
	}{
		{"failed", errors.New("boom"), http.StatusServiceUnavailable, "provisioning_failed"},
		{"needs operator", tenanthost.ErrNeedsOperator, http.StatusServiceUnavailable, "provisioning_needs_operator"},
		{"exists", tenant.ErrTenantExists, http.StatusConflict, "tenant_suspended"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			c := newControl(nil)
			p := &stubProvisioner{control: c, err: tc.err}
			h := newAPIHost(t, c, p, tenanthost.ProvisionWebhook)
			w := callAPI(h, http.MethodPut, "org-a", "api:org-a")
			if w.Code != tc.code || !strings.Contains(w.Body.String(), tc.want) {
				t.Errorf("PUT: %d %s, want %d %s", w.Code, w.Body, tc.code, tc.want)
			}
			// A retry inside the hold starts no run.
			w = callAPI(h, http.MethodPut, "org-a", "api:org-a")
			if !errors.Is(tc.err, tenant.ErrTenantExists) && w.Code != http.StatusServiceUnavailable {
				t.Errorf("retry: %d %s, want 503", w.Code, w.Body)
			}
		})
	}
}

// TestTenantAPI_InvalidOrgNeverReachesProvisioner refuses an org id that
// cannot be a tenant before any lookup.
func TestTenantAPI_InvalidOrgNeverReachesProvisioner(t *testing.T) {
	t.Parallel()
	c := newControl(nil)
	p := &stubProvisioner{control: c}
	h := newAPIHost(t, c, p, tenanthost.ProvisionWebhook)
	bad := strings.Repeat("x", 300)
	if w := callAPI(h, http.MethodPut, bad, "api:"+bad); w.Code != http.StatusBadRequest {
		t.Errorf("PUT: %d %s, want 400", w.Code, w.Body)
	}
	if c.calls.Load() != 0 || p.calls.Load() != 0 {
		t.Error("an invalid org id reached the lookup or the provisioner")
	}
}

// TestTenantAPI_Config pins the startup checks: the API needs provisioning,
// and webhook mode may run on the API alone.
func TestTenantAPI_Config(t *testing.T) {
	t.Parallel()
	c := newControl(nil)
	base := tenanthost.Config{
		Verifier: &fakeVerifier{}, HeaderName: header, Acquire: c.acquire, Static: static,
		TenantAPI: apiVerifier{},
	}
	if _, err := tenanthost.New(base); err == nil {
		t.Error("tenant API with provisioning off: New accepted it")
	}
	base.Provision = tenanthost.ProvisionConfig{
		Mode: tenanthost.ProvisionWebhook, Provisioner: &stubProvisioner{control: c},
	}
	h, err := tenanthost.New(base)
	if err != nil {
		t.Fatalf("webhook mode with the API alone: %v", err)
	}
	h.Close()
}

// TestTenantAPI_OffIsAnOrdinaryPath leaves the path to the tenant when the
// API is not configured.
func TestTenantAPI_OffIsAnOrdinaryPath(t *testing.T) {
	t.Parallel()
	c := newControl(map[string]tenant.Status{"org-a": tenant.StatusActive})
	h := newProvHost(t, c, tenanthost.ProvisionConfig{})
	if w := callAPI(h, http.MethodPut, "org-a", "api:org-a"); w.Code != http.StatusUnauthorized {
		t.Errorf("PUT with the API off: %d %s, want 401 (no assertion)", w.Code, w.Body)
	}
}
