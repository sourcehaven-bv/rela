//go:build postgres

package tenant_test

import (
	"errors"
	"testing"

	"github.com/jackc/pgx/v5"

	"github.com/Sourcehaven-BV/rela/internal/tenant"
)

// TestProvisioner_EraseLeavesTombstone pins that an erasure leaves an
// erased control row, that nothing but an explicit recreate turns it back
// into a tenant, and that the recreated tenant starts empty (TKT-TNPRV8).
func TestProvisioner_EraseLeavesTombstone(t *testing.T) {
	pv := newProvisioned(t)
	ctx := t.Context()
	var (
		res     tenant.Erasure
		rec     tenant.Record
		changed bool
	)
	a, err := pv.p.Create(ctx, "org-a", tenant.CreateOptions{})
	if err != nil {
		t.Fatal(err)
	}
	marker := pgx.Identifier{a.Schema, "marker"}.Sanitize()
	if _, err = pv.conn.Exec(ctx, "CREATE TABLE "+marker+" (v text)"); err != nil {
		t.Fatal(err)
	}
	if _, err = pv.p.SuspendForIDP(ctx, "org-a"); err != nil {
		t.Fatal(err)
	}
	if _, res, err = pv.p.Erase(ctx, "org-a"); err != nil || !res.SchemaDropped {
		t.Fatalf("Erase = %+v, %v", res, err)
	}

	rec, err = pv.r.Lookup(ctx, "org-a")
	if err != nil {
		t.Fatalf("Lookup(erased) = %v", err)
	}
	if rec.Status != tenant.StatusErased || rec.ErasedAt.IsZero() || rec.SuspendedBy != "" || rec.SchemaExists {
		t.Fatalf("tombstone = %+v; want erased, erased_at set, no cause, no schema", rec)
	}

	_, err = pv.r.Resolve(ctx, "org-a")
	if !errors.Is(err, tenant.ErrUnknownTenant) || !errors.Is(err, tenant.ErrTenantErased) {
		t.Fatalf("Resolve(erased) = %v; want ErrUnknownTenant and ErrTenantErased", err)
	}
	for _, sentinel := range []error{tenant.ErrTenantAbsent, tenant.ErrTenantProvisioning, tenant.ErrTenantInactive} {
		if errors.Is(err, sentinel) {
			t.Errorf("Resolve(erased) = %v; must not carry %v", err, sentinel)
		}
	}

	// A second erase succeeds and changes nothing.
	if _, res, err = pv.p.Erase(ctx, "org-a"); err != nil || res.SchemaDropped || res.AuditRemoved {
		t.Errorf("Erase(erased) = %+v, %v; want success with nothing removed", res, err)
	}
	if again, _ := pv.r.Lookup(ctx, "org-a"); !again.ErasedAt.Equal(rec.ErasedAt) {
		t.Errorf("a second erase moved erased_at from %v to %v", rec.ErasedAt, again.ErasedAt)
	}

	// The IdP and the operator's status changes leave the tombstone alone.
	if changed, err = pv.p.SuspendForIDP(ctx, "org-a"); err != nil || changed {
		t.Errorf("SuspendForIDP(erased) = %v, %v; want a no-op", changed, err)
	}
	if changed, err = pv.p.ResumeForIDP(ctx, "org-a"); err != nil || changed {
		t.Errorf("ResumeForIDP(erased) = %v, %v; want a no-op", changed, err)
	}
	if err = pv.p.Resume(ctx, "org-a"); !errors.Is(err, tenant.ErrWrongStatus) {
		t.Errorf("Resume(erased) = %v; want ErrWrongStatus", err)
	}
	if err = pv.p.Suspend(ctx, "org-a"); !errors.Is(err, tenant.ErrWrongStatus) {
		t.Errorf("Suspend(erased) = %v; want ErrWrongStatus", err)
	}

	if _, err = pv.p.Create(ctx, "org-a", tenant.CreateOptions{}); !errors.Is(err, tenant.ErrTenantErased) {
		t.Fatalf("Create(erased) = %v; want ErrTenantErased", err)
	}
	if again, _ := pv.r.Lookup(ctx, "org-a"); again.Status != tenant.StatusErased {
		t.Fatalf("a refused create changed the tombstone to %s", again.Status)
	}

	if _, err = pv.p.Create(ctx, "org-a", tenant.CreateOptions{Recreate: true}); err != nil {
		t.Fatalf("Create -recreate: %v", err)
	}
	rec, err = pv.r.Lookup(ctx, "org-a")
	if err != nil || rec.Status != tenant.StatusActive || !rec.ErasedAt.IsZero() || rec.SuspendedBy != "" {
		t.Fatalf("recreated = %+v, %v; want active with no erased_at or cause", rec, err)
	}
	if _, err = pv.r.Resolve(ctx, "org-a"); err != nil {
		t.Fatalf("Resolve(recreated) = %v", err)
	}
	var stale bool
	if err = pv.conn.QueryRow(ctx, "SELECT to_regclass($1) IS NOT NULL", marker).Scan(&stale); err != nil {
		t.Fatal(err)
	}
	if stale {
		t.Error("the recreated tenant holds a table from before the erasure")
	}
}

// TestProvisioner_UpgradesPreTombstoneTable pins that a control table made
// before the erased status existed is read as it is, and is upgraded by the
// next lifecycle command: the column is added and every old status check is
// replaced, whatever its name.
func TestProvisioner_UpgradesPreTombstoneTable(t *testing.T) {
	pv := newProvisioned(t)
	ctx := t.Context()
	if _, err := pv.p.Create(ctx, "org-a", tenant.CreateOptions{}); err != nil {
		t.Fatal(err)
	}
	const table = tenant.ControlSchema + ".tenants"
	if _, err := pv.conn.Exec(ctx, "ALTER TABLE "+table+" DROP COLUMN erased_at; "+
		"ALTER TABLE "+table+" DROP CONSTRAINT tenants_status_check; "+
		"ALTER TABLE "+table+" ADD CONSTRAINT old_status_check "+
		"CHECK (status IN ('provisioning', 'active', 'suspended', 'erasing'))"); err != nil {
		t.Fatal(err)
	}
	if _, err := pv.r.Resolve(ctx, "org-a"); err != nil {
		t.Fatalf("Resolve on the old table: %v", err)
	}
	if _, _, err := pv.p.Erase(ctx, "org-a"); err != nil {
		t.Fatalf("Erase on the old table: %v", err)
	}
	rec, err := pv.r.Lookup(ctx, "org-a")
	if err != nil || rec.Status != tenant.StatusErased || rec.ErasedAt.IsZero() {
		t.Fatalf("after the upgrade = %+v, %v; want erased with erased_at", rec, err)
	}
	var checks []string
	rows, err := pv.conn.Query(ctx, "SELECT con.conname FROM pg_constraint con "+
		"JOIN pg_attribute a ON a.attrelid = con.conrelid AND a.attnum = ANY (con.conkey) "+
		"WHERE con.conrelid = $1::regclass AND con.contype = 'c' AND a.attname = 'status'", table)
	if err != nil {
		t.Fatal(err)
	}
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			t.Fatal(err)
		}
		checks = append(checks, name)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	if len(checks) != 1 || checks[0] != "tenants_status_check" {
		t.Errorf("status checks after the upgrade = %v; want only tenants_status_check", checks)
	}
}
