package metamodel

import (
	"errors"
	"os"
	"testing"
)

func TestParse_ReservedPropertyNames(t *testing.T) {
	tests := []struct {
		name         string
		yaml         string
		wantErr      bool
		wantPropName string
	}{
		{
			name: "valid properties",
			yaml: `
version: "1.0"
entities:
  task:
    label: Task
    id_patterns: ["TASK-"]
    properties:
      title:
        type: string
        required: true
      status:
        type: string
`,
			wantErr: false,
		},
		{
			name: "reserved property id",
			yaml: `
version: "1.0"
entities:
  task:
    label: Task
    id_patterns: ["TASK-"]
    properties:
      id:
        type: string
      title:
        type: string
`,
			wantErr:      true,
			wantPropName: "id",
		},
		{
			name: "reserved property type",
			yaml: `
version: "1.0"
entities:
  task:
    label: Task
    id_patterns: ["TASK-"]
    properties:
      type:
        type: string
      title:
        type: string
`,
			wantErr:      true,
			wantPropName: "type",
		},
		{
			name: "reserved property in second entity",
			yaml: `
version: "1.0"
entities:
  task:
    label: Task
    id_patterns: ["TASK-"]
    properties:
      title:
        type: string
  requirement:
    label: Requirement
    id_patterns: ["REQ-"]
    properties:
      id:
        type: string
`,
			wantErr:      true,
			wantPropName: "id",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := Parse([]byte(tt.yaml))

			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error for reserved property, got nil")
				}

				var reservedErr *ReservedPropertyError
				if !errors.As(err, &reservedErr) {
					t.Fatalf("expected ReservedPropertyError, got %T: %v", err, err)
				}

				if reservedErr.PropertyName != tt.wantPropName {
					t.Errorf("expected property name %q, got %q", tt.wantPropName, reservedErr.PropertyName)
				}
			} else if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestReservedPropertyError_Error(t *testing.T) {
	err := &ReservedPropertyError{
		EntityType:   "task",
		PropertyName: "type",
	}

	expected := `entity task: property "type" is reserved and cannot be used`
	if err.Error() != expected {
		t.Errorf("expected %q, got %q", expected, err.Error())
	}
}

func TestWhitespacePropertyError_Error(t *testing.T) {
	err := &WhitespacePropertyError{
		EntityType:   "task",
		PropertyName: " id",
	}

	expected := `entity task: property name " id" has leading or trailing whitespace`
	if err.Error() != expected {
		t.Errorf("expected %q, got %q", expected, err.Error())
	}
}

func TestReservedPropertyNames(t *testing.T) {
	// Verify the reserved names are what we expect
	expectedReserved := []string{"id", "type"}

	for _, name := range expectedReserved {
		if !ReservedPropertyNames[name] {
			t.Errorf("expected %q to be reserved", name)
		}
	}

	// Verify non-reserved names are not in the map
	notReserved := []string{"title", "status", "description", "name"}
	for _, name := range notReserved {
		if ReservedPropertyNames[name] {
			t.Errorf("expected %q to NOT be reserved", name)
		}
	}
}

func TestParse_ReservedPropertyNames_WhitespaceBypas(t *testing.T) {
	// Test that reserved property names cannot be bypassed with whitespace
	// See TICKET-001: Reserved property validation can be bypassed with whitespace
	tests := []struct {
		name         string
		yaml         string
		wantErr      bool
		wantPropName string // the original property name (with whitespace)
	}{
		{
			name: "leading space on id",
			yaml: `
version: "1.0"
entities:
  task:
    label: Task
    id_patterns: ["TASK-"]
    properties:
      " id":
        type: string
      title:
        type: string
`,
			wantErr:      true,
			wantPropName: " id",
		},
		{
			name: "trailing space on id",
			yaml: `
version: "1.0"
entities:
  task:
    label: Task
    id_patterns: ["TASK-"]
    properties:
      "id ":
        type: string
      title:
        type: string
`,
			wantErr:      true,
			wantPropName: "id ",
		},
		{
			name: "both leading and trailing space on id",
			yaml: `
version: "1.0"
entities:
  task:
    label: Task
    id_patterns: ["TASK-"]
    properties:
      " id ":
        type: string
      title:
        type: string
`,
			wantErr:      true,
			wantPropName: " id ",
		},
		{
			name: "leading space on type",
			yaml: `
version: "1.0"
entities:
  task:
    label: Task
    id_patterns: ["TASK-"]
    properties:
      " type":
        type: string
      title:
        type: string
`,
			wantErr:      true,
			wantPropName: " type",
		},
		{
			name: "trailing space on type",
			yaml: `
version: "1.0"
entities:
  task:
    label: Task
    id_patterns: ["TASK-"]
    properties:
      "type ":
        type: string
      title:
        type: string
`,
			wantErr:      true,
			wantPropName: "type ",
		},
		{
			name: "whitespace-only property name",
			yaml: `
version: "1.0"
entities:
  task:
    label: Task
    id_patterns: ["TASK-"]
    properties:
      "   ":
        type: string
      title:
        type: string
`,
			wantErr:      true,
			wantPropName: "   ",
		},
		{
			name: "property with internal whitespace is allowed",
			yaml: `
version: "1.0"
entities:
  task:
    label: Task
    id_patterns: ["TASK-"]
    properties:
      "some property":
        type: string
      title:
        type: string
`,
			wantErr: false,
		},
		{
			name: "tab character in property name",
			yaml: `
version: "1.0"
entities:
  task:
    label: Task
    id_patterns: ["TASK-"]
    properties:
      "	id":
        type: string
      title:
        type: string
`,
			wantErr:      true,
			wantPropName: "\tid",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := Parse([]byte(tt.yaml))

			if !tt.wantErr {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				return
			}

			if err == nil {
				t.Fatal("expected error for property name with whitespace issue, got nil")
			}

			// Verify the property name in the error matches what we expect
			gotPropName := extractPropertyName(t, err)
			if gotPropName != tt.wantPropName {
				t.Errorf("expected property name %q, got %q", tt.wantPropName, gotPropName)
			}
		})
	}
}

// extractPropertyName extracts the property name from a ReservedPropertyError or WhitespacePropertyError
func extractPropertyName(t *testing.T, err error) string {
	t.Helper()

	var reservedErr *ReservedPropertyError
	if errors.As(err, &reservedErr) {
		return reservedErr.PropertyName
	}

	var whitespaceErr *WhitespacePropertyError
	if errors.As(err, &whitespaceErr) {
		return whitespaceErr.PropertyName
	}

	t.Fatalf("expected ReservedPropertyError or WhitespacePropertyError, got %T: %v", err, err)
	return ""
}

func TestLoad(t *testing.T) {
	// Create a temporary file with valid YAML
	tmpDir := t.TempDir()
	tmpFile := tmpDir + "/metamodel.yaml"

	validYAML := `version: "1.0"
entities:
  task:
    label: Task
    id_patterns: ["TASK-"]
    properties:
      title:
        type: string
        required: true
`

	if err := os.WriteFile(tmpFile, []byte(validYAML), 0644); err != nil {
		t.Fatalf("failed to write temp file: %v", err)
	}

	// Test successful load
	meta, err := Load(tmpFile)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if meta == nil {
		t.Fatal("expected metamodel, got nil")
	}

	if meta.Version != "1.0" {
		t.Errorf("Version = %q, want %q", meta.Version, "1.0")
	}

	if _, ok := meta.Entities["task"]; !ok {
		t.Error("expected task entity to exist")
	}
}

func TestLoad_NonExistentFile(t *testing.T) {
	_, err := Load("/nonexistent/metamodel.yaml")
	if err == nil {
		t.Error("expected error for nonexistent file")
	}
}

func TestLoad_InvalidYAML(t *testing.T) {
	tmpDir := t.TempDir()
	tmpFile := tmpDir + "/invalid.yaml"

	invalidYAML := `version: "1.0"
entities:
  invalid yaml [
`

	if err := os.WriteFile(tmpFile, []byte(invalidYAML), 0644); err != nil {
		t.Fatalf("failed to write temp file: %v", err)
	}

	_, err := Load(tmpFile)
	if err == nil {
		t.Error("expected error for invalid YAML")
	}
}

func TestParse_InvalidIDType(t *testing.T) {
	yaml := `version: "1.0"
entities:
  task:
    label: Task
    id_type: invalid_type
    properties:
      title:
        type: string
`

	_, err := Parse([]byte(yaml))
	if err == nil {
		t.Error("expected error for invalid id_type")
	}

	var idTypeErr *InvalidIDTypeError
	if !errors.As(err, &idTypeErr) {
		t.Errorf("expected InvalidIDTypeError, got %T: %v", err, err)
	}
}

func TestParse_SequentialIDType(t *testing.T) {
	yaml := `version: "1.0"
entities:
  task:
    label: Task
    id_type: sequential
    properties:
      title:
        type: string
`

	meta, err := Parse([]byte(yaml))
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	if meta.Entities["task"].IDType != IDTypeSequential {
		t.Errorf("IDType = %q, want %q", meta.Entities["task"].IDType, IDTypeSequential)
	}
}

func TestParse_StringIDType(t *testing.T) {
	yaml := `version: "1.0"
entities:
  task:
    label: Task
    id_type: string
    properties:
      title:
        type: string
`

	meta, err := Parse([]byte(yaml))
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	if meta.Entities["task"].IDType != IDTypeString {
		t.Errorf("IDType = %q, want %q", meta.Entities["task"].IDType, IDTypeString)
	}
}

func TestParse_AliasMap(t *testing.T) {
	yaml := `version: "1.0"
entities:
  requirement:
    label: Requirement
    aliases: [req, reqs]
    properties:
      title:
        type: string
  decision:
    label: Decision
    aliases: [dec]
    properties:
      title:
        type: string
`

	meta, err := Parse([]byte(yaml))
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	// Check aliasMap is built correctly
	// (it's private, but we can test via ResolveAlias method)
	if resolved := meta.ResolveAlias("req"); resolved != "requirement" {
		t.Errorf("ResolveAlias(req) = %q, want %q", resolved, "requirement")
	}
	if resolved := meta.ResolveAlias("reqs"); resolved != "requirement" {
		t.Errorf("ResolveAlias(reqs) = %q, want %q", resolved, "requirement")
	}
	if resolved := meta.ResolveAlias("dec"); resolved != "decision" {
		t.Errorf("ResolveAlias(dec) = %q, want %q", resolved, "decision")
	}
	if resolved := meta.ResolveAlias("requirement"); resolved != "requirement" {
		t.Errorf("ResolveAlias(requirement) = %q, want %q", resolved, "requirement")
	}
}

func TestDefaultMetamodel(t *testing.T) {
	meta := DefaultMetamodel()

	if meta == nil {
		t.Fatal("DefaultMetamodel returned nil")
	}

	if meta.Version != "1.0" {
		t.Errorf("Version = %q, want %q", meta.Version, "1.0")
	}

	// Check expected entities exist
	expectedEntities := []string{"requirement", "decision", "solution", "component"}
	for _, name := range expectedEntities {
		if _, ok := meta.Entities[name]; !ok {
			t.Errorf("expected entity %q to exist", name)
		}
	}

	// Check expected relations exist
	expectedRelations := []string{"addresses", "implements", "realizes", "dependsOn"}
	for _, name := range expectedRelations {
		if _, ok := meta.Relations[name]; !ok {
			t.Errorf("expected relation %q to exist", name)
		}
	}

	// Check custom types exist
	if _, ok := meta.Types["status"]; !ok {
		t.Error("expected status type to exist")
	}
	if _, ok := meta.Types["priority"]; !ok {
		t.Error("expected priority type to exist")
	}
}

func TestDefaultMetamodelYAML(t *testing.T) {
	yaml := DefaultMetamodelYAML()

	if yaml == "" {
		t.Fatal("DefaultMetamodelYAML returned empty string")
	}

	// Verify it can be parsed
	meta, err := Parse([]byte(yaml))
	if err != nil {
		t.Fatalf("DefaultMetamodelYAML produces invalid YAML: %v", err)
	}

	// Verify basic structure
	if meta.Version != "1.0" {
		t.Errorf("Version = %q, want %q", meta.Version, "1.0")
	}

	if len(meta.Entities) == 0 {
		t.Error("expected entities in default metamodel")
	}

	if len(meta.Relations) == 0 {
		t.Error("expected relations in default metamodel")
	}
}

func TestInvalidIDTypeError_Error(t *testing.T) {
	err := &InvalidIDTypeError{
		EntityType: "task",
		IDType:     "invalid",
	}

	expected := `invalid id_type for entity task: invalid (must be 'sequential' or 'string')`
	if err.Error() != expected {
		t.Errorf("expected %q, got %q", expected, err.Error())
	}
}
