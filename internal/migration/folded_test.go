package migration

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

// TestApply_KeepsFoldedScalarsFolded pins that a migration rewrite does not
// flatten a folded block scalar (`>`) onto one line. yaml.v3 emits every
// scalar without wrapping, so a long folded paragraph came back as one line
// of several hundred characters. The value must stay the same; only the
// layout is restored.
func TestApply_KeepsFoldedScalarsFolded(t *testing.T) {
	content := `version: "1.0"
description: >
  This project tracks controls for the municipality. A person is matched by
  their email address and gets rights through the roles they are linked to.

  A second paragraph stays a second paragraph.
entities:
  component:
    label: Component
    id_type: string
    description: >-
      A short folded value.
    notes: |
      literal lines
      stay as they are
`
	tmpFile := filepath.Join(t.TempDir(), "metamodel.yaml")
	if err := os.WriteFile(tmpFile, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	result, err := Apply(tmpFile, FileTypeMetamodel, testMigrationFS)
	if err != nil {
		t.Fatalf("Apply() error = %v", err)
	}
	if !result.NeedsMigration() {
		t.Fatal("expected the id_type migration to run, so the file is rewritten")
	}

	updated, err := os.ReadFile(tmpFile)
	if err != nil {
		t.Fatal(err)
	}
	out := string(updated)

	for i, line := range strings.Split(out, "\n") {
		if len(line) > foldWidth {
			t.Errorf("line %d is %d characters, want at most %d: %q", i+1, len(line), foldWidth, line)
		}
	}
	if !strings.Contains(out, "description: >\n") {
		t.Errorf("top-level description lost its folded style:\n%s", out)
	}
	if strings.Contains(out, "A second paragraph stays a second paragraph.\n\nentities:") {
		t.Errorf("a clipped folded block gained a trailing blank line:\n%s", out)
	}
	if !strings.Contains(out, "notes: |\n      literal lines\n      stay as they are\n") {
		t.Errorf("literal block changed:\n%s", out)
	}

	var before, after map[string]any
	if err := yaml.Unmarshal([]byte(content), &before); err != nil {
		t.Fatal(err)
	}
	if err := yaml.Unmarshal(updated, &after); err != nil {
		t.Fatalf("rewritten file does not parse: %v\n%s", err, out)
	}
	if before["description"] != after["description"] {
		t.Errorf("description value changed:\nbefore %q\nafter  %q", before["description"], after["description"])
	}
	beforeEnt, _ := before["entities"].(map[string]any)["component"].(map[string]any)
	afterEnt, _ := after["entities"].(map[string]any)["component"].(map[string]any)
	for _, key := range []string{"description", "notes", "label"} {
		if beforeEnt[key] != afterEnt[key] {
			t.Errorf("component %s changed: %q -> %q", key, beforeEnt[key], afterEnt[key])
		}
	}
}

func TestRefoldFolded(t *testing.T) {
	long := strings.Repeat("word ", 30) + "end"
	tests := []struct {
		name      string
		in        string
		unchanged bool
	}{
		{"long paragraph is wrapped", "a: >\n  " + long + "\n", false},
		{"sequence entry is wrapped", "a:\n  - >-\n    " + long + "\n", false},
		{"sequence item key with siblings", "- a: >\n    " + long + "\n  b: >\n    " + long + "\n  c: 1\n", false},
		{"anchored header", "a: &x >\n  " + long + "\n", false},
		{"tagged header", "a: !!str >\n  " + long + "\n", false},
		{"header with comment", "a: > # note\n  " + long + "\n", false},
		{"more-indented line is kept", "a: >\n  short\n    " + long + "\n", true},
		{"indentation indicator is left alone", "a: >2\n     " + long + "\n", true},
		{"keep chomping keeps trailing blanks", "a: >+\n  x\n\n\nb: 1\n", true},
		{"unbreakable word is kept", "a: >\n  " + strings.Repeat("x", 120) + "\n", true},
		{"double spaces are not split", "a: >\n  " + strings.Repeat("ab  ", 40) + "\n", true},
		{"space before a tab is not a break", "a: >\n  " + strings.Repeat("ab \t", 40) + "\n", true},
		{"literal block is not touched", "a: |\n  " + long + "\n", true},
		{"header-like literal content is not touched", "a: |\n  x: >\n    " + long + "\n", true},
		{"header-like comment is not touched", "# usage: >\na: 1\nb: " + long + "\n", true},
		{"block indented past the width", deepFolded(45, long), true},
		{
			// DeepEqual(NaN, NaN) is false, so the check cannot confirm the
			// value and must fall back to the input.
			"unverifiable value falls back to input",
			"n: .nan\na: >\n  " + long + "\n",
			true,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			out := string(refoldFolded([]byte(tc.in)))
			if tc.unchanged && out != tc.in {
				t.Fatalf("output changed:\n%s", out)
			}
			if !tc.unchanged {
				if out == tc.in {
					t.Fatalf("output not wrapped:\n%s", out)
				}
				for line := range strings.SplitSeq(out, "\n") {
					if len(line) > foldWidth {
						t.Errorf("line longer than %d: %q", foldWidth, line)
					}
				}
			}
			if out == tc.in {
				return
			}
			var before, after any
			if err := yaml.Unmarshal([]byte(tc.in), &before); err != nil {
				t.Fatal(err)
			}
			if err := yaml.Unmarshal([]byte(out), &after); err != nil {
				t.Fatalf("output does not parse: %v", err)
			}
			if !reflect.DeepEqual(before, after) {
				t.Errorf("value changed:\nbefore %#v\nafter  %#v", before, after)
			}
		})
	}
}

// deepFolded nests a folded block depth mappings deep, so its content is
// indented 2*depth+2 spaces.
func deepFolded(depth int, text string) string {
	var b strings.Builder
	for i := range depth {
		b.WriteString(strings.Repeat("  ", i) + "k:\n")
	}
	b.WriteString(strings.Repeat("  ", depth) + "a: >\n")
	b.WriteString(strings.Repeat("  ", depth+1) + text + "\n")
	return b.String()
}
