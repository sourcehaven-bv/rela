//go:build postgres

package tenant_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"

	"github.com/Sourcehaven-BV/rela/internal/appbuild/backendtest"
	"github.com/Sourcehaven-BV/rela/internal/tenant"
)

// provisioned is a private database with a provisioner and a resolver on it.
type provisioned struct {
	base      string
	auditRoot string
	p         *tenant.Provisioner
	r         *tenant.ClusterResolver
	conn      *pgx.Conn
}

func newProvisioned(t *testing.T) provisioned {
	t.Helper()
	base := backendtest.Database(t)
	auditRoot := tenant.AuditRoot(t.TempDir())
	p, err := tenant.NewProvisioner(base, auditRoot)
	if err != nil {
		t.Fatal(err)
	}
	r, err := tenant.NewClusterResolver(t.Context(), base)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = r.Close() })
	conn, err := pgx.Connect(t.Context(), base)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = conn.Close(context.Background()) })
	return provisioned{base: base, auditRoot: auditRoot, p: p, r: r, conn: conn}
}

func (pv provisioned) schemaExists(t *testing.T, schema string) bool {
	t.Helper()
	var ok bool
	if err := pv.conn.QueryRow(t.Context(),
		"SELECT EXISTS (SELECT 1 FROM pg_namespace WHERE nspname = $1)", schema).Scan(&ok); err != nil {
		t.Fatal(err)
	}
	return ok
}

// TestProvisioner_Lifecycle walks one tenant through create, suspend,
// resume and erase on a real database, checking what the resolver serves at
// each step.
func TestProvisioner_Lifecycle(t *testing.T) {
	pv := newProvisioned(t)
	ctx := t.Context()

	if err := pv.r.Check(ctx); err == nil || !strings.Contains(err.Error(), "tenant create") {
		t.Fatalf("Check before any tenant = %v; want a pointer at tenant create", err)
	}
	a, err := pv.p.Create(ctx, "org-a", false)
	if err != nil {
		t.Fatalf("Create(org-a): %v", err)
	}
	b, err := pv.p.Create(ctx, "org-b", false)
	if err != nil {
		t.Fatalf("Create(org-b): %v", err)
	}
	if a.Schema == b.Schema || !pv.schemaExists(t, a.Schema) {
		t.Fatalf("schemas %q and %q; want two distinct existing schemas", a.Schema, b.Schema)
	}
	if err = pv.r.Check(ctx); err != nil {
		t.Fatalf("Check: %v", err)
	}
	got, err := pv.r.Resolve(ctx, "org-a")
	if err != nil || got.Schema != a.Schema {
		t.Fatalf("Resolve(org-a) = %+v, %v", got, err)
	}
	// The tenant DSN lands in the tenant's schema with the store migrated.
	conn, err := pgx.Connect(ctx, got.DSN)
	if err != nil {
		t.Fatal(err)
	}
	var here string
	var tables int
	if err = conn.QueryRow(ctx, "SELECT current_schema(), (SELECT count(*) FROM pg_tables WHERE schemaname = $1)",
		a.Schema).Scan(&here, &tables); err != nil {
		t.Fatal(err)
	}
	_ = conn.Close(ctx)
	if here != a.Schema || tables == 0 {
		t.Errorf("tenant connection lands in %q with %d tables; want %q, migrated", here, tables, a.Schema)
	}
	if err = tenant.RefuseTenantDatabase(ctx, got.DSN); !errors.Is(err, tenant.ErrTenantDatabase) {
		t.Errorf("RefuseTenantDatabase(tenant DSN) = %v", err)
	}
	if err = tenant.RefuseTenantDatabase(ctx, pv.base); !errors.Is(err, tenant.ErrTenantDatabase) {
		t.Errorf("RefuseTenantDatabase(base DSN) = %v", err)
	}

	if _, err = pv.p.Create(ctx, "org-a", false); !errors.Is(err, tenant.ErrTenantExists) {
		t.Errorf("second Create = %v, want ErrTenantExists", err)
	}

	// Suspend: refused, data kept. Suspend again is a no-op.
	if err = pv.p.Suspend(ctx, "org-a"); err != nil {
		t.Fatal(err)
	}
	if err = pv.p.Suspend(ctx, "org-a"); err != nil {
		t.Errorf("second Suspend: %v", err)
	}
	if _, err = pv.r.Resolve(ctx, "org-a"); !errors.Is(err, tenant.ErrUnknownTenant) {
		t.Errorf("Resolve(suspended) = %v", err)
	}
	if err = pv.p.Resume(ctx, "org-a"); err != nil {
		t.Fatal(err)
	}
	if _, err = pv.r.Resolve(ctx, "org-a"); err != nil {
		t.Errorf("Resolve(resumed) = %v", err)
	}
	if err = pv.p.Resume(ctx, "org-a"); !errors.Is(err, tenant.ErrWrongStatus) {
		t.Errorf("Resume(active) = %v, want ErrWrongStatus", err)
	}

	// Erase while a server holds the tenant: the status goes to erasing,
	// the drop is refused, and nothing can resume it.
	pin, err := tenant.Pin(ctx, got)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err = pv.p.Erase(ctx, "org-a"); !errors.Is(err, tenant.ErrTenantInUse) {
		t.Fatalf("Erase while pinned = %v, want ErrTenantInUse", err)
	}
	if !pv.schemaExists(t, a.Schema) {
		t.Fatal("a refused erase dropped the schema")
	}
	rec, err := pv.r.Lookup(ctx, "org-a")
	if err != nil || rec.Status != tenant.StatusErasing {
		t.Fatalf("Lookup after a refused erase = %+v, %v; want erasing", rec, err)
	}
	if _, err = pv.r.Resolve(ctx, "org-a"); !errors.Is(err, tenant.ErrUnknownTenant) {
		t.Errorf("Resolve(erasing) = %v", err)
	}
	for _, op := range []func(context.Context, string) error{pv.p.Resume, pv.p.Suspend} {
		if err = op(ctx, "org-a"); !errors.Is(err, tenant.ErrWrongStatus) {
			t.Errorf("status change on an erasing tenant = %v, want ErrWrongStatus", err)
		}
	}
	_ = pin.Close()

	auditDir := filepath.Join(pv.auditRoot, a.Schema)
	if err = os.MkdirAll(auditDir, 0o750); err != nil {
		t.Fatal(err)
	}
	if _, res, err := pv.p.Erase(ctx, "org-a"); err != nil || !res.SchemaDropped || !res.AuditRemoved {
		t.Fatalf("Erase = %+v, %v", res, err)
	}
	if pv.schemaExists(t, a.Schema) {
		t.Error("the erased tenant's schema survived")
	}
	if _, err := pv.r.Lookup(ctx, "org-a"); !errors.Is(err, tenant.ErrUnknownTenant) {
		t.Errorf("Lookup(erased) = %v, want ErrUnknownTenant", err)
	}
	if _, err := pv.r.Resolve(ctx, "org-b"); err != nil || !pv.schemaExists(t, b.Schema) {
		t.Errorf("erasing org-a disturbed org-b: %v", err)
	}
	// A recreated org starts empty.
	if _, err := pv.p.Create(ctx, "org-a", false); err != nil {
		t.Errorf("Create after erase: %v", err)
	}
}

// TestProvisioner_CollisionRefused pins that an org whose derived schema is
// recorded for another org is refused by create and by the resolver, never
// handed the other org's schema.
func TestProvisioner_CollisionRefused(t *testing.T) {
	pv := newProvisioned(t)
	ctx := t.Context()
	if _, err := pv.p.Create(ctx, "org-a", false); err != nil {
		t.Fatal(err)
	}
	schemaX, err := tenant.SchemaFor("org-x")
	if err != nil {
		t.Fatal(err)
	}
	// Record org-x's derived schema for another org, as a hash collision
	// would.
	if _, err := pv.conn.Exec(ctx, "INSERT INTO rela_control.tenants (schema_name, org_id, status) "+
		"VALUES ($1, 'someone-else', 'active')", schemaX); err != nil {
		t.Fatal(err)
	}
	if _, err := pv.p.Create(ctx, "org-x", false); !errors.Is(err, tenant.ErrSchemaCollision) {
		t.Errorf("Create(colliding) = %v, want ErrSchemaCollision", err)
	}
	if _, err := pv.r.Resolve(ctx, "org-x"); !errors.Is(err, tenant.ErrSchemaCollision) {
		t.Errorf("Resolve(colliding) = %v, want ErrSchemaCollision", err)
	}
	if _, _, err := pv.p.Erase(ctx, "org-x"); !errors.Is(err, tenant.ErrSchemaCollision) {
		t.Errorf("Erase(colliding) = %v, want ErrSchemaCollision", err)
	}
}

// TestProvisioner_ExistingSchemaNeedsAdopt pins that create does not take
// over a schema that exists without a control row unless asked to.
func TestProvisioner_ExistingSchemaNeedsAdopt(t *testing.T) {
	pv := newProvisioned(t)
	ctx := t.Context()
	schema, err := tenant.SchemaFor("org-old")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := pv.conn.Exec(ctx, "CREATE SCHEMA "+pgx.Identifier{schema}.Sanitize()); err != nil {
		t.Fatal(err)
	}
	if _, err := pv.p.Create(ctx, "org-old", false); !errors.Is(err, tenant.ErrSchemaExists) {
		t.Fatalf("Create over an existing schema = %v, want ErrSchemaExists", err)
	}
	if _, err := pv.r.Lookup(ctx, "org-old"); !errors.Is(err, tenant.ErrUnknownTenant) {
		t.Errorf("a refused create left a control row: %v", err)
	}
	if _, err := pv.p.Create(ctx, "org-old", true); err != nil {
		t.Fatalf("Create -adopt: %v", err)
	}
	if _, err := pv.r.Resolve(ctx, "org-old"); err != nil {
		t.Errorf("Resolve(adopted): %v", err)
	}
}

// TestProvisioner_RefusesBadOrgIDs pins the org id checks at create.
func TestProvisioner_RefusesBadOrgIDs(t *testing.T) {
	pv := newProvisioned(t)
	for _, org := range []string{"", "a\nb", strings.Repeat("x", 300)} {
		if _, err := pv.p.Create(t.Context(), org, false); err == nil {
			t.Errorf("Create(%q) succeeded", org)
		}
	}
}

// TestRefuseTenantDatabase_PlainDatabasePasses pins that a database with no
// control table is not refused.
func TestRefuseTenantDatabase_PlainDatabasePasses(t *testing.T) {
	if err := tenant.RefuseTenantDatabase(t.Context(), backendtest.Database(t)); err != nil {
		t.Errorf("RefuseTenantDatabase(plain) = %v", err)
	}
}

// TestProvisioner_OnlyCreateMakesTheControlTable pins that suspend, resume
// and erase on a database without tenants find no tenant and leave the
// database as it was, so a later server start is not misled by an empty
// control table.
func TestProvisioner_OnlyCreateMakesTheControlTable(t *testing.T) {
	pv := newProvisioned(t)
	ctx := t.Context()

	if err := pv.p.Suspend(ctx, "org-a"); !errors.Is(err, tenant.ErrUnknownTenant) {
		t.Errorf("Suspend = %v, want ErrUnknownTenant", err)
	}
	if err := pv.p.Resume(ctx, "org-a"); !errors.Is(err, tenant.ErrUnknownTenant) {
		t.Errorf("Resume = %v, want ErrUnknownTenant", err)
	}
	if _, _, err := pv.p.Erase(ctx, "org-a"); !errors.Is(err, tenant.ErrUnknownTenant) {
		t.Errorf("Erase = %v, want ErrUnknownTenant", err)
	}
	if pv.schemaExists(t, tenant.ControlSchema) {
		t.Error("a command other than create made the control schema")
	}
}
