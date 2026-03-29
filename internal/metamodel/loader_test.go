package metamodel

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/storage"
)

var testMetaFS = storage.NewOsFS()

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
    id_prefix: "TASK-"
    id_type: sequential
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
    id_prefix: "TASK-"
    id_type: sequential
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
    id_prefix: "TASK-"
    id_type: sequential
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
    id_prefix: "TASK-"
    id_type: sequential
    properties:
      title:
        type: string
  requirement:
    label: Requirement
    id_prefix: "REQ-"
    id_type: sequential
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
    id_prefix: "TASK-"
    id_type: sequential
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
    id_prefix: "TASK-"
    id_type: sequential
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
    id_prefix: "TASK-"
    id_type: sequential
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
    id_prefix: "TASK-"
    id_type: sequential
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
    id_prefix: "TASK-"
    id_type: sequential
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
    id_prefix: "TASK-"
    id_type: sequential
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
    id_prefix: "TASK-"
    id_type: sequential
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
    id_prefix: "TASK-"
    id_type: sequential
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
    id_prefix: "TASK-"
    id_type: sequential
    properties:
      title:
        type: string
        required: true
`

	createFile(t, tmpFile, validYAML)

	// Test successful load
	meta, err := Load(tmpFile, testMetaFS)
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
	_, err := Load("/nonexistent/metamodel.yaml", testMetaFS)
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

	_, err := Load(tmpFile, testMetaFS)
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

func TestParse_AutoIDType(t *testing.T) {
	yaml := `version: "1.0"
entities:
  task:
    label: Task
    id_type: sequential
    id_prefix: "TASK-"
    properties:
      title:
        type: string
`

	meta, err := Parse([]byte(yaml))
	assertNoError(t, err)

	assertEqual(t, meta.Entities["task"].IDType, IDTypeSequential)
}

func TestParse_ManualIDType(t *testing.T) {
	yaml := `version: "1.0"
entities:
  task:
    label: Task
    id_type: manual
    properties:
      title:
        type: string
`

	meta, err := Parse([]byte(yaml))
	assertNoError(t, err)

	assertEqual(t, meta.Entities["task"].IDType, IDTypeManual)
}

func TestParse_AliasMap(t *testing.T) {
	yaml := `version: "1.0"
entities:
  requirement:
    label: Requirement
    aliases: [req, reqs]
    id_prefix: "REQ-"
    id_type: sequential
    properties:
      title:
        type: string
  decision:
    label: Decision
    aliases: [dec]
    id_prefix: "DEC-"
    id_type: sequential
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

	expected := `invalid id_type for entity task: invalid (must be 'short', 'sequential', or 'manual')`
	if err.Error() != expected {
		t.Errorf("expected %q, got %q", expected, err.Error())
	}
}

func TestInvalidIDCapsError_Error(t *testing.T) {
	err := &InvalidIDCapsError{
		EntityType: "task",
		IDCaps:     "mixed",
	}

	expected := `invalid id_caps for entity task: mixed (must be 'upper' or 'lower')`
	if err.Error() != expected {
		t.Errorf("expected %q, got %q", expected, err.Error())
	}
}

func TestParse_InvalidIDCaps(t *testing.T) {
	yaml := `version: "1.0"
entities:
  task:
    label: Task
    id_type: short
    id_caps: mixed
    id_prefix: "TASK-"
    properties:
      title:
        type: string
`

	_, err := Parse([]byte(yaml))
	assertError(t, err)

	var idCapsErr *InvalidIDCapsError
	if !errors.As(err, &idCapsErr) {
		t.Errorf("expected InvalidIDCapsError, got %T: %v", err, err)
	}
}

func TestParse_ValidIDCaps(t *testing.T) {
	tests := []struct {
		name     string
		idCaps   string
		expected string
	}{
		{"upper", "upper", "upper"},
		{"lower", "lower", "lower"},
		{"empty defaults to upper", "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			yaml := fmt.Sprintf(`version: "1.0"
entities:
  task:
    label: Task
    id_type: short
    id_caps: %s
    id_prefix: "TASK-"
    properties:
      title:
        type: string
`, tt.idCaps)
			if tt.idCaps == "" {
				yaml = `version: "1.0"
entities:
  task:
    label: Task
    id_type: short
    id_prefix: "TASK-"
    properties:
      title:
        type: string
`
			}

			meta, err := Parse([]byte(yaml))
			assertNoError(t, err)

			if meta.Entities["task"].IDCaps != tt.expected {
				t.Errorf("expected IDCaps=%q, got %q", tt.expected, meta.Entities["task"].IDCaps)
			}
		})
	}
}

func TestParse_IDCapsOnNonShortType(t *testing.T) {
	// id_caps should warn when set on non-short ID types
	tests := []struct {
		name       string
		yaml       string
		wantErr    bool
		wantErrStr string
	}{
		{
			name: "id_caps on sequential type warns",
			yaml: `version: "1.0"
entities:
  task:
    label: Task
    id_type: sequential
    id_caps: upper
    id_prefix: "TASK-"
    properties:
      title:
        type: string
`,
			wantErr:    true,
			wantErrStr: "'id_caps' has no effect (only applies to 'id_type: short')",
		},
		{
			name: "id_caps on manual type warns",
			yaml: `version: "1.0"
entities:
  task:
    label: Task
    id_type: manual
    id_caps: lower
    properties:
      title:
        type: string
`,
			wantErr:    true,
			wantErrStr: "'id_caps' has no effect (only applies to 'id_type: short')",
		},
		{
			name: "id_caps on default type (short) is valid",
			yaml: `version: "1.0"
entities:
  task:
    label: Task
    id_caps: upper
    id_prefix: "TASK-"
    properties:
      title:
        type: string
`,
			wantErr: false, // default id_type is "short", so id_caps is valid
		},
		{
			name: "id_caps on short type is valid",
			yaml: `version: "1.0"
entities:
  task:
    label: Task
    id_type: short
    id_caps: upper
    id_prefix: "TASK-"
    properties:
      title:
        type: string
`,
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := Parse([]byte(tt.yaml))
			if tt.wantErr {
				if err == nil {
					t.Errorf("expected error containing %q, got nil", tt.wantErrStr)
					return
				}
				if !strings.Contains(err.Error(), tt.wantErrStr) {
					t.Errorf("expected error containing %q, got: %v", tt.wantErrStr, err)
				}
			} else if err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

func TestParse_UnknownTopLevelKeys(t *testing.T) {
	tests := []struct {
		name    string
		yaml    string
		wantErr string
	}{
		{
			name: "entity instead of entities",
			yaml: `
version: "1.0"
entity:
  requirement:
    label: Requirement
    id_prefix: "REQ-"
    id_type: sequential
    properties:
      title:
        type: string
types: {}
relations: {}
`,
			wantErr: `unknown key "entity" (did you mean "entities"?)`,
		},
		{
			name: "type instead of types",
			yaml: `
version: "1.0"
entities:
  requirement:
    label: Requirement
    id_prefix: "REQ-"
    id_type: sequential
    properties:
      title:
        type: string
type:
  status:
    values: [draft]
relations: {}
`,
			wantErr: `unknown key "type" (did you mean "types"?)`,
		},
		{
			name: "relation instead of relations",
			yaml: `
version: "1.0"
entities:
  requirement:
    label: Requirement
    id_prefix: "REQ-"
    id_type: sequential
    properties:
      title:
        type: string
types: {}
relation:
  addresses:
    label: addresses
`,
			wantErr: `unknown key "relation" (did you mean "relations"?)`,
		},
		{
			name: "completely unknown key",
			yaml: `
version: "1.0"
entities:
  requirement:
    label: Requirement
    id_prefix: "REQ-"
    id_type: sequential
    properties:
      title:
        type: string
types: {}
relations: {}
widgets:
  fancy: true
`,
			wantErr: `unknown key "widgets"`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := Parse([]byte(tt.yaml))
			if err == nil {
				t.Fatal("expected error, got nil")
			}

			var validationErr *SchemaValidationError
			if !errors.As(err, &validationErr) {
				t.Fatalf("expected SchemaValidationError, got %T: %v", err, err)
			}

			found := false
			for _, e := range validationErr.Errors {
				if strings.Contains(e, tt.wantErr) {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("expected error containing %q, got: %v", tt.wantErr, err)
			}
		})
	}
}

func TestParse_UnknownPropertyType(t *testing.T) {
	yaml := `
version: "1.0"
entities:
  requirement:
    label: Requirement
    id_prefix: "REQ-"
    id_type: sequential
    properties:
      title:
        type: string
        required: true
      priority:
        type: mypriority
types: {}
relations: {}
`
	_, err := Parse([]byte(yaml))
	assertError(t, err)

	if !strings.Contains(err.Error(), "unknown type") {
		t.Errorf("expected 'unknown type' in error, got: %v", err)
	}
	if !strings.Contains(err.Error(), "mypriority") {
		t.Errorf("expected 'mypriority' in error, got: %v", err)
	}
}

func TestParse_NumberTypeSuggestsInteger(t *testing.T) {
	tests := []struct {
		name     string
		typeName string
	}{
		{"number type", "number"},
		{"float type", "float"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			yaml := fmt.Sprintf(`
version: "1.0"
entities:
  phase:
    label: Phase
    id_prefix: "PH-"
    id_type: sequential
    properties:
      order:
        type: %s
types: {}
relations: {}
`, tt.typeName)
			_, err := Parse([]byte(yaml))
			assertError(t, err)

			if !strings.Contains(err.Error(), tt.typeName) {
				t.Errorf("expected %q in error, got: %v", tt.typeName, err)
			}
			if !strings.Contains(err.Error(), `use "integer" instead`) {
				t.Errorf("expected suggestion to use integer, got: %v", err)
			}
		})
	}
}

func TestParse_EnumWithoutValues(t *testing.T) {
	yaml := `
version: "1.0"
entities:
  requirement:
    label: Requirement
    id_prefix: "REQ-"
    id_type: sequential
    properties:
      title:
        type: string
        required: true
      priority:
        type: enum
types: {}
relations: {}
`
	_, err := Parse([]byte(yaml))
	assertError(t, err)

	if !strings.Contains(err.Error(), "enum") {
		t.Errorf("expected 'enum' in error, got: %v", err)
	}
	if !strings.Contains(err.Error(), "values") {
		t.Errorf("expected 'values' in error, got: %v", err)
	}
}

func TestParse_RelationReferencesUnknownEntityType(t *testing.T) {
	yaml := `
version: "1.0"
entities:
  requirement:
    label: Requirement
    id_prefix: "REQ-"
    id_type: sequential
    properties:
      title:
        type: string
types: {}
relations:
  addresses:
    label: addresses
    from: [nonexistent]
    to: [requirement]
`
	_, err := Parse([]byte(yaml))
	assertError(t, err)

	if !strings.Contains(err.Error(), "unknown entity type") {
		t.Errorf("expected 'unknown entity type' in error, got: %v", err)
	}
	if !strings.Contains(err.Error(), "nonexistent") {
		t.Errorf("expected 'nonexistent' in error, got: %v", err)
	}
}

func TestParse_EmptyEntities(t *testing.T) {
	yaml := `
version: "1.0"
entities: {}
types: {}
relations: {}
`
	_, err := Parse([]byte(yaml))
	assertError(t, err)

	if !strings.Contains(err.Error(), "no entity types defined") {
		t.Errorf("expected 'no entity types defined' in error, got: %v", err)
	}
}

func TestParse_MissingEntityLabel(t *testing.T) {
	yaml := `
version: "1.0"
entities:
  requirement:
    id_prefix: "REQ-"
    id_type: sequential
    properties:
      title:
        type: string
types: {}
relations: {}
`
	_, err := Parse([]byte(yaml))
	assertError(t, err)

	if !strings.Contains(err.Error(), "missing 'label'") {
		t.Errorf("expected \"missing 'label'\" in error, got: %v", err)
	}
}

func TestParse_MissingEntityProperties(t *testing.T) {
	yaml := `
version: "1.0"
entities:
  requirement:
    label: Requirement
    id_prefix: "REQ-"
    id_type: sequential
types: {}
relations: {}
`
	_, err := Parse([]byte(yaml))
	assertError(t, err)

	if !strings.Contains(err.Error(), "no properties defined") {
		t.Errorf("expected 'no properties defined' in error, got: %v", err)
	}
}

func TestParse_MissingIDPrefix(t *testing.T) {
	yaml := `
version: "1.0"
entities:
  requirement:
    label: Requirement
    properties:
      title:
        type: string
types: {}
relations: {}
`
	_, err := Parse([]byte(yaml))
	assertError(t, err)

	if !strings.Contains(err.Error(), "no ID prefix defined") {
		t.Errorf("expected 'no ID prefix defined' in error, got: %v", err)
	}
}

func TestParse_MissingIDPrefixManualIDTypeOK(t *testing.T) {
	yaml := `
version: "1.0"
entities:
  requirement:
    label: Requirement
    id_type: manual
    properties:
      title:
        type: string
types: {}
relations: {}
`
	_, err := Parse([]byte(yaml))
	assertNoError(t, err)
}

func TestParse_EmptyPropertyType(t *testing.T) {
	yaml := `
version: "1.0"
entities:
  requirement:
    label: Requirement
    id_prefix: "REQ-"
    id_type: sequential
    properties:
      title:
        type: string
      notes:
        type:
types: {}
relations: {}
`
	_, err := Parse([]byte(yaml))
	assertError(t, err)

	if !strings.Contains(err.Error(), "no type specified") {
		t.Errorf("expected 'no type specified' in error, got: %v", err)
	}
}

func TestParse_MultipleValidationErrors(t *testing.T) {
	// Multiple issues should all be reported at once
	yaml := `
version: "1.0"
entities:
  requirement:
    id_prefix: "REQ-"
    id_type: sequential
  decision:
    id_prefix: "DEC-"
    id_type: sequential
types: {}
relations: {}
`
	_, err := Parse([]byte(yaml))
	assertError(t, err)

	var validationErr *SchemaValidationError
	if !errors.As(err, &validationErr) {
		t.Fatalf("expected SchemaValidationError, got %T: %v", err, err)
	}

	// Should have multiple errors (missing label, missing properties for both entities)
	if len(validationErr.Errors) < 2 {
		t.Errorf("expected multiple validation errors, got %d: %v", len(validationErr.Errors), validationErr.Errors)
	}
}

func TestParse_ValidMetamodel(t *testing.T) {
	// Ensure a fully valid metamodel still passes
	yaml := `
version: "1.0"
namespace: "https://example.org/test#"
entities:
  requirement:
    label: Requirement
    id_prefix: "REQ-"
    id_type: sequential
    properties:
      title:
        type: string
        required: true
      status:
        type: status
  decision:
    label: Decision
    id_prefix: "DEC-"
    id_type: sequential
    properties:
      title:
        type: string
        required: true
types:
  status:
    values: [draft, accepted]
relations:
  addresses:
    label: addresses
    from: [decision]
    to: [requirement]
`
	meta, err := Parse([]byte(yaml))
	assertNoError(t, err)

	if meta == nil {
		t.Fatal("expected metamodel, got nil")
	}
	assertEqual(t, len(meta.Entities), 2)
	assertEqual(t, len(meta.Relations), 1)
}

func TestSchemaValidationError_SingleError(t *testing.T) {
	err := &SchemaValidationError{
		Errors: []string{"something is wrong"},
	}
	assertEqual(t, err.Error(), "something is wrong")
}

func TestSchemaValidationError_MultipleErrors(t *testing.T) {
	err := &SchemaValidationError{
		Errors: []string{"first problem", "second problem"},
	}
	expected := "metamodel validation errors:\n  - first problem\n  - second problem"
	assertEqual(t, err.Error(), expected)
}

func TestParse_CustomTypeWithValidations(t *testing.T) {
	yaml := `
version: "1.0"
namespace: "https://example.org/"
types:
  semver:
    description: "Semantic version"
    validations:
      - pattern: '^\d+\.\d+\.\d+$'
        error: "Must be valid semver (e.g., 1.2.3)"
entities:
  component:
    label: Component
    id_prefix: "COMP-"
    properties:
      name:
        type: string
        required: true
      version:
        type: semver
`
	meta, err := Parse([]byte(yaml))
	assertNoError(t, err)

	if meta == nil {
		t.Fatal("expected metamodel, got nil")
	}

	// Verify the type was loaded
	semver, ok := meta.Types["semver"]
	if !ok {
		t.Fatal("expected semver type to be defined")
	}
	if semver.Description != "Semantic version" {
		t.Errorf("expected description 'Semantic version', got %q", semver.Description)
	}
	if len(semver.Validations) != 1 {
		t.Fatalf("expected 1 validation, got %d", len(semver.Validations))
	}
	if semver.Validations[0].Pattern != `^\d+\.\d+\.\d+$` {
		t.Errorf("unexpected pattern: %s", semver.Validations[0].Pattern)
	}
	if semver.Validations[0].Error != "Must be valid semver (e.g., 1.2.3)" {
		t.Errorf("unexpected error message: %s", semver.Validations[0].Error)
	}
}

func TestParse_CustomTypeWithInvalidRegex(t *testing.T) {
	yaml := `
version: "1.0"
namespace: "https://example.org/"
types:
  bad_type:
    validations:
      - pattern: '[invalid(regex'
        error: "This won't work"
entities:
  item:
    label: Item
    id_prefix: "ITEM-"
    properties:
      name:
        type: string
        required: true
`
	_, err := Parse([]byte(yaml))
	if err == nil {
		t.Fatal("expected error for invalid regex pattern")
	}
	if !strings.Contains(err.Error(), "invalid regex pattern") {
		t.Errorf("expected 'invalid regex pattern' in error, got: %v", err)
	}
}

func TestParse_CustomTypeWithEmptyPattern(t *testing.T) {
	yaml := `
version: "1.0"
namespace: "https://example.org/"
types:
  bad_type:
    validations:
      - pattern: ""
        error: "Empty pattern"
entities:
  item:
    label: Item
    id_prefix: "ITEM-"
    properties:
      name:
        type: string
        required: true
`
	_, err := Parse([]byte(yaml))
	if err == nil {
		t.Fatal("expected error for empty pattern")
	}
	if !strings.Contains(err.Error(), "empty pattern") {
		t.Errorf("expected 'empty pattern' in error, got: %v", err)
	}
}

func TestParse_CustomTypeWithEmptyError(t *testing.T) {
	yaml := `
version: "1.0"
namespace: "https://example.org/"
types:
  bad_type:
    validations:
      - pattern: "^test$"
        error: ""
entities:
  item:
    label: Item
    id_prefix: "ITEM-"
    properties:
      name:
        type: string
        required: true
`
	_, err := Parse([]byte(yaml))
	if err == nil {
		t.Fatal("expected error for empty error message")
	}
	if !strings.Contains(err.Error(), "empty error message") {
		t.Errorf("expected 'empty error message' in error, got: %v", err)
	}
}

func TestParse_CustomTypeWithEnumAndValidations(t *testing.T) {
	// A type can have both enum values AND regex validations
	yaml := `
version: "1.0"
namespace: "https://example.org/"
types:
  constrained_status:
    values: [draft, review, published]
    validations:
      - pattern: '^[a-z]+$'
        error: "Must be lowercase"
entities:
  document:
    label: Document
    id_prefix: "DOC-"
    properties:
      title:
        type: string
        required: true
      status:
        type: constrained_status
`
	meta, err := Parse([]byte(yaml))
	assertNoError(t, err)

	ct := meta.Types["constrained_status"]
	if len(ct.Values) != 3 {
		t.Errorf("expected 3 enum values, got %d", len(ct.Values))
	}
	if len(ct.Validations) != 1 {
		t.Errorf("expected 1 validation, got %d", len(ct.Validations))
	}
}

func TestParse_PropertyOrderExtraction(t *testing.T) {
	// Property order in YAML should be preserved in PropertyOrder field
	yaml := `version: "1.0"
entities:
  task:
    label: Task
    id_prefix: "TASK-"
    properties:
      title:
        type: string
        required: true
      priority:
        type: string
      status:
        type: string
      assignee:
        type: string
      due_date:
        type: date
`
	meta, err := Parse([]byte(yaml))
	assertNoError(t, err)

	entityDef, ok := meta.Entities["task"]
	if !ok {
		t.Fatal("task entity not found")
	}

	order := entityDef.GetPropertyOrder()
	if order == nil {
		t.Fatal("PropertyOrder should not be nil")
	}

	expected := []string{"title", "priority", "status", "assignee", "due_date"}
	if len(order) != len(expected) {
		t.Fatalf("expected %d properties in order, got %d: %v", len(expected), len(order), order)
	}

	for i, prop := range expected {
		if order[i] != prop {
			t.Errorf("property order[%d]: expected %q, got %q", i, prop, order[i])
		}
	}
}
