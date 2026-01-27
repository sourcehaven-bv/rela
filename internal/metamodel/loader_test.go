package metamodel

import (
	"errors"
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
