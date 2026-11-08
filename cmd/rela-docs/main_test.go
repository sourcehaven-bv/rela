package main

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/tenant"
)

// TestRefuseTenantMode pins that rela-docs, which opens one store from
// $RELA_DATABASE_URL, refuses a project with a tenant map and leaves every
// other project to the open that follows (TKT-M3SHYE0).
func TestRefuseTenantMode(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "schema.yaml"), []byte("entities: {}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := refuseTenantMode(root); err != nil {
		t.Fatalf("a single-store project was refused: %v", err)
	}
	if err := refuseTenantMode(t.TempDir()); err != nil {
		t.Fatalf("a directory in no project was refused: %v", err)
	}

	tenants := "tenants:\n  - {org_id: org-a, schema: tenant_a, dsn: \"host=db.invalid search_path=tenant_a\"}\n"
	if err := os.WriteFile(filepath.Join(root, tenant.ConfigFileName), []byte(tenants), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := refuseTenantMode(root); !errors.Is(err, errTenantProject) {
		t.Fatalf("refuseTenantMode(tenant project) = %v, want errTenantProject", err)
	}
}
