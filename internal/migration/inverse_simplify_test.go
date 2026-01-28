package migration

import (
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
			name: "converts name with auto-derived label to string form",
			input: `relations:
  addresses:
    inverse:
      name: addressedBy
      label: addressed by
`,
			expected: `relations:
    addresses:
        inverse: addressedBy
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
        inverse: addressedBy
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

func TestInverseSimplifyMigration_camelCaseToSpaced(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"addressedBy", "addressed by"},
		{"implementedBy", "implemented by"},
		{"realizedBy", "realized by"},
		{"dependencyOf", "dependency of"},
		{"contains", "contains"},
		{"ABC", "a b c"},
		{"", ""},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := camelCaseToSpaced(tt.input)
			if result != tt.expected {
				t.Errorf("camelCaseToSpaced(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}
