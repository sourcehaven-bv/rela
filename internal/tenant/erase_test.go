package tenant_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/tenant"
)

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
	for _, schema := range []string{"public", "information_schema", "pg_catalog", "pg_temp_1", "pg_x", tenant.ControlSchema} {
		t.Run(schema, func(t *testing.T) {
			t.Parallel()
			tn := tenant.Tenant{OrgID: "o", Schema: schema, DSN: "host=x"}
			if _, err := tenant.Erase(t.Context(), tn, t.TempDir()); err == nil {
				t.Error("Erase accepted a shared schema")
			}
			if _, err := tenant.NewMapResolver([]tenant.Tenant{tn}); err == nil {
				t.Error("NewMapResolver accepted a shared schema")
			}
		})
	}
}
