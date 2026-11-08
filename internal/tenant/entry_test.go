package tenant_test

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/tenant"
)

// entryMap has two served tenants and one marked for erasure. Each carries an
// explicit DSN, so the map loads on every build.
const entryMap = `tenants:
  - org_id: org-a
    schema: tenant_a
    dsn: "host=db.internal dbname=rela search_path=tenant_a"
  - org_id: org-b
    schema: tenant_b
    dsn: "host=db.internal dbname=rela search_path=tenant_b"
  - org_id: org-gone
    schema: tenant_gone
    dsn: "host=db.internal dbname=rela search_path=tenant_gone"
    erase: true
`

func writeTenantMap(t *testing.T, body string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, tenant.ConfigFileName)
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestConfigPath(t *testing.T) {
	t.Parallel()

	without := t.TempDir()
	got, err := tenant.ConfigPath(without)
	if err != nil || got != "" {
		t.Fatalf("ConfigPath(no map) = %q, %v; want \"\", nil", got, err)
	}

	path := writeTenantMap(t, entryMap)
	got, err = tenant.ConfigPath(filepath.Dir(path))
	if err != nil {
		t.Fatalf("ConfigPath: %v", err)
	}
	if got != path {
		t.Errorf("ConfigPath = %q, want %q", got, path)
	}
}

// TestSelect_FailsClosed pins the operator-entry lookup: a missing org, an
// unknown org and an org marked for erasure all fail, and none of them
// returns a usable tenant.
func TestSelect_FailsClosed(t *testing.T) {
	t.Parallel()

	r := entryResolver(t)

	for _, tc := range []struct {
		name, org string
		want      error
	}{
		{"missing", "", tenant.ErrTenantRequired},
		{"unknown", "org-c", tenant.ErrUnknownTenant},
		{"case differs", "ORG-A", tenant.ErrUnknownTenant},
		{"marked for erasure", "org-gone", tenant.ErrUnknownTenant},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, selErr := tenant.Select(r, tc.org)
			if !errors.Is(selErr, tc.want) {
				t.Fatalf("Select(%q) error = %v, want %v", tc.org, selErr, tc.want)
			}
			if got != (tenant.Tenant{}) {
				t.Errorf("Select(%q) returned a usable tenant %+v alongside an error", tc.org, got)
			}
		})
	}

	got, err := tenant.Select(r, "org-b")
	if err != nil {
		t.Fatalf("Select(org-b): %v", err)
	}
	if got.Schema != "tenant_b" {
		t.Errorf("Select(org-b).Schema = %q, want tenant_b", got.Schema)
	}
}

// TestLoadDeployment_BaseDSNFromEnv pins the rule the server and the CLI
// share: the base DSN comes from $RELA_DATABASE_URL when the map gives none,
// and a map that gives one keeps it.
func TestLoadDeployment_BaseDSNFromEnv(t *testing.T) {
	t.Parallel()

	env := func(k string) string {
		if k == "RELA_DATABASE_URL" {
			return "host=from-env dbname=rela"
		}
		return ""
	}
	cfg, err := tenant.LoadDeployment(writeTenantMap(t, "tenants:\n  - {org_id: a, schema: tenant_a}\n"), env)
	if err != nil {
		t.Fatalf("LoadDeployment: %v", err)
	}
	if cfg.BaseDSN != "host=from-env dbname=rela" {
		t.Errorf("BaseDSN = %q, want the environment's", cfg.BaseDSN)
	}

	cfg, err = tenant.LoadDeployment(writeTenantMap(t,
		"base_dsn: host=from-file\ntenants:\n  - {org_id: a, schema: tenant_a}\n"), env)
	if err != nil {
		t.Fatalf("LoadDeployment: %v", err)
	}
	if cfg.BaseDSN != "host=from-file" {
		t.Errorf("BaseDSN = %q, want the file's", cfg.BaseDSN)
	}
}

// TestSelect_ErrorsNameNoDSN pins that a failed selection never echoes a
// DSN, which carries a password, into an error an operator's shell prints.
func TestSelect_ErrorsNameNoDSN(t *testing.T) {
	t.Parallel()

	r := entryResolver(t)
	for _, org := range []string{"", "org-c", "org-gone"} {
		_, err := tenant.Select(r, org)
		if err == nil {
			t.Fatalf("Select(%q) succeeded", org)
		}
		if strings.Contains(err.Error(), "db.internal") || strings.Contains(err.Error(), "search_path") {
			t.Errorf("Select(%q) error %q names a DSN", org, err)
		}
	}
}

func TestOpenOne_RejectsNilBase(t *testing.T) {
	t.Parallel()

	if _, err := tenant.OpenOne(t.Context(), nil, tenant.Tenant{}); err == nil {
		t.Fatal("OpenOne(nil base) succeeded")
	}
}

// entryResolver is the resolver of entryMap.
func entryResolver(t *testing.T) tenant.Resolver {
	t.Helper()
	cfg, err := tenant.LoadDeployment(writeTenantMap(t, entryMap), func(string) string { return "" })
	if err != nil {
		t.Fatalf("LoadDeployment: %v", err)
	}
	r, err := cfg.Resolver()
	if err != nil {
		t.Fatalf("Resolver: %v", err)
	}
	return r
}

func TestSelect_RejectsNilResolver(t *testing.T) {
	t.Parallel()

	if _, err := tenant.Select(nil, "org-a"); err == nil {
		t.Fatal("Select(nil resolver) succeeded")
	}
}

// TestForProject pins tenant-mode detection for the operator entry points: a
// project without tenants is single-store (nil resolver), a build without
// tenant mode refuses a tenant project before reading its tenants, and a
// tenant-mode build returns the resolver they route by.
func TestForProject(t *testing.T) {
	t.Parallel()
	noEnv := func(string) string { return "" }

	r, err := tenant.ForProject(t.TempDir(), noEnv)
	if err != nil || r != nil {
		t.Fatalf("ForProject(single-store) = %v, %v; want nil, nil", r, err)
	}

	root := filepath.Dir(writeTenantMap(t, entryMap))
	r, err = tenant.ForProject(root, noEnv)
	if !tenant.Supported {
		if !errors.Is(err, tenant.ErrUnsupportedBuild) || r != nil {
			t.Fatalf("ForProject on a build without tenant mode = %v, %v; want nil, ErrUnsupportedBuild", r, err)
		}
		return
	}
	if err != nil || r == nil {
		t.Fatalf("ForProject(tenant project) = %v, %v; want a resolver", r, err)
	}
	got, err := tenant.Select(r, "org-a")
	if err != nil || got.Schema != "tenant_a" {
		t.Errorf("Select(org-a) = %+v, %v; want tenant_a", got, err)
	}

	// A map that fails to load is an error, never a nil (single-store) resolver.
	bad := filepath.Dir(writeTenantMap(t, "tenants: [\n"))
	if r, err := tenant.ForProject(bad, noEnv); err == nil || r != nil {
		t.Errorf("ForProject(broken map) = %v, %v; want nil and an error", r, err)
	}
}
