//go:build postgres

package tenant_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/store/pgstore"
	"github.com/Sourcehaven-BV/rela/internal/tenant"
)

func (pv provisioned) record(t *testing.T, org string) tenant.Record {
	t.Helper()
	rec, err := pv.r.Lookup(t.Context(), org)
	if err != nil {
		t.Fatalf("Lookup(%s): %v", org, err)
	}
	return rec
}

// TestProvisioner_Apply walks the tenant API's state changes on a real
// database, with the operator's and the IdP's suspensions they must not
// undo (TKT-59HWQ76).
func TestProvisioner_Apply(t *testing.T) {
	pv := newProvisioned(t)
	ctx := t.Context()
	if _, err := pv.p.Create(ctx, "org-a", tenant.CreateOptions{}); err != nil {
		t.Fatal(err)
	}
	apply := func(step string, c tenant.Change) tenant.Record {
		t.Helper()
		rec, err := pv.p.Apply(ctx, "org-a", c)
		if err != nil {
			t.Fatalf("%s: %v", step, err)
		}
		//nolint:contextcheck // record reads with t.Context(), which ctx is
		if got := pv.record(t, "org-a"); got.Status != rec.Status || got.ReadOnly != rec.ReadOnly {
			t.Fatalf("%s: Apply returned %+v, the table holds %+v", step, rec, got)
		}
		return rec
	}

	rec := apply("plan", tenant.Change{
		Plan: new("team"), Entitlements: &tenant.Entitlements{"max_users": json.Number("25"), "sso": true},
	})
	if rec.Plan != "team" || rec.Entitlements["max_users"] != json.Number("25") || rec.Entitlements["sso"] != true {
		t.Fatalf("plan: %+v", rec)
	}

	rec = apply("read_only", tenant.Change{State: new(tenant.StateReadOnly)})
	if rec.Status != tenant.StatusActive || !rec.ReadOnly || rec.Plan != "team" {
		t.Fatalf("read_only: %+v", rec)
	}
	got, err := pv.r.Resolve(ctx, "org-a")
	if err != nil || !got.ReadOnly || got.Plan != "team" || got.Entitlements["sso"] != true {
		t.Fatalf("Resolve(read-only) = %+v, %v", got, err)
	}

	rec = apply("suspend", tenant.Change{State: new(tenant.StateSuspended)})
	if rec.Status != tenant.StatusSuspended || rec.SuspendedBy != tenant.SuspendedByAPI {
		t.Fatalf("suspend: %+v", rec)
	}
	if _, err := pv.r.Resolve(ctx, "org-a"); !errors.Is(err, tenant.ErrTenantInactive) {
		t.Fatalf("Resolve(API-suspended) = %v", err)
	}
	apply("suspend again", tenant.Change{State: new(tenant.StateSuspended)})
	rec = apply("resume", tenant.Change{State: new(tenant.StateActive)})
	if rec.Status != tenant.StatusActive || rec.ReadOnly || rec.SuspendedBy != "" {
		t.Fatalf("resume: %+v", rec)
	}

	// The IdP and the operator hold their own suspensions.
	if _, err := pv.p.SuspendForIDP(ctx, "org-a"); err != nil {
		t.Fatal(err)
	}
	if _, err := pv.p.Apply(ctx, "org-a", tenant.Change{State: new(tenant.StateActive)}); !errors.Is(err,
		tenant.ErrHeldSuspended) {

		t.Fatalf("Apply(active) over the IdP = %v; want ErrHeldSuspended", err)
	}
	apply("plan while suspended", tenant.Change{Plan: new("pro")})
	apply("suspend over the IdP", tenant.Change{State: new(tenant.StateSuspended)})
	if got := pv.record(t, "org-a"); got.SuspendedBy != tenant.SuspendedByIDP || got.Plan != "pro" {
		t.Fatalf("an API suspension took over the IdP's: %+v", got)
	}
	if err := pv.p.Suspend(ctx, "org-a"); err != nil {
		t.Fatal(err)
	}
	if _, err := pv.p.Apply(ctx, "org-a", tenant.Change{State: new(tenant.StateReadOnly)}); !errors.Is(err,
		tenant.ErrHeldSuspended) {

		t.Fatalf("Apply(read_only) over the operator = %v; want ErrHeldSuspended", err)
	}
	if err := pv.p.Resume(ctx, "org-a"); err != nil {
		t.Fatal(err)
	}

	// An API suspension is one the IdP leaves alone too.
	apply("suspend", tenant.Change{State: new(tenant.StateSuspended)})
	if _, err := pv.p.ResumeForIDP(ctx, "org-a"); !errors.Is(err, tenant.ErrOperatorSuspended) {
		t.Fatalf("ResumeForIDP(API-suspended) = %v", err)
	}

	// Clearing.
	rec = apply("clear", tenant.Change{Plan: new(""), Entitlements: &tenant.Entitlements{}})
	if rec.Plan != "" || rec.Entitlements != nil {
		t.Fatalf("clear: %+v", rec)
	}

	// Refusals change nothing.
	if _, err := pv.p.Apply(ctx, "org-a", tenant.Change{Entitlements: &tenant.Entitlements{"Bad": 1.0}}); err == nil {
		t.Fatal("Apply accepted a bad entitlement name")
	}
	if _, err := pv.p.Apply(ctx, "org-b", tenant.Change{Plan: new("x")}); !errors.Is(err, tenant.ErrUnknownTenant) {
		t.Fatalf("Apply(absent org) = %v", err)
	}
}

// TestProvisioner_ApplyOtherStatuses refuses a state change on a
// provisioning tenant, and any change on an erased one.
func TestProvisioner_ApplyOtherStatuses(t *testing.T) {
	pv := newProvisioned(t)
	ctx := t.Context()
	pv.createProvisioning(t, "org-p")
	if _, err := pv.p.Apply(ctx, "org-p", tenant.Change{State: new(tenant.StateSuspended)}); !errors.Is(err,
		tenant.ErrTenantProvisioning) {

		t.Fatalf("Apply(state) on provisioning = %v", err)
	}
	if rec, err := pv.p.Apply(ctx, "org-p", tenant.Change{Plan: new("team")}); err != nil || rec.Plan != "team" {
		t.Fatalf("Apply(plan) on provisioning = %+v, %v", rec, err)
	}

	if _, err := pv.p.Create(ctx, "org-e", tenant.CreateOptions{}); err != nil {
		t.Fatal(err)
	}
	if _, _, err := pv.p.Erase(ctx, "org-e"); err != nil {
		t.Fatal(err)
	}
	if _, err := pv.p.Apply(ctx, "org-e", tenant.Change{Plan: new("x")}); !errors.Is(err, tenant.ErrWrongStatus) {
		t.Fatalf("Apply on erased = %v", err)
	}
	if _, _, err := pv.p.Archive(ctx, "org-e"); !errors.Is(err, tenant.ErrWrongStatus) {
		t.Fatalf("Archive on erased = %v", err)
	}
}

// createProvisioning records org as provisioning, as a create that stopped
// part way leaves it.
func (pv provisioned) createProvisioning(t *testing.T, org string) {
	t.Helper()
	if _, err := pv.p.Create(t.Context(), org, tenant.CreateOptions{}); err != nil {
		t.Fatal(err)
	}
	rec := pv.record(t, org)
	if _, err := pv.conn.Exec(t.Context(), "UPDATE "+tenant.ControlSchema+".tenants SET status = 'provisioning' "+
		"WHERE schema_name = $1", rec.Schema); err != nil {
		t.Fatal(err)
	}
}

// TestProvisioner_Archive archives a tenant, refuses the tenant API's
// changes to it, and lets an operator resume it; a recreated tombstone
// starts with no plan.
func TestProvisioner_Archive(t *testing.T) {
	pv := newProvisioned(t)
	ctx := t.Context()
	if _, err := pv.p.Create(ctx, "org-a", tenant.CreateOptions{}); err != nil {
		t.Fatal(err)
	}
	if _, err := pv.p.Apply(ctx, "org-a", tenant.Change{Plan: new("team"), State: new(tenant.StateReadOnly)}); err != nil {
		t.Fatal(err)
	}
	if err := pv.p.Suspend(ctx, "org-a"); err != nil {
		t.Fatal(err)
	}
	rec, changed, err := pv.p.Archive(ctx, "org-a")
	if err != nil || !changed || rec.SuspendedBy != tenant.SuspendedByArchive || rec.ArchivedAt.IsZero() {
		t.Fatalf("Archive = %+v, %v, %v", rec, changed, err)
	}
	if time.Since(rec.ArchivedAt) > time.Minute {
		t.Errorf("archived_at = %v", rec.ArchivedAt)
	}
	again, changed, err := pv.p.Archive(ctx, "org-a")
	if err != nil || changed || !again.ArchivedAt.Equal(rec.ArchivedAt) {
		t.Fatalf("second Archive = %+v, %v, %v; want no change", again, changed, err)
	}
	if _, err := pv.r.Resolve(ctx, "org-a"); !errors.Is(err, tenant.ErrTenantInactive) {
		t.Fatalf("Resolve(archived) = %v", err)
	}
	for _, c := range []tenant.Change{{State: new(tenant.StateActive)}, {Plan: new("pro")}} {
		if _, err := pv.p.Apply(ctx, "org-a", c); !errors.Is(err, tenant.ErrArchived) {
			t.Fatalf("Apply(%+v) on archived = %v; want ErrArchived", c, err)
		}
	}
	if _, err := pv.p.ResumeForIDP(ctx, "org-a"); !errors.Is(err, tenant.ErrOperatorSuspended) {
		t.Fatalf("ResumeForIDP(archived) = %v", err)
	}
	if err := pv.p.Suspend(ctx, "org-a"); err != nil {
		t.Fatal(err)
	}
	if got := pv.record(t, "org-a"); !got.Archived() || got.SuspendedBy != tenant.SuspendedByArchive {
		t.Fatalf("operator suspend of an archived tenant: %+v; want it still archived", got)
	}

	if err := pv.p.Resume(ctx, "org-a"); err != nil {
		t.Fatal(err)
	}
	rec = pv.record(t, "org-a")
	if rec.Status != tenant.StatusActive || !rec.ArchivedAt.IsZero() || rec.SuspendedBy != "" || !rec.ReadOnly {
		t.Fatalf("after operator resume: %+v; want active, unarchived, still read-only", rec)
	}

	if _, _, err := pv.p.Erase(ctx, "org-a"); err != nil {
		t.Fatal(err)
	}
	if got := pv.record(t, "org-a"); got.ReadOnly || got.Plan != "" || got.Entitlements != nil {
		t.Fatalf("erased tenant kept its plan: %+v", got)
	}
	if _, err := pv.p.Create(ctx, "org-a", tenant.CreateOptions{Recreate: true}); err != nil {
		t.Fatal(err)
	}
	if rec := pv.record(t, "org-a"); rec.ReadOnly || rec.Plan != "" || rec.Entitlements != nil ||
		!rec.ArchivedAt.IsZero() {

		t.Fatalf("recreated tenant kept its old plan: %+v", rec)
	}
}

// TestClusterResolver_Usage reads a tenant's users, active users and
// storage from its schema.
func TestClusterResolver_Usage(t *testing.T) {
	pv := newProvisioned(t)
	ctx := t.Context()
	a, err := pv.p.Create(ctx, "org-a", tenant.CreateOptions{})
	if err != nil {
		t.Fatal(err)
	}
	conn, err := pgstore.ConnectDirect(ctx, a.DSN)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = conn.Close(context.Background()) })
	now := time.Now()
	recent, old := now.Add(-24*time.Hour).UnixMicro(), now.Add(-40*24*time.Hour).UnixMicro()
	if _, err = conn.Exec(ctx,
		`INSERT INTO entities (id, type) VALUES ('U-1','person'),('U-2','person'),('T-1','task')`); err != nil {
		t.Fatal(err)
	}
	if _, err = conn.Exec(ctx, `INSERT INTO audit_log (seq, at, op, principal_user, doc, prev_hash, hash) VALUES
		(1, $1, 'create', 'alice', '{}', '', 'h1'),
		(2, $1, 'update', 'alice', '{}', 'h1', 'h2'),
		(3, $1, 'update', 'system:flow', '{}', 'h2', 'h3'),
		(4, $2, 'update', 'bob', '{}', 'h3', 'h4'),
		(5, $1, 'update', 'carol', '{}', 'h4', 'h5')`, recent, old); err != nil {
		t.Fatal(err)
	}

	u, err := pv.r.Usage(ctx, "org-a", "person", now)
	if err != nil {
		t.Fatal(err)
	}
	if u.Users != 2 || u.ActiveUsers != 2 || u.StorageBytes <= 0 || !u.MeasuredAt.Equal(now.UTC()) {
		t.Fatalf("Usage = %+v; want 2 users, 2 active, some storage", u)
	}
	if u, err := pv.r.Usage(ctx, "org-a", "", now); err != nil || u.Users != 0 {
		t.Fatalf("Usage with no user type = %+v, %v; want 0 users", u, err)
	}
	if _, err := pv.r.Usage(ctx, "org-b", "person", now); !errors.Is(err, tenant.ErrUnknownTenant) {
		t.Fatalf("Usage(absent) = %v", err)
	}
	if _, err := conn.Exec(ctx, "DROP TABLE audit_log CASCADE"); err != nil {
		t.Fatal(err)
	}
	if u, err := pv.r.Usage(ctx, "org-a", "person", now); err != nil || u.Users != 2 || u.ActiveUsers != 0 {
		t.Fatalf("Usage without an audit log = %+v, %v; want 2 users, 0 active", u, err)
	}
	if _, _, err := pv.p.Erase(ctx, "org-a"); err != nil {
		t.Fatal(err)
	}
	if _, err := pv.r.Usage(ctx, "org-a", "person", now); !errors.Is(err, tenant.ErrUnknownTenant) {
		t.Fatalf("Usage(erased) = %v", err)
	}
	pv.createProvisioning(t, "org-p")
	if _, err := pv.r.Usage(ctx, "org-p", "person", now); !errors.Is(err, tenant.ErrTenantProvisioning) {
		t.Fatalf("Usage(provisioning) = %v", err)
	}
}

// TestProvisioner_UpgradesControlTableForTenantAPI pins that a control
// table from before the tenant API reads as it is, and gets the new
// columns and causes on the first lifecycle command.
func TestProvisioner_UpgradesControlTableForTenantAPI(t *testing.T) {
	pv := newProvisioned(t)
	ctx := t.Context()
	if _, err := pv.p.Create(ctx, "org-a", tenant.CreateOptions{}); err != nil {
		t.Fatal(err)
	}
	if _, err := pv.conn.Exec(ctx, "ALTER TABLE "+tenant.ControlSchema+".tenants "+
		"DROP COLUMN read_only, DROP COLUMN plan, DROP COLUMN entitlements, DROP COLUMN archived_at, "+
		"DROP COLUMN api_hold, "+
		"DROP CONSTRAINT tenants_suspended_by_check, "+
		"ADD CONSTRAINT tenants_suspended_by_check CHECK (suspended_by IN ('operator','idp'))"); err != nil {
		t.Fatal(err)
	}
	if rec := pv.record(t, "org-a"); rec.Status != tenant.StatusActive || rec.ReadOnly || rec.Plan != "" {
		t.Fatalf("old table: %+v", rec)
	}
	if _, err := pv.p.Apply(ctx, "org-a", tenant.Change{Plan: new("team"), State: new(tenant.StateSuspended)}); err != nil {
		t.Fatalf("Apply on an old table: %v", err)
	}
	if _, _, err := pv.p.Archive(ctx, "org-a"); err != nil {
		t.Fatalf("Archive after the upgrade: %v", err)
	}
	if rec := pv.record(t, "org-a"); rec.Plan != "team" || rec.SuspendedBy != tenant.SuspendedByArchive {
		t.Fatalf("after the upgrade: %+v", rec)
	}
}

// TestRegistry_LeaseCarriesCurrentState pins that a lease on an open
// tenant carries the access state and plan resolved for this request, not
// the ones the tenant was opened with.
func TestRegistry_LeaseCarriesCurrentState(t *testing.T) {
	pv := newProvisioned(t)
	ctx := t.Context()
	if _, err := pv.p.Create(ctx, "org-a", tenant.CreateOptions{}); err != nil {
		t.Fatal(err)
	}
	reg, err := tenant.NewRegistry(pv.r, func(context.Context, tenant.Tenant) (*revokeCounter, error) {
		return &revokeCounter{}, nil
	}, 4)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = reg.Close() })
	first, err := reg.Acquire(ctx, "org-a")
	if err != nil {
		t.Fatal(err)
	}
	first.Release()
	if first.Tenant.ReadOnly {
		t.Fatal("a fresh tenant is read-only")
	}
	if _, err = pv.p.Apply(ctx, "org-a", tenant.Change{State: new(tenant.StateReadOnly), Plan: new("team")}); err != nil {
		t.Fatal(err)
	}
	second, err := reg.Acquire(ctx, "org-a")
	if err != nil {
		t.Fatal(err)
	}
	defer second.Release()
	if !second.Tenant.ReadOnly || second.Tenant.Plan != "team" {
		t.Fatalf("lease on the open tenant = %+v; want read-only on plan team", second.Tenant)
	}
}

// TestProvisioner_IDPAndAPIHolds pins that neither the IdP nor the tenant
// API lifts the other's suspension, in either order (TKT-59HWQ76).
func TestProvisioner_IDPAndAPIHolds(t *testing.T) {
	pv := newProvisioned(t)
	ctx := t.Context()
	state := func(org string, want tenant.Status, by tenant.SuspendCause, hold bool) {
		t.Helper()
		got := pv.record(t, org)
		if got.Status != want || got.SuspendedBy != by || got.APIHold != hold {
			t.Fatalf("%s = %s by %q hold %v; want %s by %q hold %v",
				org, got.Status, got.SuspendedBy, got.APIHold, want, by, hold)
		}
	}
	resumeAPI := tenant.Change{State: new(tenant.StateActive)}
	suspendAPI := tenant.Change{State: new(tenant.StateSuspended)}

	// The tenant API first, then the IdP.
	if _, err := pv.p.Create(ctx, "org-a", tenant.CreateOptions{}); err != nil {
		t.Fatal(err)
	}
	if _, err := pv.p.Apply(ctx, "org-a", suspendAPI); err != nil {
		t.Fatal(err)
	}
	state("org-a", tenant.StatusSuspended, tenant.SuspendedByAPI, true)
	if changed, err := pv.p.SuspendForIDP(ctx, "org-a"); err != nil || !changed {
		t.Fatalf("SuspendForIDP over an API suspension = %v, %v; want the IdP to take the cause", changed, err)
	}
	state("org-a", tenant.StatusSuspended, tenant.SuspendedByIDP, true)
	if _, err := pv.p.Apply(ctx, "org-a", resumeAPI); !errors.Is(err, tenant.ErrHeldSuspended) {
		t.Fatalf("API resume while the IdP holds the tenant = %v; want ErrHeldSuspended", err)
	}
	state("org-a", tenant.StatusSuspended, tenant.SuspendedByIDP, true)
	if changed, err := pv.p.ResumeForIDP(ctx, "org-a"); err != nil || changed {
		t.Fatalf("IdP resume under an API hold = %v, %v; want it still suspended", changed, err)
	}
	state("org-a", tenant.StatusSuspended, tenant.SuspendedByAPI, true)
	if _, err := pv.p.Apply(ctx, "org-a", resumeAPI); err != nil {
		t.Fatal(err)
	}
	state("org-a", tenant.StatusActive, "", false)

	// The IdP first, then the tenant API.
	if _, err := pv.p.Create(ctx, "org-b", tenant.CreateOptions{}); err != nil {
		t.Fatal(err)
	}
	if _, err := pv.p.SuspendForIDP(ctx, "org-b"); err != nil {
		t.Fatal(err)
	}
	if _, err := pv.p.Apply(ctx, "org-b", suspendAPI); err != nil {
		t.Fatal(err)
	}
	state("org-b", tenant.StatusSuspended, tenant.SuspendedByIDP, true)
	if changed, err := pv.p.ResumeForIDP(ctx, "org-b"); err != nil || changed {
		t.Fatalf("IdP resume under an API hold = %v, %v; want it still suspended", changed, err)
	}
	state("org-b", tenant.StatusSuspended, tenant.SuspendedByAPI, true)

	// An operator resume lifts every hold.
	if err := pv.p.Resume(ctx, "org-b"); err != nil {
		t.Fatal(err)
	}
	state("org-b", tenant.StatusActive, "", false)
	if changed, err := pv.p.ResumeForIDP(ctx, "org-b"); err != nil || changed {
		t.Fatalf("IdP resume of an active tenant = %v, %v; want no change", changed, err)
	}
}
