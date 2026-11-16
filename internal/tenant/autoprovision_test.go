package tenant

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// fakeCreator records creates and blocks each until release is closed.
type fakeCreator struct {
	calls   atomic.Int32
	running atomic.Int32
	peak    atomic.Int32
	release chan struct{}
	err     error
}

func (f *fakeCreator) Provision(ctx context.Context, orgID string) (Tenant, error) {
	f.calls.Add(1)
	n := f.running.Add(1)
	defer f.running.Add(-1)
	for {
		p := f.peak.Load()
		if n <= p || f.peak.CompareAndSwap(p, n) {
			break
		}
	}
	select {
	case <-f.release:
	case <-ctx.Done():
		return Tenant{}, ctx.Err()
	}
	return Tenant{OrgID: orgID}, f.err
}

func newFake(err error) *fakeCreator { return &fakeCreator{release: make(chan struct{}), err: err} }

// TestAutoProvisioner_SingleFlight pins that concurrent starts for one org
// run one create (acceptance criterion 2).
func TestAutoProvisioner_SingleFlight(t *testing.T) {
	t.Parallel()
	f := newFake(nil)
	a := newAutoProvisioner(f, AutoProvisionOptions{Burst: 100, Rate: 1000}, time.Now)
	defer a.Close()

	var wg sync.WaitGroup
	for range 32 {
		wg.Go(func() {
			if st := a.Start("org-a"); st != ProvisionRunning {
				t.Errorf("Start = %q, want provisioning", st)
			}
		})
	}
	wg.Wait()
	close(f.release)
	a.wait("org-a")
	if got := f.calls.Load(); got != 1 {
		t.Fatalf("creates = %d, want 1", got)
	}
	if st := a.State("org-a"); st != ProvisionNone {
		t.Errorf("State after success = %q, want none", st)
	}
}

// TestAutoProvisioner_Concurrency pins the bound on creates running at once.
func TestAutoProvisioner_Concurrency(t *testing.T) {
	t.Parallel()
	f := newFake(nil)
	a := newAutoProvisioner(f, AutoProvisionOptions{Burst: 100, Rate: 1000, Concurrent: 2}, time.Now)
	defer a.Close()

	orgs := []string{"o1", "o2", "o3", "o4", "o5"}
	for _, o := range orgs {
		a.Start(o)
	}
	deadline := time.Now().Add(5 * time.Second)
	for f.running.Load() < 2 && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	close(f.release)
	for _, o := range orgs {
		a.wait(o)
	}
	if p := f.peak.Load(); p != 2 {
		t.Errorf("peak concurrent creates = %d, want 2", p)
	}
	if c := f.calls.Load(); c != int32(len(orgs)) {
		t.Errorf("creates = %d, want %d", c, len(orgs))
	}
}

// TestAutoProvisioner_RateLimit pins that starts beyond the burst are
// throttled and start no create.
func TestAutoProvisioner_RateLimit(t *testing.T) {
	t.Parallel()
	f := newFake(nil)
	close(f.release)
	a := newAutoProvisioner(f, AutoProvisionOptions{Burst: 2, Rate: 1}, time.Now)
	defer a.Close()

	if a.Start("o1") != ProvisionRunning || a.Start("o2") != ProvisionRunning {
		t.Fatal("the burst should admit two creates")
	}
	if st := a.Start("o3"); st != ProvisionThrottled {
		t.Errorf("third Start = %q, want throttled", st)
	}
	a.wait("o1")
	a.wait("o2")
	if c := f.calls.Load(); c != 2 {
		t.Errorf("creates = %d, want 2", c)
	}
}

// TestAutoProvisioner_FailureWaits pins that a failed org is not retried
// until RetryAfter has passed.
func TestAutoProvisioner_FailureWaits(t *testing.T) {
	t.Parallel()
	f := newFake(errors.New("boom"))
	close(f.release)
	var now atomic.Int64
	now.Store(time.Unix(1000, 0).UnixNano())
	clock := func() time.Time { return time.Unix(0, now.Load()) }
	a := newAutoProvisioner(f, AutoProvisionOptions{Burst: 100, Rate: 1000, RetryAfter: time.Minute}, clock)
	defer a.Close()

	a.Start("org-a")
	a.wait("org-a")
	if st := a.State("org-a"); st != ProvisionFailed {
		t.Fatalf("State = %q, want failed", st)
	}
	if st := a.Start("org-a"); st != ProvisionFailed {
		t.Errorf("Start within RetryAfter = %q, want failed", st)
	}
	if c := f.calls.Load(); c != 1 {
		t.Fatalf("creates = %d, want 1", c)
	}
	now.Add(int64(time.Minute))
	if st := a.State("org-a"); st != ProvisionNone {
		t.Errorf("State after RetryAfter = %q, want none", st)
	}
	if st := a.Start("org-a"); st != ProvisionRunning {
		t.Errorf("Start after RetryAfter = %q, want provisioning", st)
	}
	a.wait("org-a")
	if c := f.calls.Load(); c != 2 {
		t.Errorf("creates = %d, want 2", c)
	}
}

// TestAutoProvisioner_Outcomes pins which create errors count as done.
func TestAutoProvisioner_Outcomes(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name string
		err  error
		want ProvisionState
	}{
		{"exists", ErrTenantExists, ProvisionNone},
		{"in use elsewhere", ErrTenantInUse, ProvisionRunning},
		{"erased", ErrTenantErased, ProvisionFailed},
		{"schema exists", ErrSchemaExists, ProvisionFailed},
		{"collision", ErrSchemaCollision, ProvisionFailed},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			f := newFake(tc.err)
			close(f.release)
			a := newAutoProvisioner(f, AutoProvisionOptions{}, time.Now)
			defer a.Close()
			a.Start("org-a")
			a.wait("org-a")
			if st := a.State("org-a"); st != tc.want {
				t.Errorf("State = %q, want %q", st, tc.want)
			}
		})
	}
}

// TestAutoProvisioner_InUseHolds pins that a create running in another
// process holds the org as running for a while, so polls meanwhile start
// no create and spend no rate-limit token.
func TestAutoProvisioner_InUseHolds(t *testing.T) {
	t.Parallel()
	f := newFake(ErrTenantInUse)
	close(f.release)
	var now atomic.Int64
	now.Store(time.Unix(1000, 0).UnixNano())
	clock := func() time.Time { return time.Unix(0, now.Load()) }
	a := newAutoProvisioner(f, AutoProvisionOptions{Burst: 1, Rate: 1}, clock)
	defer a.Close()

	a.Start("org-a")
	a.wait("org-a")
	for range 5 {
		if st := a.Start("org-a"); st != ProvisionRunning {
			t.Fatalf("Start during the hold = %q, want provisioning", st)
		}
	}
	if c := f.calls.Load(); c != 1 {
		t.Fatalf("creates during the hold = %d, want 1", c)
	}
	now.Add(int64(provisionInUseHold))
	if st := a.State("org-a"); st != ProvisionNone {
		t.Errorf("State after the hold = %q, want none", st)
	}
}

// TestAutoProvisioner_Wait pins that Wait returns once the create ends, or
// when its context is done.
func TestAutoProvisioner_Wait(t *testing.T) {
	t.Parallel()
	f := newFake(nil)
	a := newAutoProvisioner(f, AutoProvisionOptions{}, time.Now)
	defer a.Close()

	if st := a.Wait(t.Context(), "org-none"); st != ProvisionNone {
		t.Errorf("Wait with no create = %q, want none", st)
	}
	a.Start("org-a")
	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Millisecond)
	defer cancel()
	if st := a.Wait(ctx, "org-a"); st != ProvisionRunning {
		t.Errorf("Wait past its deadline = %q, want provisioning", st)
	}
	close(f.release)
	if st := a.Wait(t.Context(), "org-a"); st != ProvisionNone {
		t.Errorf("Wait for a create that succeeded = %q, want none", st)
	}
}

// TestAutoProvisioner_RefusesBadOrgAndClosed pins that an invalid org id and
// a closed provisioner start nothing.
func TestAutoProvisioner_RefusesBadOrgAndClosed(t *testing.T) {
	t.Parallel()
	f := newFake(nil)
	close(f.release)
	a := newAutoProvisioner(f, AutoProvisionOptions{}, time.Now)
	if st := a.Start("bad\x00org"); st != ProvisionFailed {
		t.Errorf("Start(bad org) = %q, want failed", st)
	}
	a.Close()
	if st := a.Start("org-a"); st != ProvisionFailed {
		t.Errorf("Start after Close = %q, want failed", st)
	}
	if c := f.calls.Load(); c != 0 {
		t.Errorf("creates = %d, want 0", c)
	}
}

// TestAutoProvisioner_CloseCancels pins that Close ends a running create.
func TestAutoProvisioner_CloseCancels(t *testing.T) {
	t.Parallel()
	f := newFake(nil) // never released: only the context can end it
	a := newAutoProvisioner(f, AutoProvisionOptions{}, time.Now)
	a.Start("org-a")
	done := make(chan struct{})
	go func() { a.Close(); close(done) }()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("Close did not cancel the running create")
	}
}

func TestParseProvisionMode(t *testing.T) {
	t.Parallel()
	for in, want := range map[string]ProvisionMode{
		"": ProvisionOff, "off": ProvisionOff, "webhook": ProvisionWebhook, "lazy": ProvisionLazy,
	} {
		got, err := ParseProvisionMode(in)
		if err != nil || got != want {
			t.Errorf("ParseProvisionMode(%q) = %q, %v; want %q", in, got, err, want)
		}
	}
	if _, err := ParseProvisionMode("always"); err == nil {
		t.Error("ParseProvisionMode accepted an unknown mode")
	}
	if ProvisionOff.Eager() || ProvisionOff.Lazy() || !ProvisionWebhook.Eager() || ProvisionWebhook.Lazy() ||
		!ProvisionLazy.Eager() || !ProvisionLazy.Lazy() {

		t.Error("mode predicates are wrong")
	}
}

func TestNewAutoProvisioner_RejectsNil(t *testing.T) {
	t.Parallel()
	if _, err := NewAutoProvisioner(nil, AutoProvisionOptions{}); err == nil {
		t.Error("NewAutoProvisioner(nil) succeeded")
	}
}
