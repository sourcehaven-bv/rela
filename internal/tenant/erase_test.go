package tenant_test

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/tenant"
)

const eraseConfig = `
tenants:
  - org_id: org-a
    schema: tenant_a
    dsn: "host=alpha dbname=rela search_path=tenant_a,public"
    erase: true
  - org_id: org-b
    schema: tenant_b
    dsn: "host=alpha dbname=rela search_path=tenant_b,public"
`

// TestConfig_MarkedTenantIsNotServed pins that a tenant marked for erasure
// fails closed as an unknown org does, and that the others are still served.
func TestConfig_MarkedTenantIsNotServed(t *testing.T) {
	t.Parallel()
	cfg, err := tenant.LoadConfig(writeConfig(t, eraseConfig))
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	r, err := cfg.Resolver()
	if err != nil {
		t.Fatalf("Resolver: %v", err)
	}
	if _, err := r.Resolve("org-a"); !errors.Is(err, tenant.ErrUnknownTenant) {
		t.Errorf("Resolve(marked org) error = %v, want ErrUnknownTenant", err)
	}
	if _, err := r.Resolve("org-b"); err != nil {
		t.Errorf("Resolve(org-b): %v", err)
	}
	if r.Len() != 1 {
		t.Errorf("Len() = %d, want 1", r.Len())
	}
}

// TestConfig_MarkedTenantStillReservesItsSchema pins that the mark does not
// take a tenant out of validation: a served org reusing a marked tenant's
// schema would be handed the data about to be erased.
func TestConfig_MarkedTenantStillReservesItsSchema(t *testing.T) {
	t.Parallel()
	cfg, err := tenant.LoadConfig(writeConfig(t, `
tenants:
  - org_id: org-a
    schema: tenant_a
    dsn: "host=alpha search_path=tenant_a,public"
    erase: true
  - org_id: org-b
    schema: tenant_a
    dsn: "host=alpha search_path=tenant_a,public"
`))
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	if _, err := cfg.Resolver(); err == nil {
		t.Error("Resolver accepted a served tenant on a marked tenant's schema")
	}
	if _, err := cfg.ErasureTarget("org-a"); err == nil {
		t.Error("ErasureTarget accepted a map the server would refuse")
	}
}

// TestConfig_AllMarkedIsRefused pins that a map serving no tenant does not
// boot.
func TestConfig_AllMarkedIsRefused(t *testing.T) {
	t.Parallel()
	cfg, err := tenant.LoadConfig(writeConfig(t, `
tenants:
  - org_id: org-a
    schema: tenant_a
    dsn: "host=alpha search_path=tenant_a,public"
    erase: true
`))
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	if _, err := cfg.Resolver(); err == nil {
		t.Error("Resolver accepted a map in which every tenant is marked for erasure")
	}
	if _, err := cfg.ErasureTarget("org-a"); err != nil {
		t.Errorf("ErasureTarget: %v", err)
	}
}

// TestConfig_ErasureTarget pins which orgs may be erased.
func TestConfig_ErasureTarget(t *testing.T) {
	t.Parallel()
	cfg, err := tenant.LoadConfig(writeConfig(t, eraseConfig))
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	got, err := cfg.ErasureTarget("org-a")
	if err != nil {
		t.Fatalf("ErasureTarget(org-a): %v", err)
	}
	if got.Schema != "tenant_a" || got.OrgID != "org-a" {
		t.Errorf("ErasureTarget(org-a) = %+v", got)
	}
	for _, tc := range []struct {
		org  string
		want error
	}{
		{"org-b", tenant.ErrNotMarkedForErasure},
		{"org-c", tenant.ErrUnknownTenant},
		{"", tenant.ErrUnknownTenant},
	} {
		if _, err := cfg.ErasureTarget(tc.org); !errors.Is(err, tc.want) {
			t.Errorf("ErasureTarget(%q) error = %v, want %v", tc.org, err, tc.want)
		}
	}
}

// TestErase_RefusesBadInput pins that Erase checks its input before it
// connects or touches the filesystem.
func TestErase_RefusesBadInput(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	// A directory a traversing schema name would reach.
	victim := filepath.Join(root, "victim")
	if err := os.Mkdir(victim, 0o750); err != nil {
		t.Fatal(err)
	}
	auditRoot := filepath.Join(root, "audit", "tenants")
	for _, tc := range []struct {
		name      string
		t         tenant.Tenant
		auditRoot string
	}{
		{"traversing schema", tenant.Tenant{OrgID: "o", Schema: "../../victim", DSN: "host=x"}, auditRoot},
		{"quoted schema", tenant.Tenant{OrgID: "o", Schema: `a"; DROP SCHEMA public; --`, DSN: "host=x"}, auditRoot},
		{"upper-case schema", tenant.Tenant{OrgID: "o", Schema: "Tenant", DSN: "host=x"}, auditRoot},
		{"no org", tenant.Tenant{Schema: "tenant_a", DSN: "host=x"}, auditRoot},
		{"no dsn", tenant.Tenant{OrgID: "o", Schema: "tenant_a"}, auditRoot},
		{"no audit root", tenant.Tenant{OrgID: "o", Schema: "tenant_a", DSN: "host=x"}, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if _, err := tenant.Erase(t.Context(), tc.t, tc.auditRoot); err == nil {
				t.Fatal("Erase accepted bad input")
			}
			if _, err := os.Stat(victim); err != nil {
				t.Errorf("a refused erase touched %s: %v", victim, err)
			}
		})
	}
}

func TestAuditRoot(t *testing.T) {
	t.Parallel()
	if got, want := tenant.AuditRoot(filepath.Join("p", ".rela")), filepath.Join("p", ".rela", "audit", "tenants"); got != want {
		t.Errorf("AuditRoot = %q, want %q", got, want)
	}
}

// TestErase_RefusesSharedSchemas pins that no tenant may name a schema that
// PostgreSQL or every tenant shares. Erasing `public` would drop pg_trgm for
// every tenant, and serving from it would put a tenant beside the fallback
// every other tenant has on its search_path.
func TestErase_RefusesSharedSchemas(t *testing.T) {
	t.Parallel()
	for _, schema := range []string{"public", "information_schema", "pg_catalog", "pg_temp_1", "pg_x"} {
		t.Run(schema, func(t *testing.T) {
			t.Parallel()
			tn := tenant.Tenant{OrgID: "o", Schema: schema, DSN: "host=x"}
			if _, err := tenant.Erase(t.Context(), tn, t.TempDir()); err == nil {
				t.Error("Erase accepted a shared schema")
			}
			if _, err := tenant.NewMapResolver([]tenant.Tenant{tn}); err == nil {
				t.Error("NewMapResolver accepted a shared schema")
			}
			cfg, err := tenant.LoadConfig(writeConfig(t,
				"base_dsn: \"host=x dbname=rela\"\ntenants:\n  - org_id: o\n    schema: "+schema+"\n"))
			if err != nil {
				t.Fatalf("LoadConfig: %v", err)
			}
			if _, err := cfg.Resolver(); err == nil {
				t.Error("Resolver accepted a shared schema")
			}
		})
	}
}
