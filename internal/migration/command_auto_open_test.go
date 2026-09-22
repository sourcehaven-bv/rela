package migration

import (
	"strings"
	"testing"
)

const autoOpenYAML = `commands:
  generate-pdf:
    label: "Generate PDF"
    script: |
      echo hi
    context: entity
    auto_open: true
  export-json:
    label: "Export JSON"
    script: |
      cat
    context: entity
  quiet:
    label: "Quiet"
    script: echo hi
    context: global
    auto_open: false
`

// TestCommandAutoOpen_RemovesKey pins that the inert key is stripped from
// every command that carries it, whatever its value — `false` is just as dead
// as `true`.
func TestCommandAutoOpen_RemovesKey(t *testing.T) {
	t.Parallel()

	doc := parseDoc(t, autoOpenYAML)
	m := &CommandAutoOpenMigration{}
	if !m.Detect(doc) {
		t.Fatal("Detect() = false on a document containing auto_open")
	}
	if err := m.Apply(doc); err != nil {
		t.Fatalf("Apply: %v", err)
	}

	got := renderDoc(t, doc)
	if strings.Contains(got, "auto_open") {
		t.Errorf("auto_open survived the migration:\n%s", got)
	}

	// Everything else must be untouched: this migration removes one dead key,
	// it does not rewrite command definitions.
	for _, want := range []string{
		"generate-pdf", "export-json", "quiet",
		"Generate PDF", "Export JSON",
		"context: entity", "context: global",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("migration lost %q:\n%s", want, got)
		}
	}
}

// TestCommandAutoOpen_Idempotent pins that a migrated file is not detected a
// second time — the runner must not report pending work forever.
func TestCommandAutoOpen_Idempotent(t *testing.T) {
	t.Parallel()

	doc := parseDoc(t, autoOpenYAML)
	m := &CommandAutoOpenMigration{}
	if err := m.Apply(doc); err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if m.Detect(doc) {
		t.Error("Detect() = true after Apply; migration is not idempotent")
	}
	if err := m.Apply(doc); err != nil {
		t.Fatalf("second Apply: %v", err)
	}
}

// TestCommandAutoOpen_NoCommands covers the shapes that must be no-ops rather
// than panics: no commands: block at all, and an empty one.
func TestCommandAutoOpen_NoCommands(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name string
		src  string
	}{
		{"no commands block", "forms:\n  ticket:\n    entity_type: ticket\n"},
		{"empty commands block", "commands: {}\n"},
		{"commands with no auto_open", "commands:\n  a:\n    label: A\n    script: echo\n    context: global\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			doc := parseDoc(t, tc.src)
			m := &CommandAutoOpenMigration{}
			if m.Detect(doc) {
				t.Error("Detect() = true on a document with no auto_open")
			}
			if err := m.Apply(doc); err != nil {
				t.Fatalf("Apply: %v", err)
			}
		})
	}
}

// TestCommandAutoOpen_Registered pins that the migration is wired into the
// registry, which is what `rela migrate` walks.
func TestCommandAutoOpen_Registered(t *testing.T) {
	t.Parallel()

	for _, m := range All() {
		if m.Name() == "command-auto-open" {
			return
		}
	}
	t.Error("command-auto-open is not in the migration registry")
}
