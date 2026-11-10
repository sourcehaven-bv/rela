package main

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/tenant"
)

// TestRefuseTenantMode pins that rela-docs, which opens one store from
// $RELA_DATABASE_URL, refuses tenant mode and a project that still holds the
// old tenant map, and leaves every other project to the open that follows
// (TKT-M3SHYE0, TKT-HF5KE12).
func TestRefuseTenantMode(t *testing.T) {
	t.Setenv("RELA_DATABASE_URL", "")
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

	t.Setenv(tenant.EnvTenantMode, "1")
	if err := refuseTenantMode(root); !errors.Is(err, errTenantProject) {
		t.Fatalf("refuseTenantMode(tenant mode) = %v, want errTenantProject", err)
	}
	t.Setenv(tenant.EnvTenantMode, "")

	if err := os.WriteFile(filepath.Join(root, "tenants.yaml"), []byte("tenants: []\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	err := refuseTenantMode(root)
	if !errors.Is(err, errTenantProject) || !errors.Is(err, tenant.ErrLegacyTenantMap) {
		t.Fatalf("refuseTenantMode(legacy map) = %v, want errTenantProject and ErrLegacyTenantMap", err)
	}
}
