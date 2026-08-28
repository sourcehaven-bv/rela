package tenant_test

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/tenant"
)

func TestMapResolver_ResolvesKnownOrg(t *testing.T) {
	t.Parallel()

	r, err := tenant.NewMapResolver([]tenant.Tenant{
		{OrgID: "org-a", Schema: "tenant_a", DSN: "host=localhost search_path=tenant_a"},
		{OrgID: "org-b", Schema: "tenant_b", DSN: "host=localhost search_path=tenant_b"},
	})
	if err != nil {
		t.Fatalf("NewMapResolver: %v", err)
	}

	got, err := r.Resolve("org-b")
	if err != nil {
		t.Fatalf("Resolve(org-b): %v", err)
	}
	if got.Schema != "tenant_b" {
		t.Errorf("schema = %q, want tenant_b", got.Schema)
	}
	if r.Len() != 2 {
		t.Errorf("Len() = %d, want 2", r.Len())
	}
}

// TestMapResolver_FailsClosed is the core security property of the resolver:
// anything that is not a known tenant yields an error AND a zero Tenant, so a
// caller that ignores the error still cannot reach a database.
func TestMapResolver_FailsClosed(t *testing.T) {
	t.Parallel()

	r, err := tenant.NewMapResolver([]tenant.Tenant{
		{OrgID: "org-a", Schema: "tenant_a", DSN: "host=localhost"},
	})
	if err != nil {
		t.Fatalf("NewMapResolver: %v", err)
	}

	for _, orgID := range []string{"", "org-unknown", "ORG-A", " org-a", "org-a "} {
		t.Run("org="+orgID, func(t *testing.T) {
			t.Parallel()
			got, err := r.Resolve(orgID)
			if !errors.Is(err, tenant.ErrUnknownTenant) {
				t.Fatalf("Resolve(%q) error = %v, want ErrUnknownTenant", orgID, err)
			}
			if got.DSN != "" || got.Schema != "" {
				t.Errorf("Resolve(%q) returned a usable tenant %+v alongside an error", orgID, got)
			}
		})
	}
}

// TestMapResolver_RejectsDuplicateSchema pins the check that exists because the
// failure it prevents is invisible: two orgs on one schema would resolve,
// connect, and read each other's rows with every layer behaving as designed.
func TestMapResolver_RejectsDuplicateSchema(t *testing.T) {
	t.Parallel()

	_, err := tenant.NewMapResolver([]tenant.Tenant{
		{OrgID: "org-a", Schema: "shared", DSN: "host=localhost"},
		{OrgID: "org-b", Schema: "shared", DSN: "host=localhost"},
	})
	if err == nil {
		t.Fatal("two orgs mapped to one schema was accepted; that is a cross-tenant leak")
	}
	if !strings.Contains(err.Error(), "shared") {
		t.Errorf("error %q should name the offending schema", err)
	}
}

func TestMapResolver_RejectsInvalidTenants(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		tenants []tenant.Tenant
	}{
		{"empty org", []tenant.Tenant{{OrgID: "", Schema: "a", DSN: "d"}}},
		{"empty dsn", []tenant.Tenant{{OrgID: "o", Schema: "a", DSN: ""}}},
		{"empty schema", []tenant.Tenant{{OrgID: "o", Schema: "", DSN: "d"}}},
		{"uppercase schema", []tenant.Tenant{{OrgID: "o", Schema: "Tenant", DSN: "d"}}},
		{"leading digit schema", []tenant.Tenant{{OrgID: "o", Schema: "1tenant", DSN: "d"}}},
		{"hyphen schema", []tenant.Tenant{{OrgID: "o", Schema: "ten-ant", DSN: "d"}}},
		{"quote in schema", []tenant.Tenant{{OrgID: "o", Schema: `a";DROP`, DSN: "d"}}},
		{"too long schema", []tenant.Tenant{{OrgID: "o", Schema: strings.Repeat("a", 32), DSN: "d"}}},
		{"duplicate org", []tenant.Tenant{
			{OrgID: "o", Schema: "a", DSN: "d"},
			{OrgID: "o", Schema: "b", DSN: "d"},
		}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if _, err := tenant.NewMapResolver(tc.tenants); err == nil {
				t.Fatalf("NewMapResolver accepted %s", tc.name)
			}
		})
	}
}

// TestMapResolver_AcceptsMaximumLengthSchema pins the boundary of the length
// cap, which exists so a name can never be truncated by PostgreSQL into a
// collision with another tenant's.
func TestMapResolver_AcceptsMaximumLengthSchema(t *testing.T) {
	t.Parallel()

	name := "a" + strings.Repeat("b", 30) // 31 chars: the documented maximum
	if _, err := tenant.NewMapResolver([]tenant.Tenant{
		{OrgID: "o", Schema: name, DSN: "d"},
	}); err != nil {
		t.Fatalf("31-character schema rejected: %v", err)
	}
}

func TestLoadConfig_ExplicitDSNs(t *testing.T) {
	t.Parallel()

	path := writeConfig(t, `
tenants:
  - org_id: org-a
    schema: tenant_a
    dsn: "host=alpha dbname=rela search_path=tenant_a,public"
  - org_id: org-b
    schema: tenant_b
    dsn: "host=beta dbname=rela search_path=tenant_b,public"
`)
	cfg, err := tenant.LoadConfig(path)
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	r, err := cfg.Resolver()
	if err != nil {
		t.Fatalf("Resolver: %v", err)
	}
	got, err := r.Resolve("org-b")
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	// An explicit per-tenant DSN is the sharding tier: it must survive
	// untouched, including the host, or promoting a tenant to its own cluster
	// would silently return it to the shared one.
	if !strings.Contains(got.DSN, "host=beta") {
		t.Errorf("DSN = %q, want the explicit host=beta preserved", got.DSN)
	}
}

func TestLoadConfig_RejectsEmptyAndMalformed(t *testing.T) {
	t.Parallel()

	t.Run("no tenants", func(t *testing.T) {
		t.Parallel()
		cfg, err := tenant.LoadConfig(writeConfig(t, "tenants: []\n"))
		if err != nil {
			t.Fatalf("LoadConfig: %v", err)
		}
		if _, err := cfg.Resolver(); err == nil {
			t.Fatal("a tenant map with no tenants was accepted")
		}
	})

	t.Run("no dsn and no base_dsn", func(t *testing.T) {
		t.Parallel()
		cfg, err := tenant.LoadConfig(writeConfig(t, "tenants:\n  - org_id: a\n    schema: tenant_a\n"))
		if err != nil {
			t.Fatalf("LoadConfig: %v", err)
		}
		if _, err := cfg.Resolver(); err == nil {
			t.Fatal("a tenant with no derivable DSN was accepted")
		}
	})

	t.Run("malformed yaml", func(t *testing.T) {
		t.Parallel()
		if _, err := tenant.LoadConfig(writeConfig(t, "tenants: [unclosed\n")); err == nil {
			t.Fatal("malformed YAML was accepted; it must not degrade to an empty map")
		}
	})

	t.Run("missing file", func(t *testing.T) {
		t.Parallel()
		if _, err := tenant.LoadConfig(filepath.Join(t.TempDir(), "absent.yaml")); err == nil {
			t.Fatal("a missing tenant map was accepted")
		}
	})
}

// TestLoadConfig_RejectsInvalidSchemaBeforeDerivation pins that a bad schema
// name is refused before it is interpolated into a connection string, rather
// than after.
func TestLoadConfig_RejectsInvalidSchemaBeforeDerivation(t *testing.T) {
	t.Parallel()

	cfg, err := tenant.LoadConfig(writeConfig(t, `
base_dsn: "host=localhost dbname=rela user=rela"
tenants:
  - org_id: org-a
    schema: "public,tenant_b"
`))
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	if _, err := cfg.Resolver(); err == nil {
		t.Fatal("a schema name containing a search_path separator was accepted")
	}
}

func writeConfig(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), tenant.ConfigFileName)
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	return path
}
