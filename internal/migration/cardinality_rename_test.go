package migration

import (
	"testing"

	"gopkg.in/yaml.v3"
)

func TestCardinalityRenameMigration_Detect(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		detect bool
	}{
		{
			name: "detects source_min",
			input: `
relations:
  addresses:
    from: [decision]
    to: [requirement]
    source_min: 1
`,
			detect: true,
		},
		{
			name: "detects source_max",
			input: `
relations:
  addresses:
    from: [decision]
    to: [requirement]
    source_max: 5
`,
			detect: true,
		},
		{
			name: "detects target_min",
			input: `
relations:
  addresses:
    from: [decision]
    to: [requirement]
    target_min: 1
`,
			detect: true,
		},
		{
			name: "detects target_max",
			input: `
relations:
  addresses:
    from: [decision]
    to: [requirement]
    target_max: 3
`,
			detect: true,
		},
		{
			name: "no detection for new keys",
			input: `
relations:
  addresses:
    from: [decision]
    to: [requirement]
    min_outgoing: 1
    max_incoming: 3
`,
			detect: false,
		},
		{
			name: "no detection without cardinality",
			input: `
relations:
  addresses:
    from: [decision]
    to: [requirement]
`,
			detect: false,
		},
		{
			name: "no detection without relations",
			input: `
entities:
  requirement:
    label: Requirement
`,
			detect: false,
		},
		{
			name: "detects mixed old and new keys",
			input: `
relations:
  addresses:
    from: [decision]
    to: [requirement]
    min_outgoing: 1
    target_max: 3
`,
			detect: true,
		},
	}

	m := &CardinalityRenameMigration{}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var doc yaml.Node
			if err := yaml.Unmarshal([]byte(tt.input), &doc); err != nil {
				t.Fatalf("failed to parse YAML: %v", err)
			}

			got := m.Detect(&doc)
			if got != tt.detect {
				t.Errorf("Detect() = %v, want %v", got, tt.detect)
			}
		})
	}
}

func TestCardinalityRenameMigration_Apply(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		checkKey map[string]bool // key -> should exist after apply
	}{
		{
			name: "renames all four keys",
			input: `
relations:
  addresses:
    from: [decision]
    to: [requirement]
    source_min: 1
    source_max: 5
    target_min: 1
    target_max: 3
`,
			checkKey: map[string]bool{
				"min_outgoing": true,
				"max_outgoing": true,
				"min_incoming": true,
				"max_incoming": true,
				"source_min":   false,
				"source_max":   false,
				"target_min":   false,
				"target_max":   false,
			},
		},
		{
			name: "renames only old keys, preserves new keys",
			input: `
relations:
  addresses:
    from: [decision]
    to: [requirement]
    min_outgoing: 1
    target_max: 3
`,
			checkKey: map[string]bool{
				"min_outgoing": true,
				"max_incoming": true,
				"target_max":   false,
			},
		},
		{
			name: "renames across multiple relations",
			input: `
relations:
  addresses:
    from: [decision]
    to: [requirement]
    source_min: 1
  implements:
    from: [solution]
    to: [decision]
    target_max: 1
`,
			checkKey: map[string]bool{},
		},
	}

	m := &CardinalityRenameMigration{}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var doc yaml.Node
			if err := yaml.Unmarshal([]byte(tt.input), &doc); err != nil {
				t.Fatalf("failed to parse YAML: %v", err)
			}

			if err := m.Apply(&doc); err != nil {
				t.Fatalf("Apply() error: %v", err)
			}

			// Verify the migration should no longer be detected
			if m.Detect(&doc) {
				t.Error("Detect() still returns true after Apply()")
			}

			// Check specific keys if provided
			if len(tt.checkKey) > 0 {
				root := GetDocumentRoot(&doc)
				relations := GetMapValue(root, "relations")
				// Check first relation definition
				if relations == nil || len(relations.Content) < 2 {
					t.Fatal("expected relations section with at least one definition")
				}
				relDef := relations.Content[1]
				for key, shouldExist := range tt.checkKey {
					exists := GetMapValue(relDef, key) != nil
					if shouldExist && !exists {
						t.Errorf("expected key %q to exist after Apply()", key)
					} else if !shouldExist && exists {
						t.Errorf("expected key %q to be removed after Apply()", key)
					}
				}
			}
		})
	}
}

func TestCardinalityRenameMigration_Apply_PreservesValues(t *testing.T) {
	input := `
relations:
  addresses:
    from: [decision]
    to: [requirement]
    source_min: 1
    source_max: 5
    target_min: 2
    target_max: 10
`
	var doc yaml.Node
	if err := yaml.Unmarshal([]byte(input), &doc); err != nil {
		t.Fatalf("failed to parse YAML: %v", err)
	}

	m := &CardinalityRenameMigration{}
	if err := m.Apply(&doc); err != nil {
		t.Fatalf("Apply() error: %v", err)
	}

	root := GetDocumentRoot(&doc)
	relations := GetMapValue(root, "relations")
	relDef := relations.Content[1]

	checks := map[string]string{
		"min_outgoing": "1",
		"max_outgoing": "5",
		"min_incoming": "2",
		"max_incoming": "10",
	}

	for key, expectedVal := range checks {
		val := GetMapValue(relDef, key)
		if val == nil {
			t.Errorf("expected key %q after Apply()", key)
			continue
		}
		if val.Value != expectedVal {
			t.Errorf("key %q: got value %q, want %q", key, val.Value, expectedVal)
		}
	}
}

func TestCardinalityRenameMigration_Apply_EmptyDocument(t *testing.T) {
	m := &CardinalityRenameMigration{}
	err := m.Apply(nil)
	if err == nil {
		t.Error("expected error for nil document")
	}
}

func TestCardinalityRenameMigration_Name(t *testing.T) {
	m := &CardinalityRenameMigration{}
	if m.Name() != "cardinality-rename" {
		t.Errorf("Name() = %q, want %q", m.Name(), "cardinality-rename")
	}
}

func TestCardinalityRenameMigration_Description(t *testing.T) {
	m := &CardinalityRenameMigration{}
	desc := m.Description()
	if desc == "" {
		t.Error("Description() should not be empty")
	}
}

func TestCardinalityRenameMigration_FileTypes(t *testing.T) {
	m := &CardinalityRenameMigration{}
	ft := m.FileTypes()
	if len(ft) != 1 || ft[0] != FileTypeMetamodel {
		t.Errorf("FileTypes() = %v, want [FileTypeMetamodel]", ft)
	}
}
