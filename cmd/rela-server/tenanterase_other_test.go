//go:build !postgres

package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestEraseTenant_NeedsPostgres pins that a build without PostgreSQL refuses
// to erase and leaves the audit directory alone.
func TestEraseTenant_NeedsPostgres(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, tenantsFile), []byte(eraseTenantsYAML), 0o600); err != nil {
		t.Fatal(err)
	}
	auditDir := filepath.Join(dir, ".rela", "audit", "tenants", "tenant_a")
	if err := os.MkdirAll(auditDir, 0o750); err != nil {
		t.Fatal(err)
	}
	code, stderr := runErase(t, "", "erase", "-project", dir, "-force", "org-a")
	if code != 1 || !strings.Contains(stderr, "postgres build") {
		t.Errorf("exit %d, stderr %q; want a postgres-build refusal", code, stderr)
	}
	if _, err := os.Stat(auditDir); err != nil {
		t.Errorf("audit directory removed after a failed drop: %v", err)
	}
	// The failed attempt is recorded on the host.
	rec, err := os.ReadFile(filepath.Join(dir, ".rela", "audit", erasureLog))
	if err != nil || !strings.Contains(string(rec), `"ok":false`) || !strings.Contains(string(rec), `"org_id":"org-a"`) {
		t.Errorf("erasure record = %q (%v), want a failed attempt for org-a", rec, err)
	}
}
