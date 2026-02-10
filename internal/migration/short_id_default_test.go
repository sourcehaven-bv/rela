package migration

import (
	"testing"

	"gopkg.in/yaml.v3"
)

func TestShortIDDefaultMigration_Detect(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		detect bool
	}{
		{
			name: "detects entity without id_type",
			input: `
entities:
  ticket:
    label: Ticket
    id_prefix: "TKT"
`,
			detect: true,
		},
		{
			name: "no detection when id_type exists",
			input: `
entities:
  ticket:
    label: Ticket
    id_prefix: "TKT"
    id_type: sequential
`,
			detect: false,
		},
		{
			name: "no detection when id_type is short",
			input: `
entities:
  ticket:
    label: Ticket
    id_prefix: "TKT"
    id_type: short
`,
			detect: false,
		},
		{
			name: "no detection when id_type is manual",
			input: `
entities:
  ticket:
    label: Ticket
    id_type: manual
`,
			detect: false,
		},
		{
			name: "detects one entity without id_type among several",
			input: `
entities:
  ticket:
    label: Ticket
    id_prefix: "TKT"
    id_type: sequential
  requirement:
    label: Requirement
    id_prefix: "REQ"
`,
			detect: true,
		},
		{
			name: "no detection without entities section",
			input: `
relations:
  addresses:
    from: [decision]
    to: [requirement]
`,
			detect: false,
		},
		{
			name: "no detection for empty entities section",
			input: `
entities: {}
`,
			detect: false,
		},
		{
			name: "detects deprecated id_type: auto",
			input: `
entities:
  ticket:
    label: Ticket
    id_prefix: "TKT"
    id_type: auto
`,
			detect: true,
		},
	}

	m := &ShortIDDefaultMigration{}

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

func TestShortIDDefaultMigration_Apply(t *testing.T) {
	tests := []struct {
		name            string
		input           string
		expectedIDTypes map[string]string // entity name -> expected id_type value
	}{
		{
			name: "adds id_type: sequential to entity without it",
			input: `
entities:
  ticket:
    label: Ticket
    id_prefix: "TKT"
`,
			expectedIDTypes: map[string]string{
				"ticket": "sequential",
			},
		},
		{
			name: "preserves existing id_type",
			input: `
entities:
  ticket:
    label: Ticket
    id_prefix: "TKT"
    id_type: short
`,
			expectedIDTypes: map[string]string{
				"ticket": "short",
			},
		},
		{
			name: "handles multiple entities",
			input: `
entities:
  ticket:
    label: Ticket
    id_prefix: "TKT"
    id_type: sequential
  requirement:
    label: Requirement
    id_prefix: "REQ"
  concept:
    label: Concept
    id_type: manual
`,
			expectedIDTypes: map[string]string{
				"ticket":      "sequential",
				"requirement": "sequential",
				"concept":     "manual",
			},
		},
		{
			name: "renames id_type: auto to sequential",
			input: `
entities:
  ticket:
    label: Ticket
    id_prefix: "TKT"
    id_type: auto
`,
			expectedIDTypes: map[string]string{
				"ticket": "sequential",
			},
		},
	}

	m := &ShortIDDefaultMigration{}

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

			// Check id_type values
			root := GetDocumentRoot(&doc)
			entities := GetMapValue(root, "entities")
			if entities == nil {
				t.Fatal("expected entities section")
			}

			for i := 0; i < len(entities.Content)-1; i += 2 {
				entityName := entities.Content[i].Value
				entityDef := entities.Content[i+1]

				expectedType, ok := tt.expectedIDTypes[entityName]
				if !ok {
					continue
				}

				idTypeNode := GetMapValue(entityDef, "id_type")
				if idTypeNode == nil {
					t.Errorf("entity %q: expected id_type to be set", entityName)
					continue
				}
				if idTypeNode.Value != expectedType {
					t.Errorf("entity %q: got id_type %q, want %q", entityName, idTypeNode.Value, expectedType)
				}
			}
		})
	}
}

func TestShortIDDefaultMigration_Apply_InsertPosition(t *testing.T) {
	// Test that id_type is inserted in a sensible position (after id_prefix)
	input := `
entities:
  ticket:
    label: Ticket
    id_prefix: "TKT"
    properties:
      title:
        type: string
`
	var doc yaml.Node
	if err := yaml.Unmarshal([]byte(input), &doc); err != nil {
		t.Fatalf("failed to parse YAML: %v", err)
	}

	m := &ShortIDDefaultMigration{}
	if err := m.Apply(&doc); err != nil {
		t.Fatalf("Apply() error: %v", err)
	}

	// Verify id_type was added
	root := GetDocumentRoot(&doc)
	entities := GetMapValue(root, "entities")
	entityDef := entities.Content[1] // ticket definition

	// Find positions of keys
	var idPrefixPos, idTypePos int
	for i := 0; i < len(entityDef.Content)-1; i += 2 {
		key := entityDef.Content[i].Value
		switch key {
		case "id_prefix":
			idPrefixPos = i
		case "id_type":
			idTypePos = i
		}
	}

	// id_type should come after id_prefix
	if idTypePos <= idPrefixPos {
		t.Errorf("id_type (pos %d) should come after id_prefix (pos %d)", idTypePos, idPrefixPos)
	}
}

func TestShortIDDefaultMigration_Apply_NilDocument(t *testing.T) {
	m := &ShortIDDefaultMigration{}
	err := m.Apply(nil)
	// Should not panic, just return nil
	if err != nil {
		t.Errorf("Apply(nil) should not return error, got: %v", err)
	}
}

func TestShortIDDefaultMigration_Name(t *testing.T) {
	m := &ShortIDDefaultMigration{}
	if m.Name() != "short-id-default" {
		t.Errorf("Name() = %q, want %q", m.Name(), "short-id-default")
	}
}

func TestShortIDDefaultMigration_Description(t *testing.T) {
	m := &ShortIDDefaultMigration{}
	desc := m.Description()
	if desc == "" {
		t.Error("Description() should not be empty")
	}
}

func TestShortIDDefaultMigration_FileTypes(t *testing.T) {
	m := &ShortIDDefaultMigration{}
	ft := m.FileTypes()
	if len(ft) != 1 || ft[0] != FileTypeMetamodel {
		t.Errorf("FileTypes() = %v, want [FileTypeMetamodel]", ft)
	}
}
