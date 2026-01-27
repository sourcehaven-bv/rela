package metamodel

import (
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/model"
)

func TestValidateEntity_EmptyRequiredProperty(t *testing.T) {
	// Bug PROP-002: Empty required property should only show one error, not two
	meta := &Metamodel{
		Entities: map[string]EntityDef{
			"requirement": {
				Label:      "Requirement",
				IDPatterns: []string{"REQ-"},
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

	// Verify it's the "missing required property" error
	if len(errs) > 0 && errs[0].Error() != "missing required property: title" {
		t.Errorf("unexpected error message: %v", errs[0])
	}
}

func TestValidateEntity_DateValidation_RFC3339(t *testing.T) {
	// Bug PROP-005: Date validation should accept the same formats as ParseDateValue
	meta := &Metamodel{
		Entities: map[string]EntityDef{
			"task": {
				Label:      "Task",
				IDPatterns: []string{"TASK-"},
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
				Label:      "Requirement",
				IDPatterns: []string{"REQ-"},
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
				if err.Error() == "entity ID TASK-001 does not match any pattern for type requirement: [REQ-]" {
					hasIDError = true
				}
			}

			if tt.wantErr && !hasIDError {
				t.Errorf("expected ID pattern validation error, got: %v", errs)
			}
			if !tt.wantErr && hasIDError {
				t.Errorf("unexpected ID pattern validation error: %v", errs)
			}
		})
	}
}
