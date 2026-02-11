package metamodel

import (
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/model"
)

func TestValidateEntity_EmptyRequiredProperty(t *testing.T) {
	// Bug PROP-002: Empty required property should only show one error, not two
	meta := &Metamodel{
		Types: map[string]CustomType{
			"status": {
				Values:  []string{"draft", "proposed", "accepted"},
				Default: "draft",
			},
		},
		Entities: map[string]EntityDef{
			"requirement": {
				Label:    "Requirement",
				IDPrefix: "REQ-",
				Properties: map[string]PropertyDef{
					"title": {
						Type:     PropertyTypeString,
						Required: true,
					},
					"status": {
						Type:     "status",
						Required: false,
					},
				},
			},
		},
	}

	entity := &model.Entity{
		ID:   "REQ-001",
		Type: "requirement",
		Properties: map[string]interface{}{
			"title":  "", // Empty string - should only trigger ONE error
			"status": "draft",
		},
	}

	errs := meta.ValidateEntity(entity)

	// Should have exactly 1 error for missing required property
	// Bug was: it showed "missing required property" AND "invalid [type] value: "
	if len(errs) != 1 {
		t.Errorf("expected 1 error for empty required property, got %d: %v", len(errs), errs)
	}

	// Verify it's a required field error
	if len(errs) > 0 {
		if errs[0].Type != ValidationErrorRequired {
			t.Errorf("expected ValidationErrorRequired, got %v", errs[0].Type)
		}
		if errs[0].Property != "title" {
			t.Errorf("expected property 'title', got %q", errs[0].Property)
		}
	}
}

func TestValidateEntity_DateValidation_RFC3339(t *testing.T) {
	// Bug PROP-005: Date validation should accept the same formats as ParseDateValue
	meta := &Metamodel{
		Entities: map[string]EntityDef{
			"task": {
				Label:    "Task",
				IDPrefix: "TASK-",
				Properties: map[string]PropertyDef{
					"title": {
						Type:     PropertyTypeString,
						Required: true,
					},
					"due_date": {
						Type:     PropertyTypeDate,
						Required: false,
					},
				},
			},
		},
	}

	tests := []struct {
		name      string
		dateValue string
		wantErr   bool
	}{
		{
			name:      "ISO 8601 date only",
			dateValue: "2026-03-15",
			wantErr:   false,
		},
		{
			name:      "RFC3339 with Z",
			dateValue: "2026-03-15T10:30:00Z",
			wantErr:   false,
		},
		{
			name:      "RFC3339 with timezone offset",
			dateValue: "2026-03-15T10:30:00+02:00",
			wantErr:   false,
		},
		{
			name:      "ISO 8601 without timezone",
			dateValue: "2026-03-15T10:30:00",
			wantErr:   false,
		},
		{
			name:      "empty string on non-required date",
			dateValue: "",
			wantErr:   false,
		},
		{
			name:      "invalid date format",
			dateValue: "15-03-2026",
			wantErr:   true,
		},
		{
			name:      "garbage input",
			dateValue: "not-a-date",
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			entity := &model.Entity{
				ID:   "TASK-001",
				Type: "task",
				Properties: map[string]interface{}{
					"title":    "Test Task",
					"due_date": tt.dateValue,
				},
			}

			errs := meta.ValidateEntity(entity)

			if tt.wantErr && len(errs) == 0 {
				t.Errorf("expected validation error for date %q, got none", tt.dateValue)
			}
			if !tt.wantErr && len(errs) > 0 {
				t.Errorf("unexpected validation error for date %q: %v", tt.dateValue, errs)
			}
		})
	}
}

func TestValidatePropertyValue_EnumEmptyString(t *testing.T) {
	// Verify that empty strings for non-required enum properties are handled correctly
	meta := &Metamodel{
		Types: map[string]CustomType{
			"priority_type": {
				Values:  []string{"low", "medium", "high"},
				Default: "medium",
			},
		},
	}

	propDef := &PropertyDef{
		Type:     "priority_type",
		Required: false,
	}

	// Empty string should fail validation (not a valid enum value)
	err := meta.ValidatePropertyValue("priority", propDef, "")
	if err == nil {
		t.Error("expected error for empty string in enum field, got nil")
	}

	// Valid value should pass
	err = meta.ValidatePropertyValue("priority", propDef, "high")
	if err != nil {
		t.Errorf("unexpected error for valid enum value: %v", err)
	}
}

func TestValidateEntity_IDPatternValidation(t *testing.T) {
	// Verify ID pattern validation is included
	meta := &Metamodel{
		Entities: map[string]EntityDef{
			"requirement": {
				Label:    "Requirement",
				IDPrefix: "REQ-",
				Properties: map[string]PropertyDef{
					"title": {
						Type:     PropertyTypeString,
						Required: true,
					},
				},
			},
		},
	}

	tests := []struct {
		name    string
		id      string
		wantErr bool
	}{
		{
			name:    "valid ID",
			id:      "REQ-001",
			wantErr: false,
		},
		{
			name:    "invalid ID pattern",
			id:      "TASK-001",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			entity := &model.Entity{
				ID:   tt.id,
				Type: "requirement",
				Properties: map[string]interface{}{
					"title": "Test Requirement",
				},
			}

			errs := meta.ValidateEntity(entity)

			hasIDError := false
			for _, err := range errs {
				if err.Error() == "entity ID TASK-001 does not match any prefix for type requirement: [REQ-]" {
					hasIDError = true
				}
			}

			if tt.wantErr && !hasIDError {
				t.Errorf("expected ID prefix validation error, got: %v", errs)
			}
			if !tt.wantErr && hasIDError {
				t.Errorf("unexpected ID prefix validation error: %v", errs)
			}
		})
	}
}

func TestValidateRelationEntities(t *testing.T) {
	meta := &Metamodel{
		Relations: map[string]RelationDef{
			"addresses": {
				From: []string{"decision"},
				To:   []string{"requirement"},
			},
		},
	}

	from := &model.Entity{ID: "DEC-001", Type: "decision"}
	to := &model.Entity{ID: "REQ-001", Type: "requirement"}

	err := meta.ValidateRelationEntities("addresses", from, to)
	if err != nil {
		t.Errorf("ValidateRelationEntities failed: %v", err)
	}

	// Invalid from type
	invalidFrom := &model.Entity{ID: "REQ-001", Type: "requirement"}
	err = meta.ValidateRelationEntities("addresses", invalidFrom, to)
	if err == nil {
		t.Error("expected error for invalid from type")
	}
}

func TestParseIntegerValue(t *testing.T) {
	tests := []struct {
		name      string
		val       interface{}
		expected  int
		expectErr bool
	}{
		{"int", 42, 42, false},
		{"int64", int64(123), 123, false},
		{"float64", 99.0, 99, false},
		{"string", "456", 456, false},
		{"invalid string", "not a number", 0, true},
		{"bool", true, 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseIntegerValue(tt.val)
			if tt.expectErr {
				if err == nil {
					t.Error("expected error but got none")
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
				if got != tt.expected {
					t.Errorf("ParseIntegerValue(%v) = %d, want %d", tt.val, got, tt.expected)
				}
			}
		})
	}
}

func TestParseBooleanValue(t *testing.T) {
	tests := []struct {
		name      string
		val       interface{}
		expected  bool
		expectErr bool
	}{
		{"bool true", true, true, false},
		{"bool false", false, false, false},
		{"string true", "true", true, false},
		{"string false", "false", false, false},
		{"invalid string", "yes", false, true},
		{"int", 1, false, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseBooleanValue(tt.val)
			if tt.expectErr {
				if err == nil {
					t.Error("expected error but got none")
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
				if got != tt.expected {
					t.Errorf("ParseBooleanValue(%v) = %v, want %v", tt.val, got, tt.expected)
				}
			}
		})
	}
}

func TestValidatePropertyValue_Boolean(t *testing.T) {
	meta := &Metamodel{}
	propDef := &PropertyDef{Type: PropertyTypeBoolean}

	// Valid boolean
	err := meta.ValidatePropertyValue("enabled", propDef, true)
	if err != nil {
		t.Errorf("unexpected error for bool: %v", err)
	}

	// Valid string boolean
	err = meta.ValidatePropertyValue("enabled", propDef, "true")
	if err != nil {
		t.Errorf("unexpected error for string bool: %v", err)
	}

	// Invalid string
	err = meta.ValidatePropertyValue("enabled", propDef, "yes")
	if err == nil {
		t.Error("expected error for invalid bool string")
	}

	// Invalid type
	err = meta.ValidatePropertyValue("enabled", propDef, 123)
	if err == nil {
		t.Error("expected error for invalid type")
	}
}

func TestValidatePropertyValue_Integer(t *testing.T) {
	meta := &Metamodel{}
	propDef := &PropertyDef{Type: PropertyTypeInteger}

	// Valid int types
	validValues := []interface{}{42, int64(123), 99.0}
	for _, val := range validValues {
		err := meta.ValidatePropertyValue("count", propDef, val)
		if err != nil {
			t.Errorf("unexpected error for %T: %v", val, err)
		}
	}

	// Valid string
	err := meta.ValidatePropertyValue("count", propDef, "456")
	if err != nil {
		t.Errorf("unexpected error for string int: %v", err)
	}

	// Invalid string
	err = meta.ValidatePropertyValue("count", propDef, "not a number")
	if err == nil {
		t.Error("expected error for invalid int string")
	}

	// Invalid type
	err = meta.ValidatePropertyValue("count", propDef, true)
	if err == nil {
		t.Error("expected error for invalid type")
	}
}

func TestValidatePropertyValue_Enum(t *testing.T) {
	meta := &Metamodel{}
	propDef := &PropertyDef{
		Type:   PropertyTypeEnum,
		Values: []string{"small", "medium", "large"},
	}

	// Valid value
	err := meta.ValidatePropertyValue("size", propDef, "medium")
	if err != nil {
		t.Errorf("unexpected error for valid enum: %v", err)
	}

	// Invalid value
	err = meta.ValidatePropertyValue("size", propDef, "extra-large")
	if err == nil {
		t.Error("expected error for invalid enum value")
	}

	// Invalid type
	err = meta.ValidatePropertyValue("size", propDef, 123)
	if err == nil {
		t.Error("expected error for non-string enum")
	}
}

func TestValidatePropertyValue_CustomType(t *testing.T) {
	meta := &Metamodel{
		Types: map[string]CustomType{
			"severity": {
				Values: []string{"low", "medium", "high", "critical"},
			},
		},
	}
	propDef := &PropertyDef{Type: "severity"}

	// Valid value
	err := meta.ValidatePropertyValue("severity", propDef, "high")
	if err != nil {
		t.Errorf("unexpected error for valid custom type: %v", err)
	}

	// Invalid value
	err = meta.ValidatePropertyValue("severity", propDef, "extreme")
	if err == nil {
		t.Error("expected error for invalid custom type value")
	}

	// Invalid type
	err = meta.ValidatePropertyValue("severity", propDef, 123)
	if err == nil {
		t.Error("expected error for non-string custom type")
	}
}

func TestValidatePropertyValue_CustomStatusType(t *testing.T) {
	// Bug #70: Custom type named "status" should override built-in status validation
	meta := &Metamodel{
		Types: map[string]CustomType{
			"status": {
				Values:  []string{"draft", "review", "approved", "active", "completed", "on_hold", "superseded", "retired"},
				Default: "draft",
			},
		},
	}
	propDef := &PropertyDef{Type: "status"}

	// All custom values should be accepted
	for _, val := range []string{"draft", "review", "approved", "active", "completed", "on_hold", "superseded", "retired"} {
		err := meta.ValidatePropertyValue("status", propDef, val)
		if err != nil {
			t.Errorf("expected custom status value %q to be valid, got: %v", val, err)
		}
	}

	// Values not in the custom type should be rejected
	err := meta.ValidatePropertyValue("status", propDef, "proposed")
	if err == nil {
		t.Error("expected error for value not in custom status type")
	}
}

func TestValidatePropertyValue_CustomPriorityType(t *testing.T) {
	// Custom type named "priority" should override built-in priority validation
	meta := &Metamodel{
		Types: map[string]CustomType{
			"priority": {
				Values:  []string{"p0", "p1", "p2", "p3"},
				Default: "p2",
			},
		},
	}
	propDef := &PropertyDef{Type: "priority"}

	// Custom values should be accepted
	err := meta.ValidatePropertyValue("priority", propDef, "p1")
	if err != nil {
		t.Errorf("expected custom priority value to be valid, got: %v", err)
	}

	// Built-in values not in custom type should be rejected
	err = meta.ValidatePropertyValue("priority", propDef, "high")
	if err == nil {
		t.Error("expected error for value not in custom priority type")
	}
}

func TestValidatePropertyValue_UndeclaredStatusType(t *testing.T) {
	// Using type "status" without declaring it in types section should error
	meta := &Metamodel{
		Types: map[string]CustomType{},
	}
	propDef := &PropertyDef{Type: "status"}

	err := meta.ValidatePropertyValue("status", propDef, "draft")
	if err == nil {
		t.Error("expected error for undeclared status type")
	}
	if !strings.Contains(strings.ToLower(err.Error()), "unknown type") {
		t.Errorf("expected 'unknown type' in error, got: %v", err)
	}
}

func TestValidatePropertyValue_UnknownType(t *testing.T) {
	// Previously, unknown types were silently accepted (no validation).
	// Now they should return an error.
	meta := &Metamodel{
		Types: map[string]CustomType{}, // no custom types defined
	}
	propDef := &PropertyDef{Type: "nonexistent"}

	err := meta.ValidatePropertyValue("myprop", propDef, "any value")
	if err == nil {
		t.Fatal("expected error for unknown property type, got nil")
	}

	if !strings.Contains(strings.ToLower(err.Error()), "unknown type") {
		t.Errorf("expected 'unknown type' in error, got: %v", err)
	}
	if !strings.Contains(err.Error(), "nonexistent") {
		t.Errorf("expected 'nonexistent' in error, got: %v", err)
	}
}

func TestValidatePropertyValue_File(t *testing.T) {
	meta := &Metamodel{}
	propDef := &PropertyDef{Type: PropertyTypeFile}

	// Valid single file path (string)
	err := meta.ValidatePropertyValue("screenshot", propDef, "attachments/ab/abcd1234.png")
	if err != nil {
		t.Errorf("unexpected error for string file path: %v", err)
	}

	// Valid multiple files as []string
	err = meta.ValidatePropertyValue("attachments", propDef, []string{"path1.png", "path2.pdf"})
	if err != nil {
		t.Errorf("unexpected error for []string file paths: %v", err)
	}

	// Valid multiple files as []interface{} (from YAML parsing)
	err = meta.ValidatePropertyValue("attachments", propDef, []interface{}{"path1.png", "path2.pdf"})
	if err != nil {
		t.Errorf("unexpected error for []interface{} file paths: %v", err)
	}

	// Invalid: []interface{} with non-string element
	err = meta.ValidatePropertyValue("attachments", propDef, []interface{}{"path1.png", 123})
	if err == nil {
		t.Error("expected error for []interface{} with non-string element")
	}

	// Invalid type (integer)
	err = meta.ValidatePropertyValue("screenshot", propDef, 123)
	if err == nil {
		t.Error("expected error for invalid type")
	}
}
