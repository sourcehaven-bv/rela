package metamodel

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

// Test helpers to avoid import cycle
func assertEqual(t *testing.T, got, want interface{}) {
	t.Helper()
	if got != want {
		t.Errorf("got %v, want %v", got, want)
	}
}

func assertNoError(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func assertError(t *testing.T, err error) {
	t.Helper()
	if err == nil {
		t.Error("expected error, got nil")
	}
}

func createFile(t *testing.T, path, content string) {
	t.Helper()
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatalf("failed to create directory %s: %v", dir, err)
	}
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write file %s: %v", path, err)
	}
}

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
	tmpFile := filepath.Join(tmpDir, "metamodel.yaml")

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

	createFile(t, tmpFile, validYAML)

	// Test successful load
	meta, err := Load(tmpFile)
	assertNoError(t, err)

	if meta == nil {
		t.Fatal("expected metamodel, got nil")
	}

	assertEqual(t, meta.Version, "1.0")

	if _, ok := meta.Entities["task"]; !ok {
		t.Error("expected task entity to exist")
	}
}

func TestLoad_NonExistentFile(t *testing.T) {
	_, err := Load("/nonexistent/metamodel.yaml")
	assertError(t, err)
}

func TestLoad_InvalidYAML(t *testing.T) {
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "invalid.yaml")

	invalidYAML := `version: "1.0"
entities:
  invalid yaml [
`

	createFile(t, tmpFile, invalidYAML)

	_, err := Load(tmpFile)
	assertError(t, err)
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
	assertError(t, err)

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
	assertNoError(t, err)

	assertEqual(t, meta.Entities["task"].IDType, IDTypeSequential)
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
	assertNoError(t, err)

	assertEqual(t, meta.Entities["task"].IDType, IDTypeString)
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
	assertNoError(t, err)

	// Check aliasMap is built correctly
	// (it's private, but we can test via ResolveAlias method)
	assertEqual(t, meta.ResolveAlias("req"), "requirement")
	assertEqual(t, meta.ResolveAlias("reqs"), "requirement")
	assertEqual(t, meta.ResolveAlias("dec"), "decision")
	assertEqual(t, meta.ResolveAlias("requirement"), "requirement")
}

func TestDefaultMetamodel(t *testing.T) {
	meta := DefaultMetamodel()

	if meta == nil {
		t.Fatal("DefaultMetamodel returned nil")
	}

	assertEqual(t, meta.Version, "1.0")

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
	assertNoError(t, err)

	// Verify basic structure
	assertEqual(t, meta.Version, "1.0")

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

	expected := `invalid id_type for entity task: invalid (must be 'auto' or 'manual')`
	if err.Error() != expected {
		t.Errorf("expected %q, got %q", expected, err.Error())
	}
}
