package tenanthost_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"maps"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/dataentry"
	"github.com/Sourcehaven-BV/rela/internal/tenant"
	"github.com/Sourcehaven-BV/rela/internal/tenanthost"
	"github.com/Sourcehaven-BV/rela/internal/testutil/logtest"
)

// controlTable is a fake tenant registry with statuses. An org it does not
// list is absent, as [tenant.ClusterResolver] reports a valid org with no
// control row.
type controlTable struct {
	mu     sync.Mutex
	status map[string]tenant.Status
	calls  atomic.Int64
}

func newControl(statuses map[string]tenant.Status) *controlTable {
	if statuses == nil {
		statuses = map[string]tenant.Status{}
	}
	return &controlTable{status: statuses}
}

func (c *controlTable) set(org string, s tenant.Status) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.status[org] = s
}

func (c *controlTable) acquire(_ context.Context, orgID string) (http.Handler, func(), error) {
	c.calls.Add(1)
	c.mu.Lock()
	s, ok := c.status[orgID]
	c.mu.Unlock()
	switch {
	case !ok:
		return nil, nil, fmt.Errorf("%w: %w: %q", tenant.ErrUnknownTenant, tenant.ErrTenantAbsent, orgID)
	case s == tenant.StatusProvisioning:
		return nil, nil, fmt.Errorf("%w: %w: %w", tenant.ErrUnknownTenant, tenant.ErrTenantInactive,
			tenant.ErrTenantProvisioning)
	case s == tenant.StatusSuspended:
		return nil, nil, fmt.Errorf("%w: %w", tenant.ErrUnknownTenant, tenant.ErrTenantInactive)
	case s == tenant.StatusErased:
		return nil, nil, fmt.Errorf("%w: %w: %q", tenant.ErrUnknownTenant, tenant.ErrTenantErased, orgID)
	case s != tenant.StatusActive:
		return nil, nil, fmt.Errorf("%w: %s", tenant.ErrUnknownTenant, s)
	}
	h := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprintf(w, "tenant=%s", orgID)
	})
	return h, func() {}, nil
}

// stubProvisioner counts its calls. A call waits for gate when it is set,
// then returns err, or marks the org active in control.
type stubProvisioner struct {
	control *controlTable
	gate    chan struct{}
	err     error
	calls   atomic.Int64
	orgs    sync.Map
}

func (p *stubProvisioner) Provision(ctx context.Context, orgID string) error {
	p.calls.Add(1)
	p.orgs.Store(orgID, true)
	if p.gate != nil {
		select {
		case <-p.gate:
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	if p.err != nil {
		return p.err
	}
	p.control.set(orgID, tenant.StatusActive)
	return nil
}

// eventWebhook accepts "<event>|<org>" bodies.
type eventWebhook struct{}

func (eventWebhook) VerifyWebhook(_ context.Context, raw string) (dataentry.WebhookClaims, error) {
	event, org, ok := strings.Cut(raw, "|")
	if !ok {
		return dataentry.WebhookClaims{}, errors.New("bad signature")
	}
	return dataentry.WebhookClaims{Event: event, OrgID: org, ID: "jti-1"}, nil
}

func newProvHost(t *testing.T, c *controlTable, cfg tenanthost.ProvisionConfig) *tenanthost.Host {
	t.Helper()
	h, err := tenanthost.New(tenanthost.Config{
		Verifier: &fakeVerifier{}, HeaderName: header, Acquire: c.acquire, Static: static,
		Webhook: eventWebhook{}, Provision: cfg,
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	t.Cleanup(h.Close)
	return h
}

func statusOf(t *testing.T, body string) string {
	t.Helper()
	var s struct {
		Status string `json:"status"`
	}
	if err := json.Unmarshal([]byte(body), &s); err != nil {
		t.Fatalf("body %q is not a status: %v", body, err)
	}
	return s.Status
}

// waitFor polls cond until it holds or a second passes.
func waitFor(t *testing.T, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(time.Second)
	for !cond() {
		if time.Now().After(deadline) {
			t.Fatal("condition not met in time")
		}
		time.Sleep(time.Millisecond)
	}
}

func TestParseProvisionMode(t *testing.T) {
	t.Parallel()
	for in, want := range map[string]tenanthost.ProvisionMode{
		"": tenanthost.ProvisionOff, "off": tenanthost.ProvisionOff,
		"webhook": tenanthost.ProvisionWebhook, "lazy": tenanthost.ProvisionLazy,
	} {
		if got, err := tenanthost.ParseProvisionMode(in); err != nil || got != want {
			t.Errorf("ParseProvisionMode(%q) = %q, %v; want %q", in, got, err, want)
		}
	}
	for _, bad := range []string{"on", "LAZY", "eager"} {
		if _, err := tenanthost.ParseProvisionMode(bad); err == nil {
			t.Errorf("ParseProvisionMode(%q) accepted", bad)
		}
	}
}

func TestNew_RejectsBadProvisionConfig(t *testing.T) {
	t.Parallel()
	c := newControl(nil)
	p := &stubProvisioner{control: c}
	cases := map[string]tenanthost.Config{
		"unknown mode":   {Provision: tenanthost.ProvisionConfig{Mode: "sometimes", Provisioner: p}},
		"no provisioner": {Provision: tenanthost.ProvisionConfig{Mode: tenanthost.ProvisionLazy}},
		"webhook mode without webhook": {Webhook: nil,
			Provision: tenanthost.ProvisionConfig{Mode: tenanthost.ProvisionWebhook, Provisioner: p}},
		"negative rate": {Provision: tenanthost.ProvisionConfig{
			Mode: tenanthost.ProvisionLazy, Provisioner: p, PerMinute: -1}},
	}
	for name, cfg := range cases {
		cfg.Verifier, cfg.HeaderName, cfg.Acquire, cfg.Static = &fakeVerifier{}, header, c.acquire, static
		if _, err := tenanthost.New(cfg); err == nil {
			t.Errorf("%s: New accepted it", name)
		}
	}
	// Lazy alone needs no webhook.
	h, err := tenanthost.New(tenanthost.Config{
		Verifier: &fakeVerifier{}, HeaderName: header, Acquire: c.acquire, Static: static,
		Provision: tenanthost.ProvisionConfig{Mode: tenanthost.ProvisionLazy, Provisioner: p},
	})
	if err != nil {
		t.Fatalf("lazy without webhook: %v", err)
	}
	h.Close()
}

// TestProvision_OffKeepsDenial pins the default: an unknown org stays a 403
// and its webhook is acknowledged and dropped.
func TestProvision_OffKeepsDenial(t *testing.T) {
	t.Parallel()
	c := newControl(map[string]tenant.Status{"org-p": tenant.StatusProvisioning})
	h := newProvHost(t, c, tenanthost.ProvisionConfig{})

	if w := do(h, http.MethodGet, "/api/v1/tickets", "tok-org-new", ""); w.Code != http.StatusForbidden {
		t.Errorf("unknown org: status = %d, want 403", w.Code)
	}
	if w := do(h, http.MethodGet, "/api/v1/tickets", "tok-org-p", ""); w.Code != http.StatusForbidden {
		t.Errorf("provisioning org: status = %d, want 403", w.Code)
	}
	if w := do(h, http.MethodPost, tenanthost.WebhookPath, "", "organization.created|org-new"); w.Code != http.StatusOK {
		t.Errorf("webhook: status = %d, want 200", w.Code)
	}
}

func TestProvision_Webhook(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		statuses map[string]tenant.Status
		body     string
		gate     bool // the provisioner blocks until the test ends
		err      error
		cfg      tenanthost.ProvisionConfig
		want     int
		calls    int64
	}{
		{name: "created for a new org", body: "organization.created|org-new", want: http.StatusOK, calls: 1},
		{name: "created finishes a half create", body: "organization.created|org-p",
			statuses: map[string]tenant.Status{"org-p": tenant.StatusProvisioning}, want: http.StatusOK, calls: 1},
		{name: "created for an active org", body: "organization.created|org-a",
			statuses: map[string]tenant.Status{"org-a": tenant.StatusActive}, want: http.StatusOK},
		{name: "still running", body: "organization.created|org-new", gate: true,
			cfg: tenanthost.ProvisionConfig{WebhookWait: 10 * time.Millisecond}, want: http.StatusAccepted, calls: 1},
		{name: "another process creating it", body: "organization.created|org-new", err: tenant.ErrTenantInUse,
			want: http.StatusAccepted, calls: 1},
		{name: "failed", body: "organization.created|org-new", err: errors.New("migrate: boom"),
			want: http.StatusBadGateway, calls: 1},
		{name: "created for an erased org is never provisioned", body: "organization.created|org-e",
			statuses: map[string]tenant.Status{"org-e": tenant.StatusErased}, want: http.StatusOK},
		{name: "other event for a new org", body: "membership.created|org-new", want: http.StatusOK},
		{name: "deletion without status sync is refused", body: "organization.deleted|org-new",
			want: http.StatusServiceUnavailable},
		{name: "invalid org id never reaches the provisioner", body: "organization.created|bad\x00org",
			want: http.StatusOK},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			c := newControl(tc.statuses)
			p := &stubProvisioner{control: c, err: tc.err}
			if tc.gate {
				p.gate = make(chan struct{})
				defer close(p.gate)
			}
			cfg := tc.cfg
			cfg.Mode, cfg.Provisioner = tenanthost.ProvisionWebhook, p
			h := newProvHost(t, c, cfg)
			w := do(h, http.MethodPost, tenanthost.WebhookPath, "", tc.body)
			if w.Code != tc.want {
				t.Fatalf("status = %d, want %d (body %q)", w.Code, tc.want, w.Body.String())
			}
			if got := p.calls.Load(); got != tc.calls {
				t.Errorf("provisioner calls = %d, want %d", got, tc.calls)
			}
		})
	}
}

// TestProvision_WebhookModeLeavesRequestsDenied pins that webhook mode does
// not provision from a request, and answers 202 for a tenant being created.
func TestProvision_WebhookModeLeavesRequestsDenied(t *testing.T) {
	t.Parallel()
	c := newControl(map[string]tenant.Status{"org-p": tenant.StatusProvisioning})
	p := &stubProvisioner{control: c}
	h := newProvHost(t, c, tenanthost.ProvisionConfig{Mode: tenanthost.ProvisionWebhook, Provisioner: p})

	if w := do(h, http.MethodGet, "/api/v1/tickets", "tok-org-new", ""); w.Code != http.StatusForbidden {
		t.Errorf("unknown org: status = %d, want 403", w.Code)
	}
	w := do(h, http.MethodGet, "/api/v1/tickets", "tok-org-p", "")
	if w.Code != http.StatusAccepted || statusOf(t, w.Body.String()) != "provisioning" {
		t.Errorf("provisioning org: %d %q, want 202 provisioning", w.Code, w.Body.String())
	}
	if p.calls.Load() != 0 {
		t.Errorf("a request provisioned in webhook mode")
	}
}

// TestProvision_LazyRefusesErased pins that lazy provisioning never
// recreates an erased tenant: a request for it is a plain 403, and the
// status route reports no tenant either.
func TestProvision_LazyRefusesErased(t *testing.T) {
	t.Parallel()
	c := newControl(map[string]tenant.Status{"org-e": tenant.StatusErased})
	p := &stubProvisioner{control: c}
	h := newProvHost(t, c, tenanthost.ProvisionConfig{Mode: tenanthost.ProvisionLazy, Provisioner: p})

	if w := do(h, http.MethodGet, "/api/v1/tickets", "tok-org-e", ""); w.Code != http.StatusForbidden {
		t.Errorf("request for an erased org: status = %d, want 403 (body %q)", w.Code, w.Body.String())
	}
	if w := do(h, http.MethodGet, tenanthost.StatusPath, "tok-org-e", ""); w.Code != http.StatusForbidden {
		t.Errorf("status route for an erased org: status = %d, want 403 (body %q)", w.Code, w.Body.String())
	}
	if w := do(h, http.MethodPost, tenanthost.WebhookPath, "", "organization.created|org-e"); w.Code != http.StatusOK {
		t.Errorf("organization.created for an erased org: status = %d, want 200", w.Code)
	}
	if got := p.calls.Load(); got != 0 {
		t.Errorf("provisioner calls = %d, want 0", got)
	}
}

// TestProvision_LazySingleFlight pins that concurrent first requests for one
// org share one run, and are answered 202 without waiting for it.
func TestProvision_LazySingleFlight(t *testing.T) {
	t.Parallel()
	c := newControl(nil)
	p := &stubProvisioner{control: c, gate: make(chan struct{})}
	h := newProvHost(t, c, tenanthost.ProvisionConfig{Mode: tenanthost.ProvisionLazy, Provisioner: p})

	const n = 32
	var wg sync.WaitGroup
	codes := make([]int, n)
	for i := range n {
		wg.Go(func() {
			w := do(h, http.MethodGet, "/api/v1/tickets", "tok-org-new", "")
			codes[i] = w.Code
			if w.Header().Get("Retry-After") == "" {
				t.Error("202 without Retry-After")
			}
		})
	}
	wg.Wait()
	for i, code := range codes {
		if code != http.StatusAccepted {
			t.Errorf("request %d: status = %d, want 202", i, code)
		}
	}
	close(p.gate)
	waitFor(t, func() bool {
		w := do(h, http.MethodGet, "/api/v1/tickets", "tok-org-new", "")
		return w.Code == http.StatusOK && strings.Contains(w.Body.String(), "tenant=org-new")
	})
	if got := p.calls.Load(); got != 1 {
		t.Errorf("provisioner calls = %d, want 1", got)
	}
}

// newRacingHost builds a lazy host whose Acquire, once armed, lets p's run
// finish after reading the tenant as absent and before the request reaches
// the runner.
func newRacingHost(t *testing.T, c *controlTable, p *stubProvisioner) (*tenanthost.Host, *atomic.Bool) {
	t.Helper()
	var h *tenanthost.Host
	armed := &atomic.Bool{}
	acquire := func(ctx context.Context, orgID string) (http.Handler, func(), error) {
		next, release, err := c.acquire(ctx, orgID)
		if err != nil && armed.CompareAndSwap(true, false) {
			close(p.gate)
			waitFor(t, func() bool { return !tenanthost.ProvisionRunning(h, orgID) })
		}
		return next, release, err
	}
	h, err := tenanthost.New(tenanthost.Config{
		Verifier: &fakeVerifier{}, HeaderName: header, Acquire: acquire, Static: static,
		Webhook: eventWebhook{}, Provision: tenanthost.ProvisionConfig{Mode: tenanthost.ProvisionLazy, Provisioner: p},
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	t.Cleanup(h.Close)
	return h, armed
}

// TestProvision_NoRerunAfterSuccess pins that a request or webhook which
// read the tenant as absent while a run was going, and reaches the runner
// only after that run succeeded, starts no second run (BUG-CZFPMSX). The
// lazy answer is the same 202 as during the run, so the call count carries
// that case. The webhook is not acknowledged, so the IdP retries and the
// retry reads the tenant again.
func TestProvision_NoRerunAfterSuccess(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name                string
		method, path, token string
		body                string
		want                int
	}{
		{name: "lazy request", method: http.MethodGet, path: "/api/v1/tickets", token: "tok-org-new",
			want: http.StatusAccepted},
		{name: "webhook", method: http.MethodPost, path: tenanthost.WebhookPath,
			body: "organization.created|org-new", want: http.StatusServiceUnavailable},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			c := newControl(nil)
			p := &stubProvisioner{control: c, gate: make(chan struct{})}
			h, armed := newRacingHost(t, c, p)

			if w := do(h, http.MethodGet, "/api/v1/tickets", "tok-org-new", ""); w.Code != http.StatusAccepted {
				t.Fatalf("first request: status = %d, want 202", w.Code)
			}
			armed.Store(true)
			w := do(h, tc.method, tc.path, tc.token, tc.body)
			if w.Code != tc.want || w.Header().Get("Retry-After") == "" {
				t.Fatalf("racing the run's end: status = %d (Retry-After %q), want %d with Retry-After",
					w.Code, w.Header().Get("Retry-After"), tc.want)
			}
			h.Close() // waits for any run the racing call started
			if got := p.calls.Load(); got != 1 {
				t.Errorf("provisioner calls = %d, want 1", got)
			}
			if w := do(h, http.MethodGet, "/api/v1/tickets", "tok-org-new", ""); w.Code != http.StatusOK {
				t.Errorf("request after the run: status = %d, want 200", w.Code)
			}
		})
	}
}

// TestProvision_SuccessHoldEnds pins that the hold after a successful run
// ends: once it has passed, a request for an org whose tenant is absent
// again starts a new run.
func TestProvision_SuccessHoldEnds(t *testing.T) {
	t.Parallel()
	c := newControl(nil)
	p := &stubProvisioner{control: c}
	h := newProvHost(t, c, tenanthost.ProvisionConfig{Mode: tenanthost.ProvisionLazy, Provisioner: p})
	var mu sync.Mutex
	now := time.Now()
	tenanthost.SetProvisionClock(h, func() time.Time { mu.Lock(); defer mu.Unlock(); return now })

	do(h, http.MethodGet, "/api/v1/tickets", "tok-org-new", "")
	waitFor(t, func() bool { return !tenanthost.ProvisionRunning(h, "org-new") && p.calls.Load() == 1 })
	c.mu.Lock()
	delete(c.status, "org-new") // an operator removed the control row
	c.mu.Unlock()

	if w := do(h, http.MethodGet, "/api/v1/tickets", "tok-org-new", ""); w.Code != http.StatusAccepted {
		t.Fatalf("request in the hold: status = %d, want 202", w.Code)
	}
	if tenanthost.ProvisionRunning(h, "org-new") || p.calls.Load() != 1 {
		t.Fatalf("a request in the hold started a run (calls = %d)", p.calls.Load())
	}
	mu.Lock()
	now = now.Add(tenanthost.SuccessHold)
	mu.Unlock()
	do(h, http.MethodGet, "/api/v1/tickets", "tok-org-new", "")
	waitFor(t, func() bool { return p.calls.Load() == 2 })
}

// TestProvision_RunOutlivesRequest pins that a run is not cancelled with the
// request that started it.
func TestProvision_RunOutlivesRequest(t *testing.T) {
	t.Parallel()
	c := newControl(nil)
	p := &stubProvisioner{control: c, gate: make(chan struct{})}
	h := newProvHost(t, c, tenanthost.ProvisionConfig{Mode: tenanthost.ProvisionLazy, Provisioner: p})

	ctx, cancel := context.WithCancel(t.Context())
	r, _ := http.NewRequestWithContext(ctx, http.MethodGet, "/api/v1/tickets", http.NoBody)
	r.Header.Set(header, "tok-org-new")
	h.ServeHTTP(discard{}, r)
	cancel()
	close(p.gate)
	waitFor(t, func() bool {
		return do(h, http.MethodGet, "/api/v1/tickets", "tok-org-new", "").Code == http.StatusOK
	})
}

type discard struct{}

func (discard) Header() http.Header         { return http.Header{} }
func (discard) Write(b []byte) (int, error) { return len(b), nil }
func (discard) WriteHeader(int)             {}

// TestProvision_LazyFailureIsRetryable pins that a failed run answers a
// retryable 503 without its cause, and that a request after the failure
// hold starts a new run.
func TestProvision_LazyFailureIsRetryable(t *testing.T) {
	t.Parallel()
	c := newControl(nil)
	p := &stubProvisioner{control: c, err: errors.New("migrate: boom")}
	h := newProvHost(t, c, tenanthost.ProvisionConfig{Mode: tenanthost.ProvisionLazy, Provisioner: p})
	var mu sync.Mutex
	now := time.Now()
	tenanthost.SetProvisionClock(h, func() time.Time { mu.Lock(); defer mu.Unlock(); return now })

	if w := do(h, http.MethodGet, "/api/v1/tickets", "tok-org-new", ""); w.Code != http.StatusAccepted {
		t.Fatalf("first request: status = %d, want 202", w.Code)
	}
	var rec *httptest.ResponseRecorder
	waitFor(t, func() bool {
		rec = do(h, http.MethodGet, "/api/v1/tickets", "tok-org-new", "")
		return rec.Code == http.StatusServiceUnavailable
	})
	if !strings.Contains(rec.Body.String(), "provisioning_failed") || rec.Header().Get("Retry-After") == "" {
		t.Errorf("failure answer %q (Retry-After %q), want provisioning_failed with Retry-After",
			rec.Body.String(), rec.Header().Get("Retry-After"))
	}
	if strings.Contains(rec.Body.String(), "boom") {
		t.Error("the failure cause reached the client")
	}
	if p.calls.Load() != 1 {
		t.Errorf("provisioner calls = %d during the hold, want 1", p.calls.Load())
	}
	mu.Lock()
	now = now.Add(20 * time.Second)
	mu.Unlock()
	if got := do(h, http.MethodGet, "/api/v1/tickets", "tok-org-new", "").Header().Get("Retry-After"); got != "10" {
		t.Errorf("Retry-After 20s into the hold = %q, want 10 (the hold left)", got)
	}

	mu.Lock()
	now = now.Add(time.Minute)
	mu.Unlock()
	if w := do(h, http.MethodGet, "/api/v1/tickets", "tok-org-new", ""); w.Code != http.StatusAccepted {
		t.Fatalf("after the hold: status = %d, want 202", w.Code)
	}
	waitFor(t, func() bool { return p.calls.Load() == 2 })
}

// TestProvision_LazyHoldsWhileAnotherProcessProvisions pins that a run that
// finds another process creating the tenant is remembered for a short
// hold. During it, polls answer 202 provisioning without starting a run or
// taking a token from the start rate, so another org can still start. After
// the hold a poll starts a run again.
func TestProvision_LazyHoldsWhileAnotherProcessProvisions(t *testing.T) {
	t.Parallel()
	c := newControl(nil)
	p := &stubProvisioner{control: c, err: fmt.Errorf("lock held: %w", tenant.ErrTenantInUse)}
	h := newProvHost(t, c, tenanthost.ProvisionConfig{
		Mode: tenanthost.ProvisionLazy, Provisioner: p, PerMinute: 3, MaxRunning: 10,
	})
	var mu sync.Mutex
	now := time.Now()
	tenanthost.SetProvisionClock(h, func() time.Time { mu.Lock(); defer mu.Unlock(); return now })

	poll := func(org string) *httptest.ResponseRecorder {
		t.Helper()
		w := do(h, http.MethodGet, "/api/v1/tickets", "tok-"+org, "")
		if w.Code != http.StatusAccepted || statusOf(t, w.Body.String()) != "provisioning" {
			t.Fatalf("%s: %d %s, want 202 provisioning", org, w.Code, w.Body.String())
		}
		return w
	}
	poll("org-new")
	waitFor(t, func() bool { return p.calls.Load() == 1 && !tenanthost.ProvisionRunning(h, "org-new") })

	for range 5 {
		if w := poll("org-new"); w.Header().Get("Retry-After") == "" {
			t.Fatal("a provisioning answer during the hold has no Retry-After")
		}
	}
	if got := p.calls.Load(); got != 1 {
		t.Fatalf("provisioner calls = %d during the hold, want 1", got)
	}
	if w := do(h, http.MethodPost, tenanthost.WebhookPath, "", "organization.created|org-new"); w.Code != http.StatusAccepted {
		t.Errorf("webhook during the hold: status = %d, want 202", w.Code)
	}

	// The polls took no token, so a second org still starts.
	poll("org-2")
	waitFor(t, func() bool { return p.calls.Load() == 2 })

	mu.Lock()
	now = now.Add(time.Minute)
	mu.Unlock()
	poll("org-new")
	waitFor(t, func() bool { return p.calls.Load() == 3 })
}

// TestProvision_RateCap pins that runs over the cap are refused with 503.
func TestProvision_RateCap(t *testing.T) {
	t.Parallel()
	t.Run("running cap", func(t *testing.T) {
		t.Parallel()
		c := newControl(nil)
		p := &stubProvisioner{control: c, gate: make(chan struct{})}
		defer close(p.gate)
		h := newProvHost(t, c, tenanthost.ProvisionConfig{
			Mode: tenanthost.ProvisionLazy, Provisioner: p, MaxRunning: 2,
		})
		for _, org := range []string{"org-1", "org-2"} {
			if w := do(h, http.MethodGet, "/api/v1/tickets", "tok-"+org, ""); w.Code != http.StatusAccepted {
				t.Fatalf("%s: status = %d, want 202", org, w.Code)
			}
		}
		w := do(h, http.MethodGet, "/api/v1/tickets", "tok-org-3", "")
		if w.Code != http.StatusServiceUnavailable || w.Header().Get("Retry-After") == "" {
			t.Errorf("third org: %d (Retry-After %q), want 503 with Retry-After", w.Code, w.Header().Get("Retry-After"))
		}
		if w := do(h, http.MethodPost, tenanthost.WebhookPath, "", "organization.created|org-4"); w.Code != http.StatusServiceUnavailable {
			t.Errorf("webhook over the cap: status = %d, want 503", w.Code)
		}
		// A request for an org already running is not refused.
		if w := do(h, http.MethodGet, "/api/v1/tickets", "tok-org-1", ""); w.Code != http.StatusAccepted {
			t.Errorf("running org: status = %d, want 202", w.Code)
		}
	})
	t.Run("rate", func(t *testing.T) {
		t.Parallel()
		c := newControl(nil)
		p := &stubProvisioner{control: c}
		h := newProvHost(t, c, tenanthost.ProvisionConfig{
			Mode: tenanthost.ProvisionLazy, Provisioner: p, PerMinute: 1, MaxRunning: 10,
		})
		if w := do(h, http.MethodGet, "/api/v1/tickets", "tok-org-1", ""); w.Code != http.StatusAccepted {
			t.Fatalf("first: status = %d, want 202", w.Code)
		}
		if w := do(h, http.MethodGet, "/api/v1/tickets", "tok-org-2", ""); w.Code != http.StatusServiceUnavailable {
			t.Errorf("second within the minute: status = %d, want 503", w.Code)
		}
	})
}

// TestProvision_InvalidOrgNeverReachesProvisioner pins that an org id that
// cannot be a tenant is refused before any run starts.
func TestProvision_InvalidOrgNeverReachesProvisioner(t *testing.T) {
	t.Parallel()
	c := newControl(nil)
	p := &stubProvisioner{control: c}
	h := newProvHost(t, c, tenanthost.ProvisionConfig{Mode: tenanthost.ProvisionLazy, Provisioner: p})
	long := "tok-" + strings.Repeat("x", 300)
	for _, tok := range []string{long, "tok-bad\x7forg"} {
		if w := do(h, http.MethodGet, "/api/v1/tickets", tok, ""); w.Code != http.StatusForbidden {
			t.Errorf("status = %d, want 403", w.Code)
		}
		if w := do(h, http.MethodGet, tenanthost.StatusPath, tok, ""); w.Code != http.StatusForbidden {
			t.Errorf("status route: status = %d, want 403", w.Code)
		}
	}
	if p.calls.Load() != 0 {
		t.Errorf("provisioner called %d times for an invalid org id", p.calls.Load())
	}
}

// TestProvision_StatusRoute pins the poll answer for each state, and that
// it answers only for the caller's own org.
func TestProvision_StatusRoute(t *testing.T) {
	t.Parallel()
	statuses := map[string]tenant.Status{
		"org-a": tenant.StatusActive, "org-p": tenant.StatusProvisioning,
		"org-s": tenant.StatusSuspended, "org-e": tenant.StatusErasing,
	}
	tests := []struct {
		mode   tenanthost.ProvisionMode
		token  string
		code   int
		status string
	}{
		{mode: tenanthost.ProvisionOff, token: "tok-org-a", code: http.StatusOK, status: "active"},
		{mode: tenanthost.ProvisionOff, token: "tok-org-p", code: http.StatusOK, status: "provisioning"},
		{mode: tenanthost.ProvisionOff, token: "tok-org-s", code: http.StatusOK, status: "suspended"},
		{mode: tenanthost.ProvisionOff, token: "tok-org-e", code: http.StatusForbidden},
		{mode: tenanthost.ProvisionOff, token: "tok-org-new", code: http.StatusForbidden},
		{mode: tenanthost.ProvisionLazy, token: "tok-org-new", code: http.StatusOK, status: "provisioning"},
		{mode: tenanthost.ProvisionLazy, token: "tok-org-s", code: http.StatusOK, status: "suspended"},
		{mode: tenanthost.ProvisionOff, token: "", code: http.StatusUnauthorized},
		{mode: tenanthost.ProvisionOff, token: "tok-noorg", code: http.StatusForbidden},
	}
	for _, tc := range tests {
		t.Run(string(tc.mode)+" "+tc.token, func(t *testing.T) {
			t.Parallel()
			st := maps.Clone(statuses)
			c := newControl(st)
			p := &stubProvisioner{control: c, gate: make(chan struct{})}
			defer close(p.gate)
			cfg := tenanthost.ProvisionConfig{Mode: tc.mode}
			if tc.mode != tenanthost.ProvisionOff {
				cfg.Provisioner = p
			}
			h := newProvHost(t, c, cfg)
			w := do(h, http.MethodGet, tenanthost.StatusPath+"?org_id=org-a", tc.token, "")
			if w.Code != tc.code {
				t.Fatalf("status = %d, want %d (body %q)", w.Code, tc.code, w.Body.String())
			}
			if tc.status != "" && statusOf(t, w.Body.String()) != tc.status {
				t.Errorf("body %q, want status %s", w.Body.String(), tc.status)
			}
			if strings.Contains(w.Body.String(), "tenant=") {
				t.Error("the status route reached a tenant router")
			}
			if strings.Contains(w.Body.String(), "org-") {
				t.Error("the status body names an org")
			}
		})
	}
}

var _ io.Writer = discard{}

// TestProvision_WriteDuringProvisioningIsRefused pins that only a read is
// answered 202 while a tenant is created. A write gets 503
// tenant_provisioning, so a client cannot read it as done.
func TestProvision_WriteDuringProvisioningIsRefused(t *testing.T) {
	t.Parallel()
	for _, mode := range []tenanthost.ProvisionMode{tenanthost.ProvisionWebhook, tenanthost.ProvisionLazy} {
		t.Run(string(mode), func(t *testing.T) {
			t.Parallel()
			c := newControl(map[string]tenant.Status{"org-p": tenant.StatusProvisioning})
			p := &stubProvisioner{control: c, gate: make(chan struct{})}
			defer close(p.gate)
			h := newProvHost(t, c, tenanthost.ProvisionConfig{Mode: mode, Provisioner: p})
			for _, m := range []string{http.MethodGet, http.MethodHead} {
				if w := do(h, m, "/api/v1/tickets", "tok-org-p", ""); w.Code != http.StatusAccepted {
					t.Errorf("%s: status = %d, want 202", m, w.Code)
				}
			}
			for _, m := range []string{http.MethodPost, http.MethodPatch, http.MethodPut, http.MethodDelete} {
				w := do(h, m, "/api/v1/tickets", "tok-org-p", `{"title":"x"}`)
				if w.Code != http.StatusServiceUnavailable || !strings.Contains(w.Body.String(), "tenant_provisioning") ||
					w.Header().Get("Retry-After") == "" {

					t.Errorf("%s: %d %q (Retry-After %q), want 503 tenant_provisioning with Retry-After",
						m, w.Code, w.Body.String(), w.Header().Get("Retry-After"))
				}
			}
		})
	}
}

// TestProvision_NeedsOperator pins the answer to a create that retrying
// cannot fix: the webhook is refused with 503 and Retry-After so the IdP
// keeps the event, requests get 503 provisioning_needs_operator without
// Retry-After, and no run starts again within the hold.
func TestProvision_NeedsOperator(t *testing.T) {
	t.Parallel()
	c := newControl(nil)
	p := &stubProvisioner{control: c,
		err: fmt.Errorf("%w: %w", tenanthost.ErrNeedsOperator, tenant.ErrSchemaExists)}
	h := newProvHost(t, c, tenanthost.ProvisionConfig{Mode: tenanthost.ProvisionLazy, Provisioner: p})
	var mu sync.Mutex
	now := time.Now()
	tenanthost.SetProvisionClock(h, func() time.Time { mu.Lock(); defer mu.Unlock(); return now })

	webhookRefused := func(when string) {
		t.Helper()
		w := do(h, http.MethodPost, tenanthost.WebhookPath, "", "organization.created|org-new")
		if w.Code != http.StatusServiceUnavailable || !strings.Contains(w.Body.String(), "operator") ||
			w.Header().Get("Retry-After") == "" {

			t.Errorf("webhook %s: %d %q (Retry-After %q), want 503 naming operator action with Retry-After",
				when, w.Code, w.Body.String(), w.Header().Get("Retry-After"))
		}
	}
	webhookRefused("first")
	for _, advance := range []time.Duration{0, 10 * time.Minute} {
		mu.Lock()
		now = now.Add(advance)
		mu.Unlock()
		rec := do(h, http.MethodGet, "/api/v1/tickets", "tok-org-new", "")
		if rec.Code != http.StatusServiceUnavailable || !strings.Contains(rec.Body.String(), "provisioning_needs_operator") {
			t.Errorf("request: %d %q, want 503 provisioning_needs_operator", rec.Code, rec.Body.String())
		}
		if ra := rec.Header().Get("Retry-After"); ra != "" {
			t.Errorf("Retry-After = %q, want none", ra)
		}
		webhookRefused("during the hold")
	}
	if got := p.calls.Load(); got != 1 {
		t.Errorf("provisioner calls = %d, want 1", got)
	}
}

// TestProvision_ExistsButNotActive pins that a create that finds the tenant
// existing in another status is acknowledged with a body that says so, and
// is not held as a failure.
func TestProvision_ExistsButNotActive(t *testing.T) {
	t.Parallel()
	c := newControl(nil)
	p := &stubProvisioner{control: c,
		err: &tenant.ExistsError{OrgID: "org-new", Status: tenant.StatusSuspended}}
	h := newProvHost(t, c, tenanthost.ProvisionConfig{Mode: tenanthost.ProvisionLazy, Provisioner: p})
	w := do(h, http.MethodPost, tenanthost.WebhookPath, "", "organization.created|org-new")
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), "not active") {
		t.Fatalf("webhook: %d %q, want 200 saying the tenant is not active", w.Code, w.Body.String())
	}
	// Not held: the next request starts a run rather than answering a failure.
	if w := do(h, http.MethodGet, "/api/v1/tickets", "tok-org-new", ""); w.Code != http.StatusAccepted {
		t.Errorf("request after: status = %d, want 202", w.Code)
	}
}

// TestProvision_CloseCancelsAndWaits pins that Host.Close cancels a run in
// progress and returns only once it has ended.
func TestProvision_CloseCancelsAndWaits(t *testing.T) {
	t.Parallel()
	c := newControl(nil)
	ended := make(chan error, 1)
	p := provisionFunc(func(ctx context.Context, _ string) error {
		<-ctx.Done()
		time.Sleep(10 * time.Millisecond)
		ended <- ctx.Err()
		return ctx.Err()
	})
	h, err := tenanthost.New(tenanthost.Config{
		Verifier: &fakeVerifier{}, HeaderName: header, Acquire: c.acquire, Static: static,
		Provision: tenanthost.ProvisionConfig{Mode: tenanthost.ProvisionLazy, Provisioner: p},
	})
	if err != nil {
		t.Fatal(err)
	}
	if w := do(h, http.MethodGet, "/api/v1/tickets", "tok-org-new", ""); w.Code != http.StatusAccepted {
		t.Fatalf("status = %d, want 202", w.Code)
	}
	waitFor(t, func() bool { return tenanthost.ProvisionRunning(h, "org-new") })
	h.Close()
	select {
	case err := <-ended:
		if !errors.Is(err, context.Canceled) {
			t.Errorf("run ended with %v, want context.Canceled", err)
		}
	default:
		t.Fatal("Close returned before the run ended")
	}
	if tenanthost.ProvisionRunning(h, "org-new") {
		t.Error("a run is still recorded after Close")
	}
}

type provisionFunc func(ctx context.Context, orgID string) error

func (f provisionFunc) Provision(ctx context.Context, orgID string) error { return f(ctx, orgID) }

// TestProvision_StuckProvisioningIsLogged pins that webhook mode, which does
// not finish a create that stopped, warns about a tenant left in the
// provisioning status at most once a minute per org.
//
// Not parallel: it reads the default logger.
func TestProvision_StuckProvisioningIsLogged(t *testing.T) {
	logs := logtest.Capture(t, slog.LevelInfo)

	c := newControl(map[string]tenant.Status{"org-p": tenant.StatusProvisioning})
	h := newProvHost(t, c, tenanthost.ProvisionConfig{
		Mode: tenanthost.ProvisionWebhook, Provisioner: &stubProvisioner{control: c},
	})
	var mu sync.Mutex
	now := time.Now()
	tenanthost.SetProvisionClock(h, func() time.Time { mu.Lock(); defer mu.Unlock(); return now })
	count := func() int { return strings.Count(logs(), "still in the provisioning status") }

	for range 3 {
		do(h, http.MethodGet, "/api/v1/tickets", "tok-org-p", "")
		do(h, http.MethodGet, tenanthost.StatusPath, "tok-org-p", "")
	}
	if got := count(); got != 1 {
		t.Fatalf("warnings = %d within a minute, want 1", got)
	}
	if !strings.Contains(logs(), "rela-server tenant create") {
		t.Errorf("warning %q does not name the command that finishes the create", logs())
	}
	mu.Lock()
	now = now.Add(time.Minute)
	mu.Unlock()
	do(h, http.MethodGet, "/api/v1/tickets", "tok-org-p", "")
	if got := count(); got != 2 {
		t.Errorf("warnings = %d after a minute, want 2", got)
	}
}
