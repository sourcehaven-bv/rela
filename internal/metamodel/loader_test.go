package metamodel

import (
	"errors"
	"fmt"
	"io/fs"
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
	meta, _, err := Load(tmpFile, testMetaFS)
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
	_, _, err := Load("/nonexistent/metamodel.yaml", testMetaFS)
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

	_, _, err := Load(tmpFile, testMetaFS)
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

// ----------------------------------------------------------------------
// TKT-RT3Y3: display_property field on entity-type definitions.
// ----------------------------------------------------------------------

// TestParse_DisplayPropertySucceeds verifies a metamodel with an
// explicit, well-formed display_property loads cleanly and the field
// round-trips onto the parsed EntityDef.
func TestParse_DisplayPropertySucceeds(t *testing.T) {
	yaml := `version: "1.0"
entities:
  document:
    label: Document
    id_prefix: "DOC-"
    display_property: titel
    properties:
      titel:
        type: string
        required: true
      doctype:
        type: string
        required: true
`
	m, err := Parse([]byte(yaml))
	assertNoError(t, err)
	assertEqual(t, m.Entities["document"].DisplayProperty, "titel")
}

// TestParse_DisplayPropertyMissing verifies the load-time existence
// check (RR-9CW5N motivation): pointing at a property that isn't
// declared on the entity must error with a diagnostic listing the
// available properties.
func TestParse_DisplayPropertyMissing(t *testing.T) {
	yaml := `version: "1.0"
entities:
  document:
    label: Document
    id_prefix: "DOC-"
    display_property: nonexistent
    properties:
      titel:
        type: string
        required: true
`
	_, err := Parse([]byte(yaml))
	assertError(t, err)
	if !strings.Contains(err.Error(), `display_property "nonexistent"`) {
		t.Errorf("error should name the offending property: %v", err)
	}
	if !strings.Contains(err.Error(), "titel") {
		t.Errorf("error should list available properties (titel): %v", err)
	}
	if !strings.Contains(err.Error(), `entity "document"`) {
		t.Errorf("error should name the entity: %v", err)
	}
}

// TestParse_DisplayPropertyWhitespace verifies the explicit whitespace
// check (RR-HDAX8): leading/trailing whitespace produces a dedicated
// diagnostic distinct from the missing-property error, so authors get
// a useful fix. The diagnostic also lists the available property names
// (RR-MPE9Y) so the author can fix a co-occurring typo in one round.
func TestParse_DisplayPropertyWhitespace(t *testing.T) {
	yaml := `version: "1.0"
entities:
  document:
    label: Document
    id_prefix: "DOC-"
    display_property: " titel "
    properties:
      titel:
        type: string
        required: true
`
	_, err := Parse([]byte(yaml))
	assertError(t, err)
	msg := err.Error()
	for _, want := range []string{"display_property", "whitespace", "titel"} {
		if !strings.Contains(msg, want) {
			t.Errorf("error should mention %q (RR-MPE9Y / RR-KFQD4): %v", want, err)
		}
	}
}

// TestParse_DisplayPropertyYAMLNull verifies that an explicit YAML null
// (display_property:) is treated as unset — no error, GetPrimaryProperty
// falls through to the autoderivation. RR-HP5IE.
func TestParse_DisplayPropertyYAMLNull(t *testing.T) {
	yaml := `version: "1.0"
entities:
  document:
    label: Document
    id_prefix: "DOC-"
    display_property:
    properties:
      titel:
        type: string
        required: true
`
	m, err := Parse([]byte(yaml))
	assertNoError(t, err)
	def := m.Entities["document"]
	assertEqual(t, def.DisplayProperty, "")
	// autoderivation: titel is the only required string property.
	assertEqual(t, def.GetPrimaryProperty(), "titel")
}

// TestParse_DisplayPropertyCaseSensitive verifies that case-mismatched
// references fail validation. The Go map lookup is case-sensitive; this
// test pins the behavior so a future "helpful" fuzzy-match refactor
// doesn't silently change semantics. RR-GO9T7.
func TestParse_DisplayPropertyCaseSensitive(t *testing.T) {
	yaml := `version: "1.0"
entities:
  document:
    label: Document
    id_prefix: "DOC-"
    display_property: TITEL
    properties:
      titel:
        type: string
        required: true
`
	_, err := Parse([]byte(yaml))
	assertError(t, err)
	if !strings.Contains(err.Error(), `"TITEL"`) {
		t.Errorf("error should reflect the case-mismatched name: %v", err)
	}
}

// TestParse_DisplayPropertyEnumOK verifies that pointing display_property
// at a non-required, non-string property is accepted at load time —
// the runtime stringifies the value via DisplayTitle. RR-9CW5N.
func TestParse_DisplayPropertyEnumOK(t *testing.T) {
	yaml := `version: "1.0"
types:
  ticket_status:
    values: [open, closed]
entities:
  ticket:
    label: Ticket
    id_prefix: "TKT-"
    display_property: status
    properties:
      titel:
        type: string
        required: true
      status:
        type: ticket_status
`
	m, err := Parse([]byte(yaml))
	assertNoError(t, err)
	def := m.Entities["ticket"]
	assertEqual(t, def.DisplayProperty, "status")
	assertEqual(t, def.GetPrimaryProperty(), "status")
}

// TestParse_DisplayPropertyList rejects a list-typed display_property at
// load time so authors don't end up with display names like "[a b c]".
// RR-AVOMV.
func TestParse_DisplayPropertyList(t *testing.T) {
	yaml := `version: "1.0"
entities:
  document:
    label: Document
    id_prefix: "DOC-"
    display_property: tags
    properties:
      titel:
        type: string
        required: true
      tags:
        type: string
        list: true
`
	_, err := Parse([]byte(yaml))
	assertError(t, err)
	msg := err.Error()
	for _, want := range []string{"display_property", "tags", "list"} {
		if !strings.Contains(msg, want) {
			t.Errorf("error should mention %q: %v", want, err)
		}
	}
}

// TestParse_DisplayPropertyDateRejected rejects date-typed properties:
// time.Time stringifies to "2026-04-25 00:00:00 +0000 UTC" which is
// almost never what an author wants for a display name. RR-IG4JJ.
func TestParse_DisplayPropertyDateRejected(t *testing.T) {
	yaml := `version: "1.0"
entities:
  document:
    label: Document
    id_prefix: "DOC-"
    display_property: created_at
    properties:
      titel:
        type: string
        required: true
      created_at:
        type: date
`
	_, err := Parse([]byte(yaml))
	assertError(t, err)
	msg := err.Error()
	for _, want := range []string{"display_property", "created_at", "date"} {
		if !strings.Contains(msg, want) {
			t.Errorf("error should mention %q: %v", want, err)
		}
	}
}

// TestParse_DisplayPropertyFileRejected rejects file-typed properties.
// RR-IG4JJ.
func TestParse_DisplayPropertyFileRejected(t *testing.T) {
	yaml := `version: "1.0"
entities:
  document:
    label: Document
    id_prefix: "DOC-"
    display_property: attachment
    properties:
      titel:
        type: string
        required: true
      attachment:
        type: file
`
	_, err := Parse([]byte(yaml))
	assertError(t, err)
	if !strings.Contains(err.Error(), "file") {
		t.Errorf("error should mention the file type: %v", err)
	}
}

// TestParse_DisplayPropertyRruleRejected rejects rrule-typed properties.
// RR-IG4JJ.
func TestParse_DisplayPropertyRruleRejected(t *testing.T) {
	yaml := `version: "1.0"
entities:
  schedule:
    label: Schedule
    id_prefix: "SCH-"
    display_property: cadence
    properties:
      titel:
        type: string
        required: true
      cadence:
        type: rrule
`
	_, err := Parse([]byte(yaml))
	assertError(t, err)
	if !strings.Contains(err.Error(), "rrule") {
		t.Errorf("error should mention the rrule type: %v", err)
	}
}

// TestParse_DisplayPropertyIntegerOK accepts an integer-typed display
// property — it stringifies via fmt.Sprintf("%v", val) at runtime.
// RR-IG4JJ pins down the allowed types alongside string/enum/boolean.
func TestParse_DisplayPropertyIntegerOK(t *testing.T) {
	yaml := `version: "1.0"
entities:
  ticket:
    label: Ticket
    id_prefix: "TKT-"
    display_property: number
    properties:
      titel:
        type: string
        required: true
      number:
        type: integer
`
	m, err := Parse([]byte(yaml))
	assertNoError(t, err)
	def := m.Entities["ticket"]
	assertEqual(t, def.DisplayProperty, "number")
}

// TestParse_DisplayPropertyAcrossIncludes verifies that display_property
// validation runs on the merged metamodel when an entity is defined in
// an included file. RR-GO9T7.
func TestParse_DisplayPropertyAcrossIncludes(t *testing.T) {
	tmpDir := t.TempDir()
	parentPath := filepath.Join(tmpDir, "metamodel.yaml")
	childPath := filepath.Join(tmpDir, "child.yaml")

	parentYAML := `version: "1.0"
includes:
  - child.yaml
entities:
  document:
    label: Document
    id_prefix: "DOC-"
    properties:
      titel:
        type: string
        required: true
`
	childYAML := `entities:
  applicatie:
    label: Applicatie
    id_prefix: "APP-"
    display_property: naam
    properties:
      naam:
        type: string
        required: true
`
	createFile(t, parentPath, parentYAML)
	createFile(t, childPath, childYAML)

	m, _, err := Load(parentPath, testMetaFS)
	assertNoError(t, err)
	def := m.Entities["applicatie"]
	assertEqual(t, def.DisplayProperty, "naam")
	assertEqual(t, def.GetPrimaryProperty(), "naam")
}

// TestLoad_AllShippedMetamodels guards against typos in dogfood and
// fixture metamodels once display_property gets adopted in any of them.
// Globs every shipped metamodel under the repo root and asserts each
// one loads without error. RR-G175B / RR-XMJX1 / RR-MT7J7 / RR-YN32Y.
//
// "Shipped metamodel" means any file matching `*metamodel*.yaml` —
// that includes the dogfood `tickets/metamodel.yaml`, the docs-project
// metamodel, every prototype's `metamodel.yaml`, and named variants
// like `prototypes/data-entry/catalog-metamodel.yaml`. Negative-case
// fixture metamodels under `testdata/` and `fixtures/` are excluded.
//
// New metamodels get covered automatically the moment they land on
// disk; nothing to keep in sync.
func TestLoad_AllShippedMetamodels(t *testing.T) {
	repoRoot := findRepoRoot(t)

	skipDirs := map[string]bool{
		"node_modules":  true,
		".git":          true,
		".ignored":      true,
		"test-fixtures": true,
		"testdata":      true,
		"fixtures":      true,
	}

	var paths []string
	err := filepath.WalkDir(repoRoot, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			// d may be nil here. Per filepath.WalkDir docs, returning
			// SkipDir from a *file* callback skips remaining files in
			// the parent directory — not what we want for a transient
			// FS hiccup. Log and continue.
			t.Logf("walk: skipping %s: %v", path, walkErr)
			if d != nil && d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if d.IsDir() {
			if skipDirs[d.Name()] {
				return filepath.SkipDir
			}
			return nil
		}
		name := d.Name()
		if !strings.HasSuffix(name, ".yaml") || !strings.Contains(name, "metamodel") {
			return nil
		}
		paths = append(paths, path)
		return nil
	})
	assertNoError(t, err)

	if len(paths) == 0 {
		t.Fatalf("expected to find at least one shipped metamodel under %s; "+
			"if this test is no longer rooted in the repo, fix findRepoRoot", repoRoot)
	}

	for _, p := range paths {
		t.Run(strings.TrimPrefix(p, repoRoot+"/"), func(t *testing.T) {
			_, _, err := Load(p, testMetaFS)
			if err != nil {
				t.Errorf("Load(%s) failed: %v", p, err)
			}
		})
	}
}

// findRepoRoot walks up from the test's working directory until it
// finds a directory containing go.mod, then returns it. Fails the test
// if go.mod is never found — indicates the package was moved without
// updating this helper. Replaces the previous "../.." dead reckoning,
// which would silently produce a wrong root if the package ever moved.
func findRepoRoot(t *testing.T) string {
	t.Helper()
	cwd, err := os.Getwd()
	assertNoError(t, err)
	dir := cwd
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatalf("findRepoRoot: walked up from %s to root without finding go.mod", cwd)
		}
		dir = parent
	}
}
