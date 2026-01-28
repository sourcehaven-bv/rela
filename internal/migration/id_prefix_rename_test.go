package migration

import (
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestIDPrefixRenameMigration_Detect(t *testing.T) {
	tests := []struct {
		name     string
		yaml     string
		wantFind bool
	}{
		{
			name: "detects id_patterns",
			yaml: `
version: "1.0"
entities:
  requirement:
    label: Requirement
    id_patterns: ["REQ-"]
`,
			wantFind: true,
		},
		{
			name: "detects multiple patterns",
			yaml: `
version: "1.0"
entities:
  component:
    label: Component
    id_patterns: ["COMP-", "CMP-"]
`,
			wantFind: true,
		},
		{
			name: "detects in multiple entities",
			yaml: `
version: "1.0"
entities:
  requirement:
    label: Requirement
    id_patterns: ["REQ-"]
  component:
    label: Component
    id_patterns: ["COMP-"]
`,
			wantFind: true,
		},
		{
			name: "no detection for id_prefix",
			yaml: `
version: "1.0"
entities:
  requirement:
    label: Requirement
    id_prefix: "REQ-"
`,
			wantFind: false,
		},
		{
			name: "no detection for id_prefixes",
			yaml: `
version: "1.0"
entities:
  component:
    label: Component
    id_prefixes: ["COMP-", "CMP-"]
`,
			wantFind: false,
		},
		{
			name: "no detection when field is missing",
			yaml: `
version: "1.0"
entities:
  requirement:
    label: Requirement
`,
			wantFind: false,
		},
		{
			name: "no detection for empty entities",
			yaml: `
version: "1.0"
entities: {}
`,
			wantFind: false,
		},
		{
			name: "no detection when entities is missing",
			yaml: `
version: "1.0"
`,
			wantFind: false,
		},
	}

	m := &IDPrefixRenameMigration{}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var doc yaml.Node
			if err := yaml.Unmarshal([]byte(tt.yaml), &doc); err != nil {
				t.Fatalf("failed to parse YAML: %v", err)
			}

			got := m.Detect(&doc)
			if got != tt.wantFind {
				t.Errorf("Detect() = %v, want %v", got, tt.wantFind)
			}
		})
	}
}

func TestIDPrefixRenameMigration_Apply(t *testing.T) {
	tests := []struct {
		name       string
		yaml       string
		wantValues map[string]interface{} // entity name -> expected field (id_prefix string or id_prefixes []string)
	}{
		{
			name: "converts single pattern to id_prefix scalar",
			yaml: `
version: "1.0"
entities:
  requirement:
    label: Requirement
    id_patterns: ["REQ-"]
`,
			wantValues: map[string]interface{}{"requirement": "REQ-"},
		},
		{
			name: "converts multiple patterns to id_prefixes",
			yaml: `
version: "1.0"
entities:
  component:
    label: Component
    id_patterns: ["COMP-", "CMP-"]
`,
			wantValues: map[string]interface{}{"component": []string{"COMP-", "CMP-"}},
		},
		{
			name: "handles multiple entities with different patterns",
			yaml: `
version: "1.0"
entities:
  requirement:
    label: Requirement
    id_patterns: ["REQ-"]
  component:
    label: Component
    id_patterns: ["COMP-", "CMP-"]
`,
			wantValues: map[string]interface{}{
				"requirement": "REQ-",
				"component":   []string{"COMP-", "CMP-"},
			},
		},
		{
			name: "leaves id_prefix unchanged",
			yaml: `
version: "1.0"
entities:
  requirement:
    label: Requirement
    id_prefix: "REQ-"
`,
			wantValues: map[string]interface{}{"requirement": "REQ-"},
		},
		{
			name: "leaves id_prefixes unchanged",
			yaml: `
version: "1.0"
entities:
  component:
    label: Component
    id_prefixes: ["COMP-", "CMP-"]
`,
			wantValues: map[string]interface{}{"component": []string{"COMP-", "CMP-"}},
		},
	}

	m := &IDPrefixRenameMigration{}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var doc yaml.Node
			if err := yaml.Unmarshal([]byte(tt.yaml), &doc); err != nil {
				t.Fatalf("failed to parse YAML: %v", err)
			}

			if err := m.Apply(&doc); err != nil {
				t.Fatalf("Apply() error = %v", err)
			}

			// Verify the values
			root := GetDocumentRoot(&doc)
			entities := GetMapValue(root, "entities")
			if entities == nil {
				t.Fatal("entities not found")
			}

			for i := 0; i < len(entities.Content)-1; i += 2 {
				entityName := entities.Content[i].Value
				entityDef := entities.Content[i+1]

				expected, ok := tt.wantValues[entityName]
				if !ok {
					continue
				}

				// Check for id_prefix (scalar) or id_prefixes (sequence)
				switch v := expected.(type) {
				case string:
					// Expected id_prefix as scalar
					idPrefixValue := GetMapValue(entityDef, "id_prefix")
					switch {
					case idPrefixValue == nil:
						t.Errorf("entity %s: id_prefix not found", entityName)
					case idPrefixValue.Kind != yaml.ScalarNode:
						t.Errorf("entity %s: id_prefix is not a scalar (got %v)", entityName, idPrefixValue.Kind)
					case idPrefixValue.Value != v:
						t.Errorf("entity %s: id_prefix = %q, want %q", entityName, idPrefixValue.Value, v)
					}
					// Ensure id_prefixes doesn't exist
					if GetMapValue(entityDef, "id_prefixes") != nil {
						t.Errorf("entity %s: id_prefixes should not exist when id_prefix is used", entityName)
					}
					// Ensure id_patterns doesn't exist
					if GetMapValue(entityDef, "id_patterns") != nil {
						t.Errorf("entity %s: id_patterns should not exist after migration", entityName)
					}

				case []string:
					// Expected id_prefixes as sequence
					idPrefixesValue := GetMapValue(entityDef, "id_prefixes")
					switch {
					case idPrefixesValue == nil:
						t.Errorf("entity %s: id_prefixes not found", entityName)
					case idPrefixesValue.Kind != yaml.SequenceNode:
						t.Errorf("entity %s: id_prefixes is not a sequence (got %v)", entityName, idPrefixesValue.Kind)
					default:
						// Verify sequence contents
						if len(idPrefixesValue.Content) != len(v) {
							t.Errorf("entity %s: id_prefixes has %d elements, want %d", entityName, len(idPrefixesValue.Content), len(v))
						} else {
							for j, expected := range v {
								if idPrefixesValue.Content[j].Value != expected {
									t.Errorf("entity %s: id_prefixes[%d] = %q, want %q", entityName, j, idPrefixesValue.Content[j].Value, expected)
								}
							}
						}
					}
					// Ensure id_prefix doesn't exist
					if GetMapValue(entityDef, "id_prefix") != nil {
						t.Errorf("entity %s: id_prefix should not exist when id_prefixes is used", entityName)
					}
					// Ensure id_patterns doesn't exist
					if GetMapValue(entityDef, "id_patterns") != nil {
						t.Errorf("entity %s: id_patterns should not exist after migration", entityName)
					}
				}
			}
		})
	}
}

func TestIDPrefixRenameMigration_PreservesComments(t *testing.T) {
	input := `# Metamodel config
version: "1.0"

entities:
  # Requirements entity
  requirement:
    label: Requirement
    id_type: auto
    id_patterns: ["REQ-"]  # Legacy patterns field

  # Components with multiple patterns
  component:
    label: Component
    id_patterns: ["COMP-", "CMP-"]
`

	var doc yaml.Node
	if err := yaml.Unmarshal([]byte(input), &doc); err != nil {
		t.Fatalf("failed to parse YAML: %v", err)
	}

	m := &IDPrefixRenameMigration{}
	if err := m.Apply(&doc); err != nil {
		t.Fatalf("Apply() error = %v", err)
	}

	// Re-encode and check for comments and changes
	output, err := yaml.Marshal(&doc)
	if err != nil {
		t.Fatalf("failed to marshal YAML: %v", err)
	}

	outputStr := string(output)

	// Check that the values were changed correctly
	if !strings.Contains(outputStr, "id_prefix:") {
		t.Error("id_prefix was not created for single-element array")
	}
	if !strings.Contains(outputStr, "id_prefixes:") {
		t.Error("id_prefixes was not created for multi-element array")
	}
	if strings.Contains(outputStr, "id_patterns:") {
		t.Error("id_patterns should have been removed")
	}

	// Check that comments are preserved (yaml.v3 preserves head comments)
	if !strings.Contains(outputStr, "# Metamodel config") {
		t.Error("top comment was not preserved")
	}
	if !strings.Contains(outputStr, "# Requirements entity") {
		t.Error("entity comment was not preserved")
	}
}

func TestIDPrefixRenameMigration_EmptyArray(t *testing.T) {
	input := `
version: "1.0"
entities:
  requirement:
    label: Requirement
    id_patterns: []
`

	var doc yaml.Node
	if err := yaml.Unmarshal([]byte(input), &doc); err != nil {
		t.Fatalf("failed to parse YAML: %v", err)
	}

	m := &IDPrefixRenameMigration{}
	if err := m.Apply(&doc); err != nil {
		t.Fatalf("Apply() error = %v", err)
	}

	// Verify it was renamed to id_prefixes (since it's not a single-element array)
	root := GetDocumentRoot(&doc)
	entities := GetMapValue(root, "entities")
	requirement := GetMapValue(entities, "requirement")

	if GetMapValue(requirement, "id_patterns") != nil {
		t.Error("id_patterns should have been removed")
	}
	if GetMapValue(requirement, "id_prefixes") == nil {
		t.Error("id_prefixes should have been created for empty array")
	}
}

func TestIDPrefixRenameMigration_Metadata(t *testing.T) {
	m := &IDPrefixRenameMigration{}

	if m.Name() != "id-prefix-rename" {
		t.Errorf("Name() = %q, want %q", m.Name(), "id-prefix-rename")
	}

	if m.Description() == "" {
		t.Error("Description() should not be empty")
	}

	fileTypes := m.FileTypes()
	if len(fileTypes) != 1 || fileTypes[0] != FileTypeMetamodel {
		t.Errorf("FileTypes() = %v, want [%v]", fileTypes, FileTypeMetamodel)
	}
}
