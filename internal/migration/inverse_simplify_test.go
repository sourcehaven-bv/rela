package migration

import (
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestInverseSimplifyMigration_Detect(t *testing.T) {
	tests := []struct {
		name     string
		yaml     string
		expected bool
	}{
		{
			name: "detects deprecated name field",
			yaml: `
relations:
  addresses:
    inverse:
      name: addressedBy
      label: addressed by
`,
			expected: true,
		},
		{
			name: "detects deprecated name field without label",
			yaml: `
relations:
  addresses:
    inverse:
      name: addressedBy
`,
			expected: true,
		},
		{
			name: "no detection when id field is used",
			yaml: `
relations:
  addresses:
    inverse:
      id: addressedBy
      label: addressed by
`,
			expected: false,
		},
		{
			name: "no detection for simple string form",
			yaml: `
relations:
  addresses:
    inverse: addressedBy
`,
			expected: false,
		},
		{
			name: "no detection when no inverse",
			yaml: `
relations:
  addresses:
    from: [decision]
    to: [requirement]
`,
			expected: false,
		},
		{
			name: "detects name field among multiple relations",
			yaml: `
relations:
  addresses:
    inverse: addressedBy
  implements:
    inverse:
      name: implementedBy
      label: implemented by
`,
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var doc yaml.Node
			if err := yaml.Unmarshal([]byte(tt.yaml), &doc); err != nil {
				t.Fatalf("failed to parse yaml: %v", err)
			}

			migration := &InverseSimplifyMigration{}
			result := migration.Detect(&doc)

			if result != tt.expected {
				t.Errorf("Detect() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestInverseSimplifyMigration_Apply(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			// DEC-6C1NAA: "addressed by" is no longer re-derivable from the id,
			// so it must be preserved (with `name` renamed to `id`).
			name: "keeps a derivable-looking label, renaming name to id",
			input: `relations:
  addresses:
    inverse:
      name: addressedBy
      label: addressed by
`,
			expected: `relations:
    addresses:
        inverse:
            id: addressedBy
            label: addressed by
`,
		},
		{
			name: "converts name without label to string form",
			input: `relations:
  addresses:
    inverse:
      name: addressedBy
`,
			expected: `relations:
    addresses:
        inverse: addressedBy
`,
		},
		{
			name: "renames name to id when custom label",
			input: `relations:
  addresses:
    inverse:
      name: addressedBy
      label: is addressed by
`,
			expected: `relations:
    addresses:
        inverse:
            id: addressedBy
            label: is addressed by
`,
		},
		{
			name: "preserves existing string form",
			input: `relations:
  addresses:
    inverse: addressedBy
`,
			expected: `relations:
    addresses:
        inverse: addressedBy
`,
		},
		{
			name: "preserves id field",
			input: `relations:
  addresses:
    inverse:
      id: addressedBy
      label: addressed by
`,
			expected: `relations:
    addresses:
        inverse:
            id: addressedBy
            label: addressed by
`,
		},
		{
			name: "handles multiple relations mixed forms",
			input: `relations:
  addresses:
    inverse:
      name: addressedBy
      label: addressed by
  implements:
    inverse:
      name: implementedBy
      label: custom implemented
  realizes:
    inverse: realizedBy
`,
			expected: `relations:
    addresses:
        inverse:
            id: addressedBy
            label: addressed by
    implements:
        inverse:
            id: implementedBy
            label: custom implemented
    realizes:
        inverse: realizedBy
`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var doc yaml.Node
			if err := yaml.Unmarshal([]byte(tt.input), &doc); err != nil {
				t.Fatalf("failed to parse input yaml: %v", err)
			}

			migration := &InverseSimplifyMigration{}
			if err := migration.Apply(&doc); err != nil {
				t.Fatalf("Apply() error = %v", err)
			}

			result, err := yaml.Marshal(&doc)
			if err != nil {
				t.Fatalf("failed to marshal result: %v", err)
			}

			if string(result) != tt.expected {
				t.Errorf("Apply() result mismatch\nGot:\n%s\nWant:\n%s", string(result), tt.expected)
			}
		})
	}
}

// TestInverseSimplifyMigration_KeepsDerivableLookingLabels pins DEC-6C1NAA for
// inverse relation labels. These labels used to be dropped because they matched
// camelCaseToSpaced(id); since nothing re-derives them any more, dropping one
// would permanently downgrade the display text to the raw camelCase id.
func TestInverseSimplifyMigration_KeepsDerivableLookingLabels(t *testing.T) {
	labels := []struct {
		id    string
		label string
	}{
		{"addressedBy", "addressed by"},
		{"implementedBy", "implemented by"},
		{"dependencyOf", "dependency of"},
	}

	for _, tt := range labels {
		t.Run(tt.id, func(t *testing.T) {
			yamlStr := `relations:
  addresses:
    inverse:
      name: ` + tt.id + `
      label: ` + tt.label + `
`
			var doc yaml.Node
			if err := yaml.Unmarshal([]byte(yamlStr), &doc); err != nil {
				t.Fatalf("unmarshal: %v", err)
			}

			m := &InverseSimplifyMigration{}
			if err := m.Apply(&doc); err != nil {
				t.Fatalf("Apply: %v", err)
			}
			out, err := yaml.Marshal(&doc)
			if err != nil {
				t.Fatalf("marshal: %v", err)
			}
			if !strings.Contains(string(out), tt.label) {
				t.Errorf("Apply() dropped inverse label %q; labels are authored, never derived\ngot:\n%s",
					tt.label, out)
			}
			// The deprecated "name" key must still be renamed to "id".
			if strings.Contains(string(out), "name:") {
				t.Errorf("Apply() left deprecated \"name\" key\ngot:\n%s", out)
			}
		})
	}
}

// TestInverseSimplifyMigration_CollapsesRedundantLabel confirms the migration
// still collapses to the string form when the label adds nothing over the id.
func TestInverseSimplifyMigration_CollapsesRedundantLabel(t *testing.T) {
	yamlStr := `relations:
  addresses:
    inverse:
      name: addressedBy
      label: addressedBy
`
	var doc yaml.Node
	if err := yaml.Unmarshal([]byte(yamlStr), &doc); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	m := &InverseSimplifyMigration{}
	if err := m.Apply(&doc); err != nil {
		t.Fatalf("Apply: %v", err)
	}
	out, err := yaml.Marshal(&doc)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if strings.Contains(string(out), "label:") {
		t.Errorf("Apply() kept a label identical to the id\ngot:\n%s", out)
	}
}
