//go:build postgres

package tenant_test

import (
	"context"
	"errors"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/tenant"
)

// lookup returns org-a's status and suspension cause.
func (pv provisioned) lookup(t *testing.T) (tenant.Status, tenant.SuspendCause) {
	t.Helper()
	rec, err := pv.r.Lookup(t.Context(), "org-a")
	if err != nil {
		t.Fatalf("Lookup(org-a): %v", err)
	}
	return rec.Status, rec.SuspendedBy
}

// TestProvisioner_IDPSuspension walks the IdP transitions and their
// interplay with an operator's on a real database.
func TestProvisioner_IDPSuspension(t *testing.T) {
	pv := newProvisioned(t)
	ctx := t.Context()
	if _, err := pv.p.Create(ctx, "org-a", tenant.CreateOptions{}); err != nil {
		t.Fatal(err)
	}
	expect := func(step string, wantStatus tenant.Status, wantBy tenant.SuspendCause) {
		t.Helper()
		if st, by := pv.lookup(t); st != wantStatus || by != wantBy {
			t.Fatalf("%s: %s by %q; want %s by %q", step, st, by, wantStatus, wantBy)
		}
	}

	// The IdP suspends and reactivates.
	if changed, err := pv.p.SuspendForIDP(ctx, "org-a"); err != nil || !changed {
		t.Fatalf("SuspendForIDP = %v, %v", changed, err)
	}
	expect("IdP suspend", tenant.StatusSuspended, tenant.SuspendedByIDP)
	if _, err := pv.r.Resolve(ctx, "org-a"); !errors.Is(err, tenant.ErrTenantInactive) {
		t.Fatalf("Resolve(IdP-suspended) = %v", err)
	}
	if changed, err := pv.p.SuspendForIDP(ctx, "org-a"); err != nil || changed {
		t.Fatalf("second SuspendForIDP = %v, %v; want no change", changed, err)
	}
	if changed, err := pv.p.ResumeForIDP(ctx, "org-a"); err != nil || !changed {
		t.Fatalf("ResumeForIDP = %v, %v", changed, err)
	}
	expect("IdP resume", tenant.StatusActive, "")
	if changed, err := pv.p.ResumeForIDP(ctx, "org-a"); err != nil || changed {
		t.Fatalf("ResumeForIDP(active) = %v, %v; want no change", changed, err)
	}

	// An operator suspension is never undone by the IdP, and an IdP
	// suspension of it keeps the operator as the cause.
	if err := pv.p.Suspend(ctx, "org-a"); err != nil {
		t.Fatal(err)
	}
	expect("operator suspend", tenant.StatusSuspended, tenant.SuspendedByOperator)
	if changed, err := pv.p.SuspendForIDP(ctx, "org-a"); err != nil || changed {
		t.Fatalf("SuspendForIDP(operator-suspended) = %v, %v; want no change", changed, err)
	}
	expect("IdP suspend over operator", tenant.StatusSuspended, tenant.SuspendedByOperator)
	if _, err := pv.p.ResumeForIDP(ctx, "org-a"); !errors.Is(err, tenant.ErrOperatorSuspended) {
		t.Fatalf("ResumeForIDP(operator-suspended) = %v; want ErrOperatorSuspended", err)
	}
	expect("IdP resume over operator", tenant.StatusSuspended, tenant.SuspendedByOperator)

	// The operator resumes, which clears the cause.
	if err := pv.p.Resume(ctx, "org-a"); err != nil {
		t.Fatal(err)
	}
	expect("operator resume", tenant.StatusActive, "")

	// An operator suspension of an IdP-suspended tenant takes it over.
	if _, err := pv.p.SuspendForIDP(ctx, "org-a"); err != nil {
		t.Fatal(err)
	}
	if err := pv.p.Suspend(ctx, "org-a"); err != nil {
		t.Fatal(err)
	}
	expect("operator suspend over IdP", tenant.StatusSuspended, tenant.SuspendedByOperator)
	if _, err := pv.p.ResumeForIDP(ctx, "org-a"); !errors.Is(err, tenant.ErrOperatorSuspended) {
		t.Fatalf("ResumeForIDP after an operator took over = %v", err)
	}
	// The operator may resume an IdP suspension too.
	if err := pv.p.Resume(ctx, "org-a"); err != nil {
		t.Fatal(err)
	}
	if _, err := pv.p.SuspendForIDP(ctx, "org-a"); err != nil {
		t.Fatal(err)
	}
	if err := pv.p.Resume(ctx, "org-a"); err != nil {
		t.Fatal(err)
	}
	expect("operator resume of IdP suspension", tenant.StatusActive, "")

	// An org with no tenant is unknown to both.
	if _, err := pv.p.SuspendForIDP(ctx, "org-z"); !errors.Is(err, tenant.ErrUnknownTenant) {
		t.Errorf("SuspendForIDP(no tenant) = %v", err)
	}
	if _, err := pv.p.ResumeForIDP(ctx, "org-z"); !errors.Is(err, tenant.ErrUnknownTenant) {
		t.Errorf("ResumeForIDP(no tenant) = %v", err)
	}
}

// TestProvisioner_IDPSuspensionOtherStatuses pins that the IdP leaves a
// provisioning and an erasing tenant alone, and asks for a retry on the
// first.
func TestProvisioner_IDPSuspensionOtherStatuses(t *testing.T) {
	pv := newProvisioned(t)
	ctx := t.Context()
	a, err := pv.p.Create(ctx, "org-a", tenant.CreateOptions{})
	if err != nil {
		t.Fatal(err)
	}
	set := func(status tenant.Status) {
		t.Helper()
		if _, err := pv.conn.Exec(ctx, "UPDATE "+tenant.ControlSchema+".tenants SET status = $1 WHERE schema_name = $2",
			string(status), a.Schema); err != nil {
			t.Fatal(err)
		}
	}
	set(tenant.StatusProvisioning)
	if _, err := pv.p.SuspendForIDP(ctx, "org-a"); !errors.Is(err, tenant.ErrTenantProvisioning) {
		t.Errorf("SuspendForIDP(provisioning) = %v; want ErrTenantProvisioning", err)
	}
	if changed, err := pv.p.ResumeForIDP(ctx, "org-a"); err != nil || changed {
		t.Errorf("ResumeForIDP(provisioning) = %v, %v; want no change", changed, err)
	}
	set(tenant.StatusErasing)
	for name, op := range map[string]func(context.Context, string) (bool, error){
		"suspend": pv.p.SuspendForIDP, "resume": pv.p.ResumeForIDP,
	} {
		if changed, err := op(ctx, "org-a"); err != nil || changed {
			t.Errorf("%s on an erasing tenant = %v, %v; want no change", name, changed, err)
		}
	}
	if st, _ := pv.lookup(t); st != tenant.StatusErasing {
		t.Errorf("status = %s, want erasing", st)
	}
}

// TestProvisioner_UpgradesOldControlTable pins that a control table made
// before suspended_by existed is read by the resolver as it is, and gets
// the column on the first lifecycle command. A suspension recorded before
// has no cause, so the IdP does not resume it.
func TestProvisioner_UpgradesOldControlTable(t *testing.T) {
	pv := newProvisioned(t)
	ctx := t.Context()
	a, err := pv.p.Create(ctx, "org-a", tenant.CreateOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = pv.conn.Exec(ctx, "ALTER TABLE "+tenant.ControlSchema+".tenants DROP COLUMN suspended_by; "+
		"UPDATE "+tenant.ControlSchema+".tenants SET status = 'suspended' WHERE schema_name = '"+a.Schema+"'"); err != nil {
		t.Fatal(err)
	}
	if st, by := pv.lookup(t); st != tenant.StatusSuspended || by != "" {
		t.Fatalf("old table: %s by %q; want suspended with no cause", st, by)
	}
	if _, err = pv.p.ResumeForIDP(ctx, "org-a"); !errors.Is(err, tenant.ErrOperatorSuspended) {
		t.Fatalf("ResumeForIDP(suspended before the column) = %v; want ErrOperatorSuspended", err)
	}
	if err = pv.p.Resume(ctx, "org-a"); err != nil {
		t.Fatalf("Resume after the upgrade: %v", err)
	}
	if changed, err := pv.p.SuspendForIDP(ctx, "org-a"); err != nil || !changed {
		t.Fatalf("SuspendForIDP after the upgrade = %v, %v", changed, err)
	}
	if _, by := pv.lookup(t); by != tenant.SuspendedByIDP {
		t.Fatalf("cause after the upgrade = %q", by)
	}
}

// TestRegistry_IDPSuspensionClosesOpenTenant pins that an IdP suspension
// closes an open tenant at once when the host revalidates the org, as an
// operator suspension does at the next request or revalidation pass.
func TestRegistry_IDPSuspensionClosesOpenTenant(t *testing.T) {
	pv := newProvisioned(t)
	ctx := t.Context()
	if _, err := pv.p.Create(ctx, "org-a", tenant.CreateOptions{}); err != nil {
		t.Fatal(err)
	}
	var opened *revokeCounter
	reg, err := tenant.NewRegistry(pv.r, func(context.Context, tenant.Tenant) (*revokeCounter, error) {
		opened = &revokeCounter{}
		return opened, nil
	}, 4)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = reg.Close() })
	held, err := reg.Acquire(ctx, "org-a")
	if err != nil {
		t.Fatal(err)
	}

	if _, err = pv.p.SuspendForIDP(ctx, "org-a"); err != nil {
		t.Fatal(err)
	}
	reg.RevalidateOrg(ctx, "org-a")
	if opened.revoked() != 1 {
		t.Fatalf("the open tenant was told to end its streams %d times, want 1", opened.revoked())
	}
	if _, err = reg.Acquire(ctx, "org-a"); !errors.Is(err, tenant.ErrTenantInactive) {
		t.Fatalf("Acquire(IdP-suspended) = %v; want ErrTenantInactive", err)
	}
	held.Release()
	reg.WaitClosing()
	if opened.count() != 1 || reg.Resident() != 0 {
		t.Fatalf("closes %d, resident %d; want 1 and 0", opened.count(), reg.Resident())
	}
}
