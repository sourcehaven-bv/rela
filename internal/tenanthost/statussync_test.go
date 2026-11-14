package tenanthost_test

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/tenant"
	"github.com/Sourcehaven-BV/rela/internal/tenanthost"
)

// idpOrgs is a fake IdP operator API: org id to suspended. An org it does
// not list is unknown; err, when set, is every answer.
type idpOrgs struct {
	suspended map[string]bool
	err       error
}

func (o *idpOrgs) OrgSuspended(_ context.Context, orgID string) (bool, error) {
	if o.err != nil {
		return false, o.err
	}
	s, ok := o.suspended[orgID]
	if !ok {
		return false, tenanthost.ErrOrgNotFound
	}
	return s, nil
}

// idpSuspension applies the IdP rules of [tenant.Provisioner] to a
// controlTable, recording who suspended each tenant.
type idpSuspension struct {
	c     *controlTable
	mu    sync.Mutex
	cause map[string]tenant.SuspendCause
}

func (s *idpSuspension) status(org string) (tenant.Status, bool) {
	s.c.mu.Lock()
	defer s.c.mu.Unlock()
	st, ok := s.c.status[org]
	return st, ok
}

func (s *idpSuspension) SuspendForIDP(_ context.Context, org string) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	st, ok := s.status(org)
	switch {
	case !ok:
		return false, fmt.Errorf("%w: %q", tenant.ErrUnknownTenant, org)
	case st == tenant.StatusProvisioning:
		return false, fmt.Errorf("%w: %w", tenant.ErrWrongStatus, tenant.ErrTenantProvisioning)
	case st != tenant.StatusActive:
		return false, nil
	}
	s.c.set(org, tenant.StatusSuspended)
	s.cause[org] = tenant.SuspendedByIDP
	return true, nil
}

func (s *idpSuspension) ResumeForIDP(_ context.Context, org string) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	st, ok := s.status(org)
	switch {
	case !ok:
		return false, fmt.Errorf("%w: %q", tenant.ErrUnknownTenant, org)
	case st != tenant.StatusSuspended:
		return false, nil
	case s.cause[org] != tenant.SuspendedByIDP:
		return false, tenant.ErrOperatorSuspended
	}
	s.c.set(org, tenant.StatusActive)
	delete(s.cause, org)
	return true, nil
}

// revalidations records the orgs the host asked to revalidate.
type revalidations struct {
	mu   sync.Mutex
	orgs []string
}

func (r *revalidations) revalidate(_ context.Context, org string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.orgs = append(r.orgs, org)
}

func (r *revalidations) list() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]string(nil), r.orgs...)
}

type syncFixture struct {
	host    *tenanthost.Host
	control *controlTable
	apply   *idpSuspension
	idp     *idpOrgs
	revals  *revalidations
}

func newSyncHost(t *testing.T, statuses map[string]tenant.Status, idp *idpOrgs) syncFixture {
	t.Helper()
	c := newControl(statuses)
	apply := &idpSuspension{c: c, cause: map[string]tenant.SuspendCause{}}
	revals := &revalidations{}
	ss, err := tenanthost.NewStatusSync(idp, apply, revals.revalidate)
	if err != nil {
		t.Fatal(err)
	}
	h, err := tenanthost.New(tenanthost.Config{
		Verifier: &fakeVerifier{}, HeaderName: header, Acquire: c.acquire, Static: static,
		Webhook: eventWebhook{}, StatusSync: ss,
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	t.Cleanup(h.Close)
	return syncFixture{host: h, control: c, apply: apply, idp: idp, revals: revals}
}

func updated(org string) string { return "organization.updated|" + org }

// TestStatusSync_FollowsTheIdP walks one tenant through an IdP suspension
// and reactivation, and checks that the events are not passed on.
func TestStatusSync_FollowsTheIdP(t *testing.T) {
	t.Parallel()
	idp := &idpOrgs{suspended: map[string]bool{"org-a": true}}
	f := newSyncHost(t, map[string]tenant.Status{"org-a": tenant.StatusActive}, idp)

	w := do(f.host, http.MethodPost, tenanthost.WebhookPath, "", updated("org-a"))
	if w.Code != http.StatusOK || strings.Contains(w.Body.String(), "tenant=") {
		t.Fatalf("suspend: %d %q; want 200, not passed to the tenant", w.Code, w.Body.String())
	}
	if st, _ := f.apply.status("org-a"); st != tenant.StatusSuspended {
		t.Fatalf("status after the IdP suspended the org = %s", st)
	}
	if got := f.revals.list(); len(got) != 1 || got[0] != "org-a" {
		t.Fatalf("revalidated %v; want [org-a]", got)
	}

	// A second update for the suspended tenant is answered, not refused.
	if w = do(f.host, http.MethodPost, tenanthost.WebhookPath, "", updated("org-a")); w.Code != http.StatusOK {
		t.Fatalf("update for a suspended tenant: %d", w.Code)
	}

	idp.suspended["org-a"] = false
	w = do(f.host, http.MethodPost, tenanthost.WebhookPath, "", updated("org-a"))
	if w.Code != http.StatusOK || strings.Contains(w.Body.String(), "tenant=") {
		t.Fatalf("resume: %d %q; want 200, not passed to the tenant", w.Code, w.Body.String())
	}
	if st, _ := f.apply.status("org-a"); st != tenant.StatusActive {
		t.Fatalf("status after the IdP reactivated the org = %s", st)
	}
	if f.control.calls.Load() != 0 {
		t.Errorf("the tenant was acquired %d times; want none", f.control.calls.Load())
	}
}

// TestStatusSync_OperatorSuspensionHolds pins that the IdP never undoes an
// operator's suspension.
func TestStatusSync_OperatorSuspensionHolds(t *testing.T) {
	t.Parallel()
	idp := &idpOrgs{suspended: map[string]bool{"org-a": false}}
	f := newSyncHost(t, map[string]tenant.Status{"org-a": tenant.StatusSuspended}, idp)
	f.apply.cause["org-a"] = tenant.SuspendedByOperator

	if w := do(f.host, http.MethodPost, tenanthost.WebhookPath, "", updated("org-a")); w.Code != http.StatusOK {
		t.Fatalf("update: %d; want 200", w.Code)
	}
	if st, _ := f.apply.status("org-a"); st != tenant.StatusSuspended {
		t.Fatalf("an operator suspension was undone by the IdP: %s", st)
	}
}

func TestStatusSync_Answers(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		statuses map[string]tenant.Status
		idp      *idpOrgs
		org      string
		want     int
	}{
		{name: "org with no tenant", idp: &idpOrgs{suspended: map[string]bool{"org-z": true}},
			org: "org-z", want: http.StatusOK},
		{name: "org the IdP does not know", statuses: map[string]tenant.Status{"org-a": tenant.StatusActive},
			idp: &idpOrgs{}, org: "org-a", want: http.StatusOK},
		{name: "IdP unreachable", statuses: map[string]tenant.Status{"org-a": tenant.StatusActive},
			idp: &idpOrgs{err: errors.New("dial: connection refused")}, org: "org-a", want: http.StatusBadGateway},
		{name: "tenant still being created", statuses: map[string]tenant.Status{"org-a": tenant.StatusProvisioning},
			idp: &idpOrgs{suspended: map[string]bool{"org-a": true}}, org: "org-a", want: http.StatusServiceUnavailable},
		{name: "erasing tenant", statuses: map[string]tenant.Status{"org-a": tenant.StatusErasing},
			idp: &idpOrgs{suspended: map[string]bool{"org-a": true}}, org: "org-a", want: http.StatusOK},
		{name: "erased tenant, IdP suspended", statuses: map[string]tenant.Status{"org-a": tenant.StatusErased},
			idp: &idpOrgs{suspended: map[string]bool{"org-a": true}}, org: "org-a", want: http.StatusOK},
		{name: "erased tenant, IdP active", statuses: map[string]tenant.Status{"org-a": tenant.StatusErased},
			idp: &idpOrgs{suspended: map[string]bool{"org-a": false}}, org: "org-a", want: http.StatusOK},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			f := newSyncHost(t, tc.statuses, tc.idp)
			w := do(f.host, http.MethodPost, tenanthost.WebhookPath, "", updated(tc.org))
			if w.Code != tc.want {
				t.Fatalf("%s: %d %q; want %d", tc.name, w.Code, w.Body.String(), tc.want)
			}
			if tc.want == http.StatusServiceUnavailable && w.Header().Get("Retry-After") == "" {
				t.Error("503 without Retry-After")
			}
			if st, ok := f.apply.status(tc.org); ok && tc.statuses[tc.org] != st {
				t.Errorf("status changed from %s to %s", tc.statuses[tc.org], st)
			}
		})
	}
}

// TestStatusSync_OtherEventsUnchanged pins that only organization.updated
// is handled by the host: another event for a suspended tenant is still
// refused with a retryable status.
func TestStatusSync_OtherEventsUnchanged(t *testing.T) {
	t.Parallel()
	f := newSyncHost(t, map[string]tenant.Status{"org-a": tenant.StatusSuspended},
		&idpOrgs{suspended: map[string]bool{"org-a": false}})
	w := do(f.host, http.MethodPost, tenanthost.WebhookPath, "", "user.deleted|org-a")
	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("user.deleted for a suspended tenant: %d; want 503", w.Code)
	}
	w = do(f.host, http.MethodPost, tenanthost.WebhookPath, "", "organization.deleted|org-a")
	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("organization.deleted for a suspended tenant: %d; want 503", w.Code)
	}
	if st, _ := f.apply.status("org-a"); st != tenant.StatusSuspended {
		t.Errorf("status changed to %s", st)
	}
}

// TestStatusSync_Off pins that without StatusSync an organization.updated
// is passed to the tenant as before.
func TestStatusSync_Off(t *testing.T) {
	t.Parallel()
	c := newControl(map[string]tenant.Status{"org-a": tenant.StatusActive})
	h := newProvHost(t, c, tenanthost.ProvisionConfig{})
	w := do(h, http.MethodPost, tenanthost.WebhookPath, "", updated("org-a"))
	if w.Code != http.StatusOK || w.Body.String() != "tenant=org-a" {
		t.Fatalf("update without sync: %d %q; want it passed to the tenant", w.Code, w.Body.String())
	}
}

func TestNewStatusSync_RejectsMissingCollaborators(t *testing.T) {
	t.Parallel()
	c := newControl(nil)
	apply := &idpSuspension{c: c, cause: map[string]tenant.SuspendCause{}}
	reval := func(context.Context, string) {}
	if _, err := tenanthost.NewStatusSync(nil, apply, reval); err == nil {
		t.Error("nil source accepted")
	}
	if _, err := tenanthost.NewStatusSync(&idpOrgs{}, nil, reval); err == nil {
		t.Error("nil writer accepted")
	}
	if _, err := tenanthost.NewStatusSync(&idpOrgs{}, apply, nil); err == nil {
		t.Error("nil revalidator accepted")
	}
	s, err := tenanthost.NewStatusSync(&idpOrgs{}, apply, reval)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := tenanthost.New(tenanthost.Config{
		Verifier: &fakeVerifier{}, HeaderName: header, Acquire: c.acquire, Static: static, StatusSync: s,
	}); err == nil {
		t.Error("status sync without the webhook accepted")
	}
}

// changingIDP is an IdP whose answer changes between calls. answer gives
// the answer of call n, counting from 1. When gate is set, call 1 waits
// for it. Every call is announced on entered.
type changingIDP struct {
	answer  func(n int) bool
	gate    chan struct{}
	entered chan int
	mu      sync.Mutex
	calls   int
}

func (o *changingIDP) OrgSuspended(ctx context.Context, _ string) (bool, error) {
	o.mu.Lock()
	o.calls++
	n := o.calls
	o.mu.Unlock()
	if o.entered != nil {
		o.entered <- n
	}
	if n == 1 && o.gate != nil {
		select {
		case <-o.gate:
		case <-ctx.Done():
			return false, ctx.Err()
		}
	}
	return o.answer(n), nil
}

func (o *changingIDP) count() int {
	o.mu.Lock()
	defer o.mu.Unlock()
	return o.calls
}

// idpSuspended returns an idpSuspension over one tenant the IdP suspended.
func idpSuspended(org string) *idpSuspension {
	c := newControl(map[string]tenant.Status{org: tenant.StatusSuspended})
	return &idpSuspension{c: c, cause: map[string]tenant.SuspendCause{org: tenant.SuspendedByIDP}}
}

// TestStatusSync_ConcurrentSyncsTakeTurns interleaves two syncs of one org.
// The first reads "active" from the IdP and is held there; the IdP then
// suspends the org, and a second sync starts. The second must not read the
// IdP until the first is done, and the tenant must end up suspended, not
// resumed by the first sync's stale answer.
func TestStatusSync_ConcurrentSyncsTakeTurns(t *testing.T) {
	t.Parallel()
	apply := idpSuspended("org-a")
	idp := &changingIDP{
		answer:  func(n int) bool { return n > 1 },
		gate:    make(chan struct{}),
		entered: make(chan int, 8),
	}
	revals := &revalidations{}
	ss, err := tenanthost.NewStatusSync(idp, apply, revals.revalidate)
	if err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	errs := make([]error, 2)
	wg.Go(func() { errs[0] = ss.SyncStatus(t.Context(), "org-a") })
	if n := <-idp.entered; n != 1 {
		t.Fatalf("first IdP call is call %d", n)
	}
	wg.Go(func() { errs[1] = ss.SyncStatus(t.Context(), "org-a") })
	select {
	case n := <-idp.entered:
		t.Fatalf("the second sync read the IdP (call %d) while the first held the org's turn", n)
	case <-time.After(50 * time.Millisecond):
	}
	close(idp.gate)
	wg.Wait()
	for i, err := range errs {
		if err != nil {
			t.Errorf("sync %d: %v", i, err)
		}
	}
	if st, _ := apply.status("org-a"); st != tenant.StatusSuspended {
		t.Fatalf("status after the interleaved syncs = %s, want suspended", st)
	}
	if got := idp.count(); got != 3 {
		t.Errorf("IdP calls = %d, want 3 (read, recheck after the resume, second sync)", got)
	}
}

// TestStatusSync_ResumeRechecksTheIdP pins the cross-process guard: the IdP
// answers "active", and then "suspended" because the org was suspended
// meanwhile, and a sync in another process found nothing to change. The
// resume is followed by a second read, which suspends the tenant again and
// closes it.
func TestStatusSync_ResumeRechecksTheIdP(t *testing.T) {
	t.Parallel()
	apply := idpSuspended("org-a")
	idp := &changingIDP{answer: func(n int) bool { return n > 1 }}
	revals := &revalidations{}
	ss, err := tenanthost.NewStatusSync(idp, apply, revals.revalidate)
	if err != nil {
		t.Fatal(err)
	}
	if err = ss.SyncStatus(t.Context(), "org-a"); err != nil {
		t.Fatal(err)
	}
	if st, _ := apply.status("org-a"); st != tenant.StatusSuspended {
		t.Fatalf("status = %s, want suspended again after the recheck", st)
	}
	if got := revals.list(); len(got) != 1 || got[0] != "org-a" {
		t.Errorf("revalidated %v; want [org-a]", got)
	}

	// A resume the IdP still agrees with stays.
	idp2 := &changingIDP{answer: func(int) bool { return false }}
	ss2, err := tenanthost.NewStatusSync(idp2, apply, revals.revalidate)
	if err != nil {
		t.Fatal(err)
	}
	if err = ss2.SyncStatus(t.Context(), "org-a"); err != nil {
		t.Fatal(err)
	}
	if st, _ := apply.status("org-a"); st != tenant.StatusActive || idp2.count() != 2 {
		t.Errorf("status = %s after %d IdP calls; want active after 2", st, idp2.count())
	}
}

// TestStatusSync_WaitHonoursContext pins that a sync waiting for the org's
// turn gives up when its context ends.
func TestStatusSync_WaitHonoursContext(t *testing.T) {
	t.Parallel()
	apply := idpSuspended("org-a")
	idp := &changingIDP{answer: func(int) bool { return true }, gate: make(chan struct{}), entered: make(chan int, 8)}
	ss, err := tenanthost.NewStatusSync(idp, apply, func(context.Context, string) {})
	if err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	wg.Go(func() { _ = ss.SyncStatus(t.Context(), "org-a") })
	<-idp.entered
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if err = ss.SyncStatus(ctx, "org-a"); !errors.Is(err, context.Canceled) {
		t.Errorf("waiting sync with a cancelled context: %v, want context.Canceled", err)
	}
	close(idp.gate)
	wg.Wait()
}
