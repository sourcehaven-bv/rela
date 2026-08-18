package cli

import (
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/dataentryconfig"
)

// AM-acl-audit-permission-consumers-complete, part 1.
//
// Each data-entry surface that carries a `permission:` is tested on its own,
// so dropping one from dataEntryPermissions.UsedPermissions fails a test
// rather than silently making the audit report a working grant as dead
// config. A single combined fixture would pass with three of four surfaces
// missing — the same incompleteness that caused BUG-919PM6.
func TestDataEntryPermissions_PerSurface(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		yaml string
	}{
		{
			name: "standalone document",
			yaml: `
documents:
  sales-report:
    title: Sales report
    permission: report:sales
`,
		},
		{
			name: "dashboard card",
			yaml: `
dashboard:
  cards:
    - title: Sales
      permission: report:sales
`,
		},
		{
			name: "navigation entry",
			yaml: `
navigation:
  - label: Sales
    list: orders
    permission: report:sales
`,
		},
		{
			name: "nested navigation entry",
			yaml: `
navigation:
  - group: Reports
    items:
      - label: Sales
        list: orders
        permission: report:sales
`,
		},
		{
			name: "command",
			yaml: `
commands:
  export-sales:
    label: Export
    permission: report:sales
`,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			root := t.TempDir()
			writeDataEntry(t, root, tc.yaml)

			perms, err := loadDataEntryPermissions(root)
			if err != nil {
				t.Fatalf("loadDataEntryPermissions: %v", err)
			}
			if got := perms.UsedPermissions(); !slices.Contains(got, "report:sales") {
				t.Errorf("permission on a %s not collected; got %q", tc.name, got)
			}
		})
	}
}

// A missing data-entry.yaml is COMPLETE information — the project has no UI
// gates — so it yields a usable consumer, not nil. Returning nil here would
// suppress A7 for every project that has no data-entry.yaml.
func TestDataEntryPermissions_MissingFileIsNotNil(t *testing.T) {
	t.Parallel()
	perms, err := loadDataEntryPermissions(t.TempDir())
	if err != nil {
		t.Fatalf("missing data-entry.yaml must not error: %v", err)
	}
	if perms == nil {
		t.Fatal("missing data-entry.yaml must yield a consumer, not nil (nil suppresses A7)")
	}
	if got := perms.UsedPermissions(); len(got) != 0 {
		t.Errorf("expected no permissions, got %q", got)
	}
}

// Malformed YAML is INCOMPLETE information: the caller must see an error so it
// can suppress A7 rather than treat "we could not parse it" as "no UI gates".
func TestDataEntryPermissions_MalformedYAMLErrors(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeDataEntry(t, root, "navigation: [unclosed\n")

	if _, err := loadDataEntryPermissions(root); err == nil {
		t.Error("malformed data-entry.yaml must error so the caller can suppress A7")
	}
}

// The adapter must not validate: the audit's subject is acl.yaml, and a
// project whose data-entry.yaml is invalid should still get its ACL findings.
func TestDataEntryPermissions_DoesNotValidateConfig(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	writeDataEntry(t, root, `
navigation:
  - label: Sales
    list: no-such-list-in-any-metamodel
    permission: report:sales
`)

	perms, err := loadDataEntryPermissions(root)
	if err != nil {
		t.Fatalf("adapter must not validate against the metamodel: %v", err)
	}
	if got := perms.UsedPermissions(); !slices.Contains(got, "report:sales") {
		t.Errorf("expected report:sales despite an invalid list ref, got %q", got)
	}
}

func writeDataEntry(t *testing.T, root, body string) {
	t.Helper()
	path := filepath.Join(root, dataentryconfig.ConfigFile)
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}
