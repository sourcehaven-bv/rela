package tenanthost_test

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/dataentry"
	"github.com/Sourcehaven-BV/rela/internal/tenant"
	"github.com/Sourcehaven-BV/rela/internal/tenanthost"
)

// apiRole is the role the tests configure for the tenant API.
const apiRole = "tenant-manager"

// opToken is the assertion of an operator principal holding the API role.
const opToken = "op:" + apiRole

// opVerifier accepts the forms below and passes anything else to
// [fakeVerifier]:
//
//   - "op:<role>": an operator principal with that role and no org;
//   - "op-org:<role>": the same principal, but with an org;
//   - "user:<role>": a user principal with that role and no org.
type opVerifier struct{ fakeVerifier }

func (v *opVerifier) VerifyAssertion(ctx context.Context, raw string) (dataentry.AssertedIdentity, error) {
	if role, ok := strings.CutPrefix(raw, "op:"); ok {
		return dataentry.AssertedIdentity{Subject: "harbourmaster", PrincipalType: "operator", Roles: []string{role}}, nil
	}
	if role, ok := strings.CutPrefix(raw, "op-org:"); ok {
		return dataentry.AssertedIdentity{Subject: "harbourmaster", PrincipalType: "operator",
			Roles: []string{role}, OrgID: "org-a"}, nil
	}
	if role, ok := strings.CutPrefix(raw, "user:"); ok {
		return dataentry.AssertedIdentity{Subject: "alice", PrincipalType: "user", Roles: []string{role}}, nil
	}
	return v.fakeVerifier.VerifyAssertion(ctx, raw)
}

// apiControl is a fake [tenanthost.TenantControl] over a controlTable. It
// keeps the plan fields per org, and applies changes by a simplified form
// of [tenant.Provisioner.Apply]'s rules.
type apiControl struct {
	c        *controlTable
	mu       sync.Mutex
	extra    map[string]tenant.Record
	applyErr error
	usageErr error
	actors   []string
}

func newAPIControl(c *controlTable) *apiControl {
	return &apiControl{c: c, extra: map[string]tenant.Record{}}
}

func (a *apiControl) Lookup(_ context.Context, orgID string) (tenant.Record, error) {
	a.c.mu.Lock()
	s, ok := a.c.status[orgID]
	a.c.mu.Unlock()
	if !ok {
		return tenant.Record{}, errors.Join(tenant.ErrUnknownTenant, tenant.ErrTenantAbsent)
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	rec := a.extra[orgID]
	rec.OrgID, rec.Status, rec.SchemaExists = orgID, s, true
	return rec, nil
}

func (a *apiControl) Apply(ctx context.Context, orgID, actor string, c tenant.Change) (tenant.Record, error) {
	rec, err := a.Lookup(ctx, orgID)
	if err != nil {
		return tenant.Record{}, err
	}
	a.mu.Lock()
	a.actors = append(a.actors, actor)
	if a.applyErr != nil {
		a.mu.Unlock()
		return tenant.Record{}, a.applyErr
	}
	if c.Plan != nil {
		rec.Plan = *c.Plan
	}
	if c.Entitlements != nil {
		rec.Entitlements = *c.Entitlements
	}
	if c.State != nil {
		rec.ReadOnly = *c.State == tenant.StateReadOnly
	}
	a.extra[orgID] = rec
	a.mu.Unlock()
	if c.State != nil {
		s := tenant.StatusActive
		if *c.State == tenant.StateSuspended {
			s = tenant.StatusSuspended
		}
		a.c.set(orgID, s)
	}
	return a.Lookup(ctx, orgID)
}

func (a *apiControl) Archive(ctx context.Context, orgID, actor string) (tenant.Record, error) {
	if _, err := a.Lookup(ctx, orgID); err != nil {
		return tenant.Record{}, err
	}
	a.mu.Lock()
	a.actors = append(a.actors, actor)
	rec := a.extra[orgID]
	rec.SuspendedBy = tenant.SuspendedByArchive
	rec.ArchivedAt = time.Date(2026, 11, 21, 12, 0, 0, 0, time.UTC)
	a.extra[orgID] = rec
	a.mu.Unlock()
	a.c.set(orgID, tenant.StatusSuspended)
	return a.Lookup(ctx, orgID)
}

func (a *apiControl) Usage(ctx context.Context, orgID string) (tenant.Usage, error) {
	if _, err := a.Lookup(ctx, orgID); err != nil {
		return tenant.Usage{}, err
	}
	if a.usageErr != nil {
		return tenant.Usage{}, a.usageErr
	}
	return tenant.Usage{Users: 3, ActiveUsers: 2, StorageBytes: 4096,
		MeasuredAt: time.Date(2026, 11, 21, 12, 0, 0, 0, time.UTC)}, nil
}

func apiConfig(c *controlTable, p tenanthost.Provisioner, mode tenanthost.ProvisionMode) (tenanthost.Config, *apiControl) {
	ctl := newAPIControl(c)
	return tenanthost.Config{
		Verifier: &opVerifier{}, HeaderName: header, Acquire: c.acquire, Static: static,
		Provision: tenanthost.ProvisionConfig{Mode: mode, Provisioner: p},
		TenantAPI: &tenanthost.TenantAPIConfig{Role: apiRole, Control: ctl},
	}, ctl
}

func newAPIHost(t *testing.T, c *controlTable, p tenanthost.Provisioner,
	mode tenanthost.ProvisionMode,
) (*tenanthost.Host, *apiControl) {
	t.Helper()
	cfg, ctl := apiConfig(c, p, mode)
	h, err := tenanthost.New(cfg)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	t.Cleanup(h.Close)
	return h, ctl
}

func callAPI(h http.Handler, method, org, token string) *httptest.ResponseRecorder {
	return callAPIBody(h, method, tenanthost.TenantsPath+org, token, "")
}

func callAPIBody(h http.Handler, method, path, token, body string) *httptest.ResponseRecorder {
	var rd io.Reader = http.NoBody
	if body != "" {
		rd = strings.NewReader(body)
	}
	r := httptest.NewRequest(method, path, rd)
	if token != "" {
		r.Header.Set(header, "Bearer "+token)
	}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	return w
}

// apiBody is the tenant API's answer about one tenant.
type apiBody struct {
	OrgID        string         `json:"org_id"`
	State        string         `json:"state"`
	SuspendedBy  string         `json:"suspended_by"`
	ReadOnly     bool           `json:"read_only"`
	Plan         string         `json:"plan"`
	Entitlements map[string]any `json:"entitlements"`
	ArchivedAt   *time.Time     `json:"archived_at"`
}

func bodyOf(t *testing.T, w *httptest.ResponseRecorder) apiBody {
	t.Helper()
	var b apiBody
	if err := json.Unmarshal(w.Body.Bytes(), &b); err != nil {
		t.Fatalf("body %q is not a tenant: %v", w.Body, err)
	}
	return b
}

func stateOf(t *testing.T, w *httptest.ResponseRecorder) string {
	t.Helper()
	return bodyOf(t, w).State
}

// TestTenantAPI_Auth pins who may call the tenant API: only a verified
// operator principal with no org that holds the configured role. An end
// user's assertion is refused, and so is an operator without the role.
func TestTenantAPI_Auth(t *testing.T) {
	t.Parallel()
	c := newControl(map[string]tenant.Status{"org-a": tenant.StatusActive})
	p := &stubProvisioner{control: c}
	h, _ := newAPIHost(t, c, p, tenanthost.ProvisionWebhook)

	cases := []struct {
		name, method, org, token string
		code                     int
	}{
		{"no token", http.MethodPut, "org-b", "", http.StatusUnauthorized},
		{"bad token", http.MethodPut, "org-b", "forged", http.StatusUnauthorized},
		{"user of the org", http.MethodPut, "org-b", "tok-org-b", http.StatusForbidden},
		{"user without org", http.MethodGet, "org-a", "tok-noorg", http.StatusForbidden},
		{"user with the role", http.MethodGet, "org-a", "user:" + apiRole, http.StatusForbidden},
		{"operator without the role", http.MethodGet, "org-a", "op:viewer", http.StatusForbidden},
		{"operator with an org", http.MethodGet, "org-a", "op-org:" + apiRole, http.StatusForbidden},
		{"operator usage without the role", http.MethodGet, "org-a/usage", "op:viewer", http.StatusForbidden},
		// The bare prefix is not the API, so it needs an end-user assertion.
		{"no org", http.MethodGet, "", opToken, http.StatusForbidden},
		{"nested path", http.MethodGet, "org-a/x", opToken, http.StatusNotFound},
		{"method", http.MethodPost, "org-a", opToken, http.StatusMethodNotAllowed},
		{"usage method", http.MethodPut, "org-a/usage", opToken, http.StatusMethodNotAllowed},
		{"operator", http.MethodGet, "org-a", opToken, http.StatusOK},
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
	if got := callAPI(h, http.MethodPut, "org-a", "").Code; got != http.StatusUnauthorized {
		t.Errorf("no token: %d", got)
	}
}

// TestTenantAPI_OperatorRefusedElsewhere pins that an operator assertion,
// which carries no org, reaches no tenant on any other path.
func TestTenantAPI_OperatorRefusedElsewhere(t *testing.T) {
	t.Parallel()
	c := newControl(map[string]tenant.Status{"org-a": tenant.StatusActive})
	h, _ := newAPIHost(t, c, &stubProvisioner{control: c}, tenanthost.ProvisionWebhook)
	for _, path := range []string{"/api/v1/tickets", tenanthost.StatusPath, "/api/v1/_tenants"} {
		w := callAPIBody(h, http.MethodGet, path, opToken, "")
		if w.Code != http.StatusForbidden || !strings.Contains(w.Body.String(), "unknown_tenant") {
			t.Errorf("GET %s as operator: %d %s, want 403 unknown_tenant", path, w.Code, w.Body)
		}
	}
	for _, path := range []string{"/api/v1/tickets", tenanthost.StatusPath} {
		w := callAPIBody(h, http.MethodGet, path, "op-org:"+apiRole, "")
		if w.Code != http.StatusForbidden || !strings.Contains(w.Body.String(), "unknown_tenant") {
			t.Errorf("GET %s as operator with an org: %d %s, want 403 unknown_tenant", path, w.Code, w.Body)
		}
	}
	if n := c.calls.Load(); n != 0 {
		t.Errorf("an operator request acquired a tenant %d times", n)
	}
}

// TestTenantAPI_CreateThenRead creates a tenant and reads it back.
func TestTenantAPI_CreateThenRead(t *testing.T) {
	t.Parallel()
	c := newControl(nil)
	p := &stubProvisioner{control: c}
	h, _ := newAPIHost(t, c, p, tenanthost.ProvisionWebhook)

	if w := callAPI(h, http.MethodGet, "org-a", opToken); w.Code != http.StatusNotFound {
		t.Fatalf("GET before create: %d %s, want 404", w.Code, w.Body)
	}
	w := callAPI(h, http.MethodPut, "org-a", opToken)
	if w.Code != http.StatusCreated || stateOf(t, w) != "active" {
		t.Fatalf("PUT: %d %s, want 201 active", w.Code, w.Body)
	}
	if b := bodyOf(t, w); b.OrgID != "org-a" || b.Entitlements == nil {
		t.Errorf("PUT body %s lacks the org id or the entitlements object", w.Body)
	}
	w = callAPI(h, http.MethodPut, "org-a", opToken)
	if w.Code != http.StatusOK || stateOf(t, w) != "active" {
		t.Errorf("second PUT: %d %s, want 200 active", w.Code, w.Body)
	}
	w = callAPI(h, http.MethodGet, "org-a", opToken)
	if w.Code != http.StatusOK || stateOf(t, w) != "active" {
		t.Errorf("GET: %d %s, want 200 active", w.Code, w.Body)
	}
	if n := p.calls.Load(); n != 1 {
		t.Errorf("provisioner ran %d times, want 1", n)
	}
}

// TestTenantAPI_SlowCreateIsAccepted answers 202 with a Location while the
// create runs, and the state reads provisioning until it ends.
func TestTenantAPI_SlowCreateIsAccepted(t *testing.T) {
	t.Parallel()
	c := newControl(nil)
	p := &stubProvisioner{control: c, gate: make(chan struct{})}
	cfg, _ := apiConfig(c, p, tenanthost.ProvisionWebhook)
	cfg.Provision.WebhookWait = 1
	h, err := tenanthost.New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(h.Close)

	w := callAPI(h, http.MethodPut, "org-a", opToken)
	if w.Code != http.StatusAccepted || stateOf(t, w) != "provisioning" {
		t.Fatalf("PUT: %d %s, want 202 provisioning", w.Code, w.Body)
	}
	if loc := w.Header().Get("Location"); loc != tenanthost.TenantsPath+"org-a" {
		t.Errorf("Location = %q", loc)
	}
	if w.Header().Get("Retry-After") == "" {
		t.Error("202 without Retry-After")
	}
	w = callAPI(h, http.MethodGet, "org-a", opToken)
	if w.Code != http.StatusOK || stateOf(t, w) != "provisioning" {
		t.Errorf("GET while running: %d %s, want provisioning", w.Code, w.Body)
	}
	close(p.gate)
	waitFor(t, func() bool {
		return stateOf(t, callAPI(h, http.MethodGet, "org-a", opToken)) == "active"
	})
}

// TestTenantAPI_States reports and refuses by the tenant's status.
func TestTenantAPI_States(t *testing.T) {
	t.Parallel()
	c := newControl(map[string]tenant.Status{
		"org-s":  tenant.StatusSuspended,
		"org-e":  tenant.StatusErased,
		"org-er": tenant.StatusErasing,
		"org-p":  tenant.StatusProvisioning,
		"org-ar": tenant.StatusSuspended,
		"org-ro": tenant.StatusActive,
	})
	p := &stubProvisioner{control: c}
	h, ctl := newAPIHost(t, c, p, tenanthost.ProvisionLazy)
	ctl.extra["org-ar"] = tenant.Record{SuspendedBy: tenant.SuspendedByArchive, ArchivedAt: time.Now()}
	ctl.extra["org-ro"] = tenant.Record{ReadOnly: true}

	reads := map[string]string{
		"org-s": "suspended", "org-e": "erased", "org-er": "erasing", "org-p": "provisioning",
		"org-ar": "archived", "org-ro": "read_only",
	}
	for org, want := range reads {
		w := callAPI(h, http.MethodGet, org, opToken)
		if w.Code != http.StatusOK || stateOf(t, w) != want {
			t.Errorf("GET %s: %d %s, want %s", org, w.Code, w.Body, want)
		}
	}
	for org, code := range map[string]string{
		"org-s": "tenant_suspended", "org-e": "tenant_erased", "org-er": "tenant_erased", "org-ar": "tenant_archived",
	} {
		w := callAPI(h, http.MethodPut, org, opToken)
		if w.Code != http.StatusConflict || !strings.Contains(w.Body.String(), code) {
			t.Errorf("PUT %s: %d %s, want 409 %s", org, w.Code, w.Body, code)
		}
	}
	// A read-only tenant exists and is served: PUT answers it as it is.
	if w := callAPI(h, http.MethodPut, "org-ro", opToken); w.Code != http.StatusOK || stateOf(t, w) != "read_only" {
		t.Errorf("PUT org-ro: %d %s, want 200 read_only", w.Code, w.Body)
	}
	// A create that stopped part way is finished.
	if w := callAPI(h, http.MethodPut, "org-p", opToken); w.Code != http.StatusCreated {
		t.Errorf("PUT org-p: %d %s, want 201", w.Code, w.Body)
	}
	if n := p.calls.Load(); n != 1 {
		t.Errorf("provisioner ran %d times, want 1 (org-p only)", n)
	}
}

// TestTenantAPI_Patch sets the state, plan and entitlements, and refuses a
// body that is not a valid change before it reaches the control table.
func TestTenantAPI_Patch(t *testing.T) {
	t.Parallel()
	c := newControl(map[string]tenant.Status{"org-a": tenant.StatusActive})
	h, ctl := newAPIHost(t, c, &stubProvisioner{control: c}, tenanthost.ProvisionWebhook)
	path := tenanthost.TenantsPath + "org-a"

	w := callAPIBody(h, http.MethodPatch, path, opToken,
		`{"state":"read_only","plan":"team","entitlements":{"max_users":25,"sso":true,"tier":"b"}}`)
	if w.Code != http.StatusOK {
		t.Fatalf("PATCH: %d %s", w.Code, w.Body)
	}
	b := bodyOf(t, w)
	if b.State != "read_only" || !b.ReadOnly || b.Plan != "team" || b.Entitlements["max_users"] != float64(25) ||
		b.Entitlements["sso"] != true {

		t.Errorf("PATCH body = %+v", b)
	}
	if len(ctl.actors) != 1 || ctl.actors[0] != "harbourmaster" {
		t.Errorf("actors = %v, want the operator's subject", ctl.actors)
	}

	if w := callAPIBody(h, http.MethodPatch, path, opToken, `{"state":"suspended"}`); stateOf(t, w) != "suspended" {
		t.Errorf("suspend: %d %s", w.Code, w.Body)
	}

	bad := map[string]string{
		"empty":            `{}`,
		"not json":         `state=active`,
		"unknown field":    `{"status":"active"}`,
		"unknown state":    `{"state":"archived"}`,
		"nested value":     `{"entitlements":{"limits":{"users":1}}}`,
		"bad name":         `{"entitlements":{"Max-Users":1}}`,
		"two values":       `{"plan":"a"}{"plan":"b"}`,
		"plan too long":    `{"plan":"` + strings.Repeat("p", 200) + `"}`,
		"body too large":   `{"plan":"` + strings.Repeat("p", 20<<10) + `"}`,
		"array value":      `{"entitlements":{"regions":["eu"]}}`,
		"null entitlement": `{"entitlements":{"x":null}}`,
		"control in plan":  `{"plan":"te\u0000am"}`,
		"control in value": `{"entitlements":{"tier":"a\nb"}}`,
		"long number":      `{"entitlements":{"max_users":1` + strings.Repeat("0", 40) + `}}`,
	}
	for name, body := range bad {
		w := callAPIBody(h, http.MethodPatch, path, opToken, body)
		if w.Code != http.StatusBadRequest || !strings.Contains(w.Body.String(), "invalid_change") {
			t.Errorf("%s: %d %s, want 400 invalid_change", name, w.Code, w.Body)
		}
	}
	if len(ctl.actors) != 2 {
		t.Errorf("a refused body reached the control table: %d applies", len(ctl.actors))
	}
}

// TestTenantAPI_PatchRefusals maps the control table's refusals.
func TestTenantAPI_PatchRefusals(t *testing.T) {
	t.Parallel()
	cases := []struct {
		err  error
		code int
		want string
	}{
		{tenant.ErrArchived, http.StatusConflict, "tenant_archived"},
		{tenant.ErrHeldSuspended, http.StatusConflict, "tenant_held"},
		{errors.Join(tenant.ErrWrongStatus, tenant.ErrTenantProvisioning), http.StatusConflict, "tenant_provisioning"},
		{tenant.ErrWrongStatus, http.StatusConflict, "wrong_status"},
		{errors.Join(tenant.ErrUnknownTenant, tenant.ErrTenantAbsent), http.StatusNotFound, "unknown_tenant"},
		{errors.New("connection refused"), http.StatusServiceUnavailable, "tenant_unavailable"},
	}
	for _, tc := range cases {
		c := newControl(map[string]tenant.Status{"org-a": tenant.StatusActive})
		h, ctl := newAPIHost(t, c, &stubProvisioner{control: c}, tenanthost.ProvisionWebhook)
		ctl.applyErr = tc.err
		w := callAPIBody(h, http.MethodPatch, tenanthost.TenantsPath+"org-a", opToken, `{"state":"active"}`)
		if w.Code != tc.code || !strings.Contains(w.Body.String(), tc.want) {
			t.Errorf("%v: %d %s, want %d %s", tc.err, w.Code, w.Body, tc.code, tc.want)
		}
		if strings.Contains(w.Body.String(), "connection refused") {
			t.Errorf("the answer names the internal error: %s", w.Body)
		}
	}
	c := newControl(nil)
	h, _ := newAPIHost(t, c, &stubProvisioner{control: c}, tenanthost.ProvisionWebhook)
	if w := callAPIBody(h, http.MethodPatch, tenanthost.TenantsPath+"org-x", opToken,
		`{"plan":"team"}`); w.Code != http.StatusNotFound {
		t.Errorf("PATCH an absent org: %d %s, want 404", w.Code, w.Body)
	}
}

// TestTenantAPI_Archive archives a tenant with DELETE; it then reads as
// archived and a create is refused.
func TestTenantAPI_Archive(t *testing.T) {
	t.Parallel()
	c := newControl(map[string]tenant.Status{"org-a": tenant.StatusActive})
	p := &stubProvisioner{control: c}
	h, _ := newAPIHost(t, c, p, tenanthost.ProvisionWebhook)

	w := callAPI(h, http.MethodDelete, "org-a", opToken)
	b := bodyOf(t, w)
	if w.Code != http.StatusOK || b.State != "archived" || b.ArchivedAt == nil || b.SuspendedBy != "archive" {
		t.Fatalf("DELETE: %d %s, want 200 archived", w.Code, w.Body)
	}
	if w := callAPI(h, http.MethodPut, "org-a", opToken); w.Code != http.StatusConflict {
		t.Errorf("PUT after archive: %d %s, want 409", w.Code, w.Body)
	}
	if w := callAPI(h, http.MethodDelete, "org-x", opToken); w.Code != http.StatusNotFound {
		t.Errorf("DELETE an absent org: %d %s, want 404", w.Code, w.Body)
	}
	if n := p.calls.Load(); n != 0 {
		t.Errorf("provisioner ran %d times", n)
	}
}

// TestTenantAPI_Usage reads a tenant's usage.
func TestTenantAPI_Usage(t *testing.T) {
	t.Parallel()
	c := newControl(map[string]tenant.Status{"org-a": tenant.StatusActive})
	h, ctl := newAPIHost(t, c, &stubProvisioner{control: c}, tenanthost.ProvisionWebhook)

	w := callAPI(h, http.MethodGet, "org-a/usage", opToken)
	want := `{"org_id":"org-a","users":3,"active_users_30d":2,"storage_bytes":4096,` +
		`"measured_at":"2026-11-21T12:00:00Z"}`
	if w.Code != http.StatusOK || strings.TrimSpace(w.Body.String()) != want {
		t.Errorf("usage: %d %s, want %s", w.Code, w.Body, want)
	}
	if w := callAPI(h, http.MethodGet, "org-x/usage", opToken); w.Code != http.StatusNotFound {
		t.Errorf("usage of an absent org: %d %s, want 404", w.Code, w.Body)
	}
	ctl.usageErr = errors.Join(tenant.ErrWrongStatus, tenant.ErrTenantProvisioning)
	if w := callAPI(h, http.MethodGet, "org-a/usage", opToken); w.Code != http.StatusConflict ||
		!strings.Contains(w.Body.String(), "tenant_provisioning") {

		t.Errorf("usage of a provisioning org: %d %s, want 409 tenant_provisioning", w.Code, w.Body)
	}
}

// TestTenantAPI_StatusShowsPlan pins that a tenant's own users read its
// plan, entitlements and read-only flag at [tenanthost.StatusPath].
func TestTenantAPI_StatusShowsPlan(t *testing.T) {
	t.Parallel()
	c := newControl(map[string]tenant.Status{"org-a": tenant.StatusActive})
	h, ctl := newAPIHost(t, c, &stubProvisioner{control: c}, tenanthost.ProvisionWebhook)
	ctl.extra["org-a"] = tenant.Record{ReadOnly: true, Plan: "team",
		Entitlements: tenant.Entitlements{"max_users": json.Number("25")}}

	w := callAPIBody(h, http.MethodGet, tenanthost.StatusPath, "tok-org-a", "")
	want := `{"status":"active","read_only":true,"plan":"team","entitlements":{"max_users":25}}`
	if w.Code != http.StatusOK || strings.TrimSpace(w.Body.String()) != want {
		t.Errorf("status: %d %s, want %s", w.Code, w.Body, want)
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
		{"exists", tenant.ErrTenantExists, http.StatusNotFound, "unknown_tenant"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			c := newControl(nil)
			p := &stubProvisioner{control: c, err: tc.err}
			h, _ := newAPIHost(t, c, p, tenanthost.ProvisionWebhook)
			w := callAPI(h, http.MethodPut, "org-a", opToken)
			if w.Code != tc.code || !strings.Contains(w.Body.String(), tc.want) {
				t.Errorf("PUT: %d %s, want %d %s", w.Code, w.Body, tc.code, tc.want)
			}
			// A retry inside the hold starts no run.
			w = callAPI(h, http.MethodPut, "org-a", opToken)
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
	h, _ := newAPIHost(t, c, p, tenanthost.ProvisionWebhook)
	bad := strings.Repeat("x", 300)
	if w := callAPI(h, http.MethodPut, bad, opToken); w.Code != http.StatusBadRequest {
		t.Errorf("PUT: %d %s, want 400", w.Code, w.Body)
	}
	if c.calls.Load() != 0 || p.calls.Load() != 0 {
		t.Error("an invalid org id reached the lookup or the provisioner")
	}
}

// TestTenantAPI_Config pins the startup checks: the API needs provisioning,
// a role and tenant control, and webhook mode may run on the API alone.
func TestTenantAPI_Config(t *testing.T) {
	t.Parallel()
	c := newControl(nil)
	base, ctl := apiConfig(c, nil, tenanthost.ProvisionOff)
	base.Provision = tenanthost.ProvisionConfig{}
	if _, err := tenanthost.New(base); err == nil {
		t.Error("tenant API with provisioning off: New accepted it")
	}
	base.Provision = tenanthost.ProvisionConfig{
		Mode: tenanthost.ProvisionWebhook, Provisioner: &stubProvisioner{control: c},
	}
	for name, api := range map[string]*tenanthost.TenantAPIConfig{
		"no role":    {Control: ctl},
		"no control": {Role: apiRole},
	} {
		cfg := base
		cfg.TenantAPI = api
		if _, err := tenanthost.New(cfg); err == nil {
			t.Errorf("%s: New accepted it", name)
		}
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
	if w := callAPI(h, http.MethodPut, "org-a", ""); w.Code != http.StatusUnauthorized {
		t.Errorf("PUT with the API off: %d %s, want 401 (no assertion)", w.Code, w.Body)
	}
}

// TestReadOnly pins which requests a read-only tenant serves.
func TestReadOnly(t *testing.T) {
	t.Parallel()
	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) })
	h := tenanthost.ReadOnly(next)
	cases := []struct {
		method, path string
		ok           bool
	}{
		{http.MethodGet, "/api/v1/tickets", true},
		{http.MethodHead, "/api/v1/tickets", true},
		{http.MethodOptions, "/api/v1/tickets", true},
		{"PROPFIND", "/dav/", true},
		{"REPORT", "/dav/cal", true},
		{http.MethodPut, "/api/v1/_events/c1/subscriptions", true},
		{http.MethodPost, tenanthost.WebhookPath, true},
		{http.MethodPost, "/api/v1/tickets", false},
		{http.MethodPut, "/api/v1/tickets/TKT-1", false},
		{http.MethodPatch, "/api/v1/tickets/TKT-1", false},
		{http.MethodDelete, "/api/v1/tickets/TKT-1", false},
		{http.MethodPost, "/api/v1/_mcp", false},
		{http.MethodPut, "/api/v1/_events/c1/other", false},
		{http.MethodPut, "/api/v1/_events//subscriptions", false},
		{http.MethodPut, "/api/v1/_events/c1/../../tickets/x", false},
		{http.MethodPost, "/webhooks/idp/../../api/v1/tickets", false},
		{http.MethodPut, "/api/v1/_events/c1%2F..%2F..%2Ftickets/subscriptions", false},
		{http.MethodPut, "/api/v1/_events/c1/subscriptions/", false},
		{http.MethodPut, "/api/v1//_events/c1/subscriptions", false},
	}
	for _, tc := range cases {
		w := httptest.NewRecorder()
		h.ServeHTTP(w, httptest.NewRequest(tc.method, tc.path, http.NoBody))
		if got := w.Code == http.StatusNoContent; got != tc.ok {
			t.Errorf("%s %s: %d %s, want served=%v", tc.method, tc.path, w.Code, w.Body, tc.ok)
		}
		if !tc.ok && !strings.Contains(w.Body.String(), "tenant_read_only") {
			t.Errorf("%s %s: body %s lacks tenant_read_only", tc.method, tc.path, w.Body)
		}
	}
}
