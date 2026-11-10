package tenant_test

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/tenant"
)

// entryResolver serves org-a and org-b. org-gone is not served, as a
// suspended or erasing org is not.
func entryResolver(t *testing.T) tenant.Resolver {
	t.Helper()
	r, err := tenant.NewMapResolver([]tenant.Tenant{
		{OrgID: "org-a", Schema: "tenant_a", DSN: "host=db.internal dbname=rela search_path=tenant_a"},
		{OrgID: "org-b", Schema: "tenant_b", DSN: "host=db.internal dbname=rela search_path=tenant_b"},
	})
	if err != nil {
		t.Fatalf("NewMapResolver: %v", err)
	}
	return r
}

// TestSelect_FailsClosed pins the operator-entry lookup: a missing org and
// an org the resolver does not serve both fail, and neither returns a
// usable tenant.
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
		{"not served", "org-gone", tenant.ErrUnknownTenant},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, selErr := tenant.Select(t.Context(), r, tc.org)
			if !errors.Is(selErr, tc.want) {
				t.Fatalf("Select(%q) error = %v, want %v", tc.org, selErr, tc.want)
			}
			if got != (tenant.Tenant{}) {
				t.Errorf("Select(%q) returned a usable tenant %+v alongside an error", tc.org, got)
			}
		})
	}

	got, err := tenant.Select(t.Context(), r, "org-b")
	if err != nil {
		t.Fatalf("Select(org-b): %v", err)
	}
	if got.Schema != "tenant_b" {
		t.Errorf("Select(org-b).Schema = %q, want tenant_b", got.Schema)
	}
}

// TestSelect_ErrorsNameNoDSN pins that a failed selection never echoes a
// DSN, which carries a password, into an error an operator's shell prints.
func TestSelect_ErrorsNameNoDSN(t *testing.T) {
	t.Parallel()

	r := entryResolver(t)
	for _, org := range []string{"", "org-c", "org-gone"} {
		_, err := tenant.Select(t.Context(), r, org)
		if err == nil {
			t.Fatalf("Select(%q) succeeded", org)
		}
		if strings.Contains(err.Error(), "db.internal") || strings.Contains(err.Error(), "search_path") {
			t.Errorf("Select(%q) error %q names a DSN", org, err)
		}
	}
}

func TestSelect_RejectsNilResolver(t *testing.T) {
	t.Parallel()

	if _, err := tenant.Select(t.Context(), nil, "org-a"); err == nil {
		t.Fatal("Select(nil resolver) succeeded")
	}
}

func TestOpenOne_RejectsNilBase(t *testing.T) {
	t.Parallel()

	if _, err := tenant.OpenOne(t.Context(), nil, tenant.Tenant{}); err == nil {
		t.Fatal("OpenOne(nil base) succeeded")
	}
}

// TestForDeployment pins tenant-mode detection for the operator entry
// points: off without the switch, whatever files the project holds;
// ErrUnsupportedBuild on a build without tenant mode; and a resolver, or an
// error without a base DSN, on the postgres build.
func TestForDeployment(t *testing.T) {
	t.Parallel()

	r, err := tenant.ForDeployment(t.Context(), func(string) string { return "" })
	if err != nil || r != nil {
		t.Fatalf("ForDeployment(off) = %v, %v; want nil, nil", r, err)
	}
	on := func(k string) string {
		if k == tenant.EnvTenantMode {
			return "1"
		}
		return ""
	}
	r, err = tenant.ForDeployment(t.Context(), on)
	if !tenant.Supported {
		if !errors.Is(err, tenant.ErrUnsupportedBuild) || r != nil {
			t.Fatalf("ForDeployment on a build without tenant mode = %v, %v; want nil, ErrUnsupportedBuild", r, err)
		}
		return
	}
	if err == nil || r != nil {
		t.Fatalf("ForDeployment(on, no DSN) = %v, %v; want nil and an error", r, err)
	}
}

// TestModeEnabled pins that only "1" switches tenant mode on.
func TestModeEnabled(t *testing.T) {
	t.Parallel()
	for v, want := range map[string]bool{"1": true, "": false, "0": false, "true": false, "yes": false} {
		if got := tenant.ModeEnabled(func(string) string { return v }); got != want {
			t.Errorf("ModeEnabled(%q) = %v, want %v", v, got, want)
		}
	}
}

// TestRefuseLegacyMap pins that a project holding the old tenant map is
// refused, whatever the file holds, and a symlink counts as present.
func TestRefuseLegacyMap(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	if err := tenant.RefuseLegacyMap(dir); err != nil {
		t.Fatalf("RefuseLegacyMap(no file) = %v", err)
	}
	if err := tenant.RefuseLegacyMap(""); err != nil {
		t.Fatalf("RefuseLegacyMap(\"\") = %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "tenants.yaml"), nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := tenant.RefuseLegacyMap(dir); !errors.Is(err, tenant.ErrLegacyTenantMap) {
		t.Fatalf("RefuseLegacyMap(empty file) = %v, want ErrLegacyTenantMap", err)
	}
	link := t.TempDir()
	if err := os.Symlink(filepath.Join(link, "nowhere"), filepath.Join(link, "tenants.yaml")); err != nil {
		t.Fatal(err)
	}
	if err := tenant.RefuseLegacyMap(link); !errors.Is(err, tenant.ErrLegacyTenantMap) {
		t.Fatalf("RefuseLegacyMap(dangling symlink) = %v, want ErrLegacyTenantMap", err)
	}
}

// TestNothingReadsTheTenantMap pins TKT-HF5KE12: no production code reads
// tenants.yaml. Only entry.go names it, to refuse a project that holds it.
func TestNothingReadsTheTenantMap(t *testing.T) {
	t.Parallel()

	root := filepath.Join("..", "..")
	var offenders []string
	for _, top := range []string{"cmd", "internal"} {
		err := filepath.WalkDir(filepath.Join(root, top), func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
				return nil
			}
			body, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			if strings.Contains(string(body), "tenants.yaml") &&
				filepath.ToSlash(path) != "../../internal/tenant/entry.go" {

				offenders = append(offenders, path)
			}
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	if len(offenders) > 0 {
		t.Errorf("these files still name tenants.yaml: %v", offenders)
	}
}
