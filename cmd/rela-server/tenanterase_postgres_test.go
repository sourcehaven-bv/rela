//go:build postgres

package main

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"

	"github.com/Sourcehaven-BV/rela/internal/appbuild/backendtest"
)

// TestEraseTenant_EndToEnd runs `rela-server tenant erase` over a tenant map
// whose DSNs are derived from $RELA_DATABASE_URL, as in production: the
// typed confirmation erases the marked tenant and leaves the other one.
func TestEraseTenant_EndToEnd(t *testing.T) {
	base := backendtest.Database(t)
	_, schemaA := backendtest.SchemaIn(t, base)
	dsnB, schemaB := backendtest.SchemaIn(t, base)

	dir := t.TempDir()
	tenantsYAML := "tenants:\n" +
		"  - org_id: org-a\n    schema: " + schemaA + "\n    erase: true\n" +
		"  - org_id: org-b\n    schema: " + schemaB + "\n"
	if err := os.WriteFile(filepath.Join(dir, tenantsFile), []byte(tenantsYAML), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "schema.yaml"), []byte(tenantSchema), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, s := range []string{schemaA, schemaB} {
		d := filepath.Join(dir, ".rela", "audit", "tenants", s)
		if err := os.MkdirAll(d, 0o750); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(d, "audit.jsonl"), []byte("{}\n"), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	var out, errOut bytes.Buffer
	code := eraseTenant(t.Context(), []string{"erase", "-project", dir, "org-a"}, eraseIO{
		in: strings.NewReader("org-a\n"), out: &out, err: &errOut,
		getenv: func(k string) string {
			if k == "RELA_DATABASE_URL" {
				return base
			}
			return ""
		},
	})
	if code != 0 {
		t.Fatalf("exit %d, stderr %q", code, errOut.String())
	}
	if !strings.Contains(out.String(), "dropped") {
		t.Errorf("stdout %q does not report the drop", out.String())
	}
	if strings.Contains(out.String()+errOut.String(), base) {
		t.Error("the output carries the DSN")
	}
	rec, err := os.ReadFile(filepath.Join(dir, ".rela", "audit", erasureLog))
	if err != nil || !strings.Contains(string(rec), `"ok":true`) || !strings.Contains(string(rec), schemaA) {
		t.Errorf("erasure record = %q (%v), want a successful erasure of %s", rec, err, schemaA)
	}

	conn, err := pgx.Connect(t.Context(), dsnB)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer func() { _ = conn.Close(context.Background()) }()
	exists := func(schema string) bool {
		var ok bool
		if err := conn.QueryRow(t.Context(),
			"SELECT EXISTS (SELECT 1 FROM pg_namespace WHERE nspname = $1)", schema).Scan(&ok); err != nil {
			t.Fatalf("look up schema: %v", err)
		}
		return ok
	}
	if exists(schemaA) {
		t.Error("the marked tenant's schema survived")
	}
	if !exists(schemaB) {
		t.Error("the other tenant's schema was dropped")
	}
	auditRoot := filepath.Join(dir, ".rela", "audit", "tenants")
	if _, err := os.Stat(filepath.Join(auditRoot, schemaA)); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("the marked tenant's audit directory survived: %v", err)
	}
	if _, err := os.Stat(filepath.Join(auditRoot, schemaB, "audit.jsonl")); err != nil {
		t.Errorf("the other tenant's audit record is gone: %v", err)
	}
}
