package metamodel

import (
	"context"
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/migration"
	"github.com/Sourcehaven-BV/rela/internal/storage"
)

// A schema that still declares automations: is refused with the message
// that points at flows.yaml (TKT-GWOBH2U).
func TestFSLoader_RefusesAutomations(t *testing.T) {
	fs := storage.NewMemFS()
	const path = "/proj/schema.yaml"
	schema := "entities:\n  ticket: {id_type: sequential}\nautomations:\n  - name: a\n    on: {created: true}\n"
	if err := fs.MkdirAll("/proj", 0o755); err != nil {
		t.Fatal(err)
	}
	if err := fs.WriteFile(path, []byte(schema), 0o644); err != nil {
		t.Fatal(err)
	}

	_, _, err := NewFSLoader(fs, path).Load(context.Background())
	if !migration.IsMigrationError(err) {
		t.Fatalf("err = %v, want a migration error", err)
	}
	want := path + ": automations: are declared in flows.yaml under triggers: since this release " +
		"(TKT-GWOBH2U); run 'rela migrate' to move them"
	if err.Error() != want {
		t.Errorf("err =\n%s\nwant\n%s", err, want)
	}
	if strings.Contains(err.Error(), "deprecated syntax") {
		t.Errorf("message reads as a syntax migration: %s", err)
	}
}

// An included file that still declares automations: is refused too, and the
// error names the include, not the root schema (TKT-GWOBH2U). Without this
// check the include parser would drop the key and the automations would be
// silently lost.
func TestFSLoader_RefusesAutomationsInInclude(t *testing.T) {
	fs := storage.NewMemFS()
	const root = "/proj/schema.yaml"
	const inc = "/proj/extra.yaml"
	files := map[string]string{
		root: "includes:\n  - extra.yaml\nentities:\n" + validEntity("ticket", "TKT-"),
		inc:  "entities:\n" + validEntity("note", "NOTE-") + "automations:\n  - name: a\n    on: {created: true}\n",
	}
	if err := fs.MkdirAll("/proj", 0o755); err != nil {
		t.Fatal(err)
	}
	for p, body := range files {
		if err := fs.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	_, _, err := NewFSLoader(fs, root).Load(context.Background())
	if !migration.IsMigrationError(err) {
		t.Fatalf("err = %v, want a migration error", err)
	}
	if !strings.HasPrefix(err.Error(), inc+": ") {
		t.Errorf("err = %s, want it to name the include %s", err, inc)
	}
	if !strings.Contains(err.Error(), "flows.yaml under triggers:") {
		t.Errorf("err = %s, want the flows.yaml hint", err)
	}
}

// Parse and Load refuse a schema that still declares automations:, with the
// migration's message, so a caller that skips FSLoader cannot drop them
// silently. LoadWithoutMigrationCheck accepts it: 'rela migrate' reads the
// schema with it before the move. A null or empty list is accepted
// everywhere.
func TestAutomationsKey_ParseAndLoad(t *testing.T) {
	const path = "/proj/schema.yaml"
	base := "entities:\n" + validEntity("ticket", "TKT-")
	tests := []struct {
		name, automations string
		refused           bool
	}{
		{"list", "automations:\n  - name: a\n    on: {created: true}\n", true},
		{"mapping", "automations: {name: a}\n", true},
		{"empty list", "automations: []\n", false},
		{"null", "automations:\n", false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			schema := base + tc.automations
			fs := storage.NewMemFS()
			if err := fs.MkdirAll("/proj", 0o755); err != nil {
				t.Fatal(err)
			}
			if err := fs.WriteFile(path, []byte(schema), 0o644); err != nil {
				t.Fatal(err)
			}

			_, parseErr := Parse([]byte(schema))
			_, _, loadErr := Load(path, fs)
			for _, c := range []struct {
				fn  string
				err error
			}{{"Parse", parseErr}, {"Load", loadErr}} {
				switch {
				case !tc.refused && c.err != nil:
					t.Errorf("%s: %v", c.fn, c.err)
				case tc.refused && !migration.IsMigrationError(c.err):
					t.Errorf("%s: err = %v, want a migration error", c.fn, c.err)
				case tc.refused && !strings.Contains(c.err.Error(), "flows.yaml under triggers:"):
					t.Errorf("%s: err = %v, want the flows.yaml hint", c.fn, c.err)
				}
			}

			m, _, err := LoadWithoutMigrationCheck(path, fs)
			if err != nil {
				t.Fatalf("LoadWithoutMigrationCheck: %v", err)
			}
			if _, ok := m.Entities["ticket"]; !ok {
				t.Error("ticket entity missing")
			}
		})
	}
}

// validEntity is one entity block that passes metamodel validation, so the
// tests reach the automations check instead of failing earlier.
func validEntity(name, prefix string) string {
	return "  " + name + ":\n    label: " + name + "\n    id_type: sequential\n    id_prefix: \"" + prefix + "\"\n" +
		"    properties:\n      title: {type: string}\n"
}
