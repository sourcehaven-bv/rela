package tenant_test

import (
	"errors"
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

	got, err := r.Resolve(t.Context(), "org-b")
	if err != nil {
		t.Fatalf("Resolve(org-b): %v", err)
	}
	if got.Schema != "tenant_b" {
		t.Errorf("schema = %q, want tenant_b", got.Schema)
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
			got, err := r.Resolve(t.Context(), orgID)
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
