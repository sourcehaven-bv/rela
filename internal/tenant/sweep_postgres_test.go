//go:build postgres

package tenant_test

import (
	"testing"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/Sourcehaven-BV/rela/internal/tenant"
)

// TestClusterResolver_DueTenants pins the sweep's query on a real database
// (TKT-ZOU5BCT): it reports an active tenant whose flow run or channel
// timer is due, ignores one that is not due yet or has nothing pending,
// and skips suspended tenants.
func TestClusterResolver_DueTenants(t *testing.T) {
	pv := newProvisioned(t)
	ctx := t.Context()
	now := time.Now()

	tenants := map[string]tenant.Tenant{}
	for _, org := range []string{"org-flow", "org-chan", "org-later", "org-idle", "org-off", "org-old", "org-broken"} {
		tn, err := pv.p.Create(ctx, org, false)
		if err != nil {
			t.Fatalf("Create(%s): %v", org, err)
		}
		tenants[org] = tn
	}
	exec := func(org, sql string, args ...any) {
		t.Helper()
		if _, err := pv.conn.Exec(ctx, sql, args...); err != nil {
			t.Fatalf("%s: %v", org, err)
		}
	}
	flowRun := func(org string, due time.Time) {
		t.Helper()
		s := tenants[org].Schema
		exec(org, `INSERT INTO `+pgx.Identifier{s, "flow_definitions"}.Sanitize()+
			` (flow, version, def, digest, saved_at) VALUES ('f', 'v1', '\x00', 'd', 0)`)
		exec(org, `INSERT INTO `+pgx.Identifier{s, "flow_runs"}.Sanitize()+
			` (id, flow, version, subject_id, singleton, next_due_at, created_at, updated_at, rev, doc)
			VALUES ('r1', 'f', 'v1', 'TKT-1', 0, $1, 0, 0, 1, '{}')`, due.UnixMicro())
	}
	flowRun("org-flow", now.Add(-time.Minute))
	flowRun("org-later", now.Add(time.Hour))
	flowRun("org-off", now.Add(-time.Minute))
	exec("org-chan", `INSERT INTO `+pgx.Identifier{tenants["org-chan"].Schema, "channels"}.Sanitize()+
		` (anchor_id, type, state, created_by, created_at, session_next_due_at) VALUES ('TKT-1', 'chat', 'open', 'u', 0, $1)`,
		now.Add(-2*time.Minute).UnixMicro())
	// A tenant last opened before channels gained session_next_due_at is
	// still swept for its flows.
	flowRun("org-old", now.Add(-time.Minute))
	exec("org-old", `ALTER TABLE `+pgx.Identifier{tenants["org-old"].Schema, "channels"}.Sanitize()+
		` DROP COLUMN session_next_due_at`)
	// A tenant whose due query fails is skipped, and the others still are
	// swept.
	flowRun("org-broken", now.Add(-time.Minute))
	broken := pgx.Identifier{tenants["org-broken"].Schema, "channels"}.Sanitize()
	exec("org-broken", `DROP TABLE `+broken+` CASCADE`)
	exec("org-broken", `CREATE TABLE `+broken+` (session_next_due_at text, tomb text, state text)`)
	if err := pv.p.Suspend(ctx, "org-off"); err != nil {
		t.Fatal(err)
	}

	due, err := pv.r.DueTenants(ctx, now)
	if err != nil {
		t.Fatalf("DueTenants: %v", err)
	}
	got := map[string]time.Time{}
	for _, d := range due {
		got[d.OrgID] = d.DueAt
	}
	if len(got) != 3 || got["org-flow"].IsZero() || got["org-chan"].IsZero() || got["org-old"].IsZero() {
		t.Fatalf("DueTenants = %+v, want org-flow, org-chan and org-old", due)
	}
	if d := got["org-flow"].Sub(now.Add(-time.Minute)); d < -time.Millisecond || d > time.Millisecond {
		t.Errorf("org-flow DueAt = %v, want about %v", got["org-flow"], now.Add(-time.Minute))
	}
	later, err := pv.r.DueTenants(ctx, now.Add(2*time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if len(later) != 4 {
		t.Errorf("DueTenants(+2h) = %+v, want org-flow, org-chan, org-old and org-later", later)
	}
}
