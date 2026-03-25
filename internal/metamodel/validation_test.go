package metamodel

import (
	"regexp"
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

func TestValidatePropertyValue_RegexValidation(t *testing.T) {
	meta := &Metamodel{
		Types: map[string]CustomType{
			"semver": {
				Validations: []TypeValidation{
					{
						Pattern: `^\d+\.\d+\.\d+$`,
						Error:   "Must be valid semver (e.g., 1.2.3)",
					},
				},
			},
		},
	}
	propDef := &PropertyDef{Type: "semver"}

	tests := []struct {
		name    string
		value   string
		wantErr bool
		errMsg  string
	}{
		{"valid semver", "1.2.3", false, ""},
		{"valid semver zeros", "0.0.0", false, ""},
		{"valid semver large", "123.456.789", false, ""},
		{"invalid no dots", "123", true, "Must be valid semver"},
		{"invalid one dot", "1.2", true, "Must be valid semver"},
		{"invalid letters", "1.2.x", true, "Must be valid semver"},
		{"invalid extra dots", "1.2.3.4", true, "Must be valid semver"},
		{"empty string skipped", "", false, ""}, // Empty strings skip regex validation
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := meta.ValidatePropertyValue("version", propDef, tt.value)
			if tt.wantErr {
				if err == nil {
					t.Errorf("expected error for %q, got nil", tt.value)
				} else if !strings.Contains(err.Error(), tt.errMsg) {
					t.Errorf("expected error containing %q, got %q", tt.errMsg, err.Error())
				}
			} else if err != nil {
				t.Errorf("unexpected error for %q: %v", tt.value, err)
			}
		})
	}
}

func TestValidatePropertyValue_MultipleRegexValidations(t *testing.T) {
	meta := &Metamodel{
		Types: map[string]CustomType{
			"rrule": {
				Description: "iCal recurrence rule",
				Validations: []TypeValidation{
					{
						Pattern: `^FREQ=`,
						Error:   "Must start with FREQ=",
					},
					{
						Pattern: `FREQ=(YEARLY|MONTHLY|WEEKLY|DAILY)`,
						Error:   "FREQ must be YEARLY, MONTHLY, WEEKLY, or DAILY",
					},
				},
			},
		},
	}
	propDef := &PropertyDef{Type: "rrule"}

	tests := []struct {
		name        string
		value       string
		wantErr     bool
		errCount    int
		errContains []string
	}{
		{
			name:    "valid weekly",
			value:   "FREQ=WEEKLY;BYDAY=MO",
			wantErr: false,
		},
		{
			name:    "valid daily",
			value:   "FREQ=DAILY",
			wantErr: false,
		},
		{
			name:        "missing FREQ prefix",
			value:       "BYDAY=MO",
			wantErr:     true,
			errCount:    2,
			errContains: []string{"Must start with FREQ=", "FREQ must be"},
		},
		{
			name:        "invalid frequency",
			value:       "FREQ=HOURLY",
			wantErr:     true,
			errCount:    1,
			errContains: []string{"FREQ must be YEARLY, MONTHLY, WEEKLY, or DAILY"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := meta.ValidatePropertyValue("schedule", propDef, tt.value)
			if tt.wantErr {
				if err == nil {
					t.Errorf("expected error for %q, got nil", tt.value)
					return
				}
				// Check error count
				errParts := strings.Split(err.Error(), "; ")
				if tt.errCount > 0 && len(errParts) != tt.errCount {
					t.Errorf("expected %d error(s), got %d: %q", tt.errCount, len(errParts), err.Error())
				}
				// Check error contains expected messages
				for _, msg := range tt.errContains {
					if !strings.Contains(err.Error(), msg) {
						t.Errorf("expected error to contain %q, got %q", msg, err.Error())
					}
				}
			} else if err != nil {
				t.Errorf("unexpected error for %q: %v", tt.value, err)
			}
		})
	}
}

func TestValidatePropertyValue_EnumWithRegexValidation(t *testing.T) {
	// A type can have both enum values AND regex validations
	meta := &Metamodel{
		Types: map[string]CustomType{
			"constrained_status": {
				Values: []string{"draft", "review", "published"},
				Validations: []TypeValidation{
					{
						Pattern: `^[a-z]+$`,
						Error:   "Must be lowercase letters only",
					},
				},
			},
		},
	}
	propDef := &PropertyDef{Type: "constrained_status"}

	// Valid enum value that also passes regex
	err := meta.ValidatePropertyValue("status", propDef, "draft")
	if err != nil {
		t.Errorf("expected valid value to pass, got: %v", err)
	}

	// Invalid enum value (not in list)
	err = meta.ValidatePropertyValue("status", propDef, "pending")
	if err == nil {
		t.Error("expected error for invalid enum value")
	}
	if !strings.Contains(err.Error(), "allowed") {
		t.Errorf("expected enum error, got: %v", err)
	}
}

func TestValidatePropertyValue_TypeWithNoValidation(t *testing.T) {
	// A custom type with no values and no validations should accept any string
	meta := &Metamodel{
		Types: map[string]CustomType{
			"alias_string": {
				Description: "Just an alias for string",
				// No Values, no Validations
			},
		},
	}
	propDef := &PropertyDef{Type: "alias_string"}

	// Any string should be valid
	err := meta.ValidatePropertyValue("field", propDef, "anything goes here!")
	if err != nil {
		t.Errorf("expected any string to be valid, got: %v", err)
	}

	// But non-strings should fail
	err = meta.ValidatePropertyValue("field", propDef, 123)
	if err == nil {
		t.Error("expected error for non-string value")
	}
}

func TestValidatePropertyValue_RegexOnStringList(t *testing.T) {
	// Compile regexes to populate the cache (normally done by loader)
	semverType := CustomType{
		Validations: []TypeValidation{
			{Pattern: `^\d+\.\d+\.\d+$`, Error: "Must be semver"},
		},
	}
	re, _ := regexp.Compile(semverType.Validations[0].Pattern)
	semverType.Validations[0].SetCompiled(re)

	meta := &Metamodel{
		Types: map[string]CustomType{
			"semver": semverType,
		},
	}
	propDef := &PropertyDef{Type: "semver", List: true}

	// All valid
	err := meta.ValidatePropertyValue("versions", propDef, []string{"1.0.0", "2.0.0"})
	if err != nil {
		t.Errorf("expected valid list, got: %v", err)
	}

	// One invalid
	err = meta.ValidatePropertyValue("versions", propDef, []string{"1.0.0", "bad"})
	if err == nil {
		t.Fatal("expected error for invalid list item")
	}
	if !strings.Contains(err.Error(), "item[1]") {
		t.Errorf("expected error to mention item index, got: %v", err)
	}
	if !strings.Contains(err.Error(), "Must be semver") {
		t.Errorf("expected error to mention validation message, got: %v", err)
	}

	// Multiple invalid - should report all errors
	err = meta.ValidatePropertyValue("versions", propDef, []string{"bad1", "1.0.0", "bad2"})
	if err == nil {
		t.Fatal("expected error for invalid list items")
	}
	if !strings.Contains(err.Error(), "item[0]") {
		t.Errorf("expected error to mention item[0], got: %v", err)
	}
	if !strings.Contains(err.Error(), "item[2]") {
		t.Errorf("expected error to mention item[2], got: %v", err)
	}
}

func TestValidatePropertyValue_RegexOnInterfaceList(t *testing.T) {
	// Compile regexes to populate the cache (normally done by loader)
	semverType := CustomType{
		Validations: []TypeValidation{
			{Pattern: `^\d+\.\d+\.\d+$`, Error: "Must be semver"},
		},
	}
	re, _ := regexp.Compile(semverType.Validations[0].Pattern)
	semverType.Validations[0].SetCompiled(re)

	meta := &Metamodel{
		Types: map[string]CustomType{
			"semver": semverType,
		},
	}
	propDef := &PropertyDef{Type: "semver", List: true}

	// All valid ([]interface{} as from YAML parsing)
	err := meta.ValidatePropertyValue("versions", propDef, []interface{}{"1.0.0", "2.0.0"})
	if err != nil {
		t.Errorf("expected valid list, got: %v", err)
	}

	// One invalid
	err = meta.ValidatePropertyValue("versions", propDef, []interface{}{"1.0.0", "bad"})
	if err == nil {
		t.Fatal("expected error for invalid list item")
	}
	if !strings.Contains(err.Error(), "item[1]") {
		t.Errorf("expected error to mention item index, got: %v", err)
	}

	// Non-string item in list
	err = meta.ValidatePropertyValue("versions", propDef, []interface{}{"1.0.0", 123})
	if err == nil {
		t.Fatal("expected error for non-string list item")
	}
	if !strings.Contains(err.Error(), "item[1]") {
		t.Errorf("expected error to mention item index, got: %v", err)
	}
	if !strings.Contains(err.Error(), "must be a string") {
		t.Errorf("expected error to mention type issue, got: %v", err)
	}
}

func TestValidatePropertyValue_EmptyListWithRegexOnly(t *testing.T) {
	// A regex-only type (no enum values) with an empty list should pass
	// (let 'required' handle empty lists if needed)
	semverType := CustomType{
		Validations: []TypeValidation{
			{Pattern: `^\d+\.\d+\.\d+$`, Error: "Must be semver"},
		},
	}
	re, _ := regexp.Compile(semverType.Validations[0].Pattern)
	semverType.Validations[0].SetCompiled(re)

	meta := &Metamodel{
		Types: map[string]CustomType{
			"semver": semverType,
		},
	}
	propDef := &PropertyDef{Type: "semver", List: true}

	// Empty list should pass for regex-only types
	err := meta.ValidatePropertyValue("versions", propDef, []string{})
	if err != nil {
		t.Errorf("expected empty list to pass for regex-only type, got: %v", err)
	}
}
