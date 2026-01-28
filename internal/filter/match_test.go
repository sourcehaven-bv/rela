package filter

import (
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/model"
)

func TestMatchString(t *testing.T) {
	propDef := &metamodel.PropertyDef{Type: metamodel.PropertyTypeString}
	mm := &metamodel.Metamodel{}

	tests := []struct {
		name    string
		value   interface{}
		filter  string
		want    bool
		wantErr bool
	}{
		{"exact match", "hello", "title=hello", true, false},
		{"no match", "hello", "title=world", false, false},
		{"glob match", "A.9.1.1", "title=A.9.*", true, false},
		{"glob no match", "A.10.1", "title=A.9.*", false, false},
		{"regex match", "access control policy", "title=~access.*policy", true, false},
		{"regex no match", "security policy", "title=~access.*policy", false, false},
		{"not equal match", "hello", "title!=world", true, false},
		{"not equal no match", "hello", "title!=hello", false, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			entity := &model.Entity{
				ID:         "TEST-001",
				Type:       "test",
				Properties: map[string]interface{}{"title": tt.value},
			}

			f, err := Parse(tt.filter)
			if err != nil {
				t.Fatalf("Parse(%q) error: %v", tt.filter, err)
			}

			got, err := Match(entity, f, propDef, mm)
			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("Match error: %v", err)
			}
			if got != tt.want {
				t.Errorf("Match = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestMatchDate(t *testing.T) {
	propDef := &metamodel.PropertyDef{
		Type:   metamodel.PropertyTypeDate,
		Format: "2006-01-02",
	}
	mm := &metamodel.Metamodel{}

	tests := []struct {
		name    string
		value   interface{}
		filter  string
		want    bool
		wantErr bool
	}{
		{"equal", "2025-02-01", "valid_until=2025-02-01", true, false},
		{"not equal", "2025-02-01", "valid_until=2025-03-01", false, false},
		{"less than true", "2025-01-15", "valid_until<2025-02-01", true, false},
		{"less than false", "2025-02-15", "valid_until<2025-02-01", false, false},
		{"less than equal true", "2025-02-01", "valid_until<=2025-02-01", true, false},
		{"greater than true", "2025-03-01", "valid_until>2025-02-01", true, false},
		{"greater than false", "2025-01-01", "valid_until>2025-02-01", false, false},
		{"greater than equal true", "2025-02-01", "valid_until>=2025-02-01", true, false},
		{"invalid entity date", "not-a-date", "valid_until=2025-02-01", false, true},
		{"invalid filter date", "2025-02-01", "valid_until=not-a-date", false, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			entity := &model.Entity{
				ID:         "TEST-001",
				Type:       "test",
				Properties: map[string]interface{}{"valid_until": tt.value},
			}

			f, err := Parse(tt.filter)
			if err != nil {
				t.Fatalf("Parse(%q) error: %v", tt.filter, err)
			}

			got, err := Match(entity, f, propDef, mm)
			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("Match error: %v", err)
			}
			if got != tt.want {
				t.Errorf("Match = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestMatchInteger(t *testing.T) {
	propDef := &metamodel.PropertyDef{Type: metamodel.PropertyTypeInteger}
	mm := &metamodel.Metamodel{}

	tests := []struct {
		name    string
		value   interface{}
		filter  string
		want    bool
		wantErr bool
	}{
		{"equal int", 5, "score=5", true, false},
		{"equal string", "5", "score=5", true, false},
		{"not equal", 5, "score=10", false, false},
		{"less than true", 3, "score<5", true, false},
		{"less than false", 7, "score<5", false, false},
		{"greater than true", 8, "score>5", true, false},
		{"greater than false", 3, "score>5", false, false},
		{"less equal boundary", 5, "score<=5", true, false},
		{"greater equal boundary", 5, "score>=5", true, false},
		{"invalid entity value", "abc", "score=5", false, true},
		{"invalid filter value", 5, "score=abc", false, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			entity := &model.Entity{
				ID:         "TEST-001",
				Type:       "test",
				Properties: map[string]interface{}{"score": tt.value},
			}

			f, err := Parse(tt.filter)
			if err != nil {
				t.Fatalf("Parse(%q) error: %v", tt.filter, err)
			}

			got, err := Match(entity, f, propDef, mm)
			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("Match error: %v", err)
			}
			if got != tt.want {
				t.Errorf("Match = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestMatchBoolean(t *testing.T) {
	propDef := &metamodel.PropertyDef{Type: metamodel.PropertyTypeBoolean}
	mm := &metamodel.Metamodel{}

	tests := []struct {
		name    string
		value   interface{}
		filter  string
		want    bool
		wantErr bool
	}{
		{"equal true bool", true, "archived=true", true, false},
		{"equal false bool", false, "archived=false", true, false},
		{"equal true string", "true", "archived=true", true, false},
		{"equal false string", "false", "archived=false", true, false},
		{"not equal", true, "archived=false", false, false},
		{"not equal operator", true, "archived!=false", true, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			entity := &model.Entity{
				ID:         "TEST-001",
				Type:       "test",
				Properties: map[string]interface{}{"archived": tt.value},
			}

			f, err := Parse(tt.filter)
			if err != nil {
				t.Fatalf("Parse(%q) error: %v", tt.filter, err)
			}

			got, err := Match(entity, f, propDef, mm)
			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("Match error: %v", err)
			}
			if got != tt.want {
				t.Errorf("Match = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestMatchEnum(t *testing.T) {
	propDef := &metamodel.PropertyDef{
		Type:   metamodel.PropertyTypeEnum,
		Values: []string{"draft", "proposed", "accepted"},
	}
	mm := &metamodel.Metamodel{}

	tests := []struct {
		name    string
		value   interface{}
		filter  string
		want    bool
		wantErr bool
	}{
		{"exact match", "draft", "status=draft", true, false},
		{"no match", "draft", "status=accepted", false, false},
		{"not equal match", "draft", "status!=accepted", true, false},
		{"invalid filter value", "draft", "status=invalid", false, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			entity := &model.Entity{
				ID:         "TEST-001",
				Type:       "test",
				Properties: map[string]interface{}{"status": tt.value},
			}

			f, err := Parse(tt.filter)
			if err != nil {
				t.Fatalf("Parse(%q) error: %v", tt.filter, err)
			}

			got, err := Match(entity, f, propDef, mm)
			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("Match error: %v", err)
			}
			if got != tt.want {
				t.Errorf("Match = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestMatchNilValue(t *testing.T) {
	propDef := &metamodel.PropertyDef{Type: metamodel.PropertyTypeString}
	mm := &metamodel.Metamodel{}

	entity := &model.Entity{
		ID:         "TEST-001",
		Type:       "test",
		Properties: map[string]interface{}{},
	}

	// Test that missing property with empty filter value matches
	f, _ := Parse("title=")
	got, err := Match(entity, f, propDef, mm)
	if err != nil {
		t.Fatalf("Match error: %v", err)
	}
	if !got {
		t.Error("Expected nil property to match empty filter value")
	}

	// Test that missing property with non-empty filter value doesn't match
	f, _ = Parse("title=hello")
	got, err = Match(entity, f, propDef, mm)
	if err != nil {
		t.Fatalf("Match error: %v", err)
	}
	if got {
		t.Error("Expected nil property to not match non-empty filter value")
	}
}

// TestMatchMissingProperty tests that entities with missing properties do not match
// any filter on that property (neither = nor !=), except when explicitly checking for empty.
// This is TICKET-003: Missing/nil properties should NOT match != filter for non-empty values.
func TestMatchMissingProperty(t *testing.T) {
	mm := &metamodel.Metamodel{}

	tests := []struct {
		name    string
		propDef *metamodel.PropertyDef
		filter  string
		want    bool
		wantErr bool
	}{
		// String property tests
		{
			name:    "missing string property should not match =value",
			propDef: &metamodel.PropertyDef{Type: metamodel.PropertyTypeString},
			filter:  "title=hello",
			want:    false,
		},
		{
			name:    "missing string property should not match !=value (TICKET-003)",
			propDef: &metamodel.PropertyDef{Type: metamodel.PropertyTypeString},
			filter:  "title!=hello",
			want:    false, // Bug: currently returns true
		},
		{
			name:    "missing string property should match = (empty)",
			propDef: &metamodel.PropertyDef{Type: metamodel.PropertyTypeString},
			filter:  "title=",
			want:    true,
		},
		{
			name:    "missing string property should not match != (empty)",
			propDef: &metamodel.PropertyDef{Type: metamodel.PropertyTypeString},
			filter:  "title!=",
			want:    false,
		},
		// Integer property tests
		{
			name:    "missing integer property should not match =0",
			propDef: &metamodel.PropertyDef{Type: metamodel.PropertyTypeInteger},
			filter:  "lines_of_code=0",
			want:    false,
		},
		{
			name:    "missing integer property should not match !=0 (TICKET-003)",
			propDef: &metamodel.PropertyDef{Type: metamodel.PropertyTypeInteger},
			filter:  "lines_of_code!=0",
			want:    false, // Bug: currently returns true
		},
		{
			name:    "missing integer property should not match >0",
			propDef: &metamodel.PropertyDef{Type: metamodel.PropertyTypeInteger},
			filter:  "lines_of_code>0",
			want:    false,
		},
		{
			name:    "missing integer property should not match <100",
			propDef: &metamodel.PropertyDef{Type: metamodel.PropertyTypeInteger},
			filter:  "lines_of_code<100",
			want:    false,
		},
		// Boolean property tests
		{
			name:    "missing boolean property should not match =true",
			propDef: &metamodel.PropertyDef{Type: metamodel.PropertyTypeBoolean},
			filter:  "archived=true",
			want:    false,
		},
		{
			name:    "missing boolean property should not match !=true (TICKET-003)",
			propDef: &metamodel.PropertyDef{Type: metamodel.PropertyTypeBoolean},
			filter:  "archived!=true",
			want:    false, // Bug: currently returns true
		},
		// Enum property tests
		{
			name:    "missing enum property should not match =value",
			propDef: &metamodel.PropertyDef{Type: metamodel.PropertyTypeEnum, Values: []string{"high", "medium", "low"}},
			filter:  "priority=high",
			want:    false,
		},
		{
			name:    "missing enum property should not match !=value (TICKET-003)",
			propDef: &metamodel.PropertyDef{Type: metamodel.PropertyTypeEnum, Values: []string{"high", "medium", "low"}},
			filter:  "priority!=high",
			want:    false, // Bug: currently returns true
		},
		// Date property tests
		{
			name:    "missing date property should not match =date",
			propDef: &metamodel.PropertyDef{Type: metamodel.PropertyTypeDate, Format: "2006-01-02"},
			filter:  "due_date=2025-01-01",
			want:    false,
		},
		{
			name:    "missing date property should not match !=date (TICKET-003)",
			propDef: &metamodel.PropertyDef{Type: metamodel.PropertyTypeDate, Format: "2006-01-02"},
			filter:  "due_date!=2025-01-01",
			want:    false, // Bug: currently returns true
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Entity with no properties set (missing property)
			entity := &model.Entity{
				ID:         "TEST-001",
				Type:       "test",
				Properties: map[string]interface{}{},
			}

			f, err := Parse(tt.filter)
			if err != nil {
				t.Fatalf("Parse(%q) error: %v", tt.filter, err)
			}

			got, err := Match(entity, f, tt.propDef, mm)
			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("Match error: %v", err)
			}
			if got != tt.want {
				t.Errorf("Match = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestMatchAllAND(t *testing.T) {
	mm := &metamodel.Metamodel{}
	entityDef := &metamodel.EntityDef{
		Properties: map[string]metamodel.PropertyDef{
			"status":   {Type: metamodel.PropertyTypeEnum, Values: []string{"draft", "accepted"}},
			"priority": {Type: metamodel.PropertyTypeEnum, Values: []string{"high", "low"}},
		},
	}

	entity := &model.Entity{
		ID:   "TEST-001",
		Type: "test",
		Properties: map[string]interface{}{
			"status":   "accepted",
			"priority": "high",
		},
	}

	// Both filters match
	filters, _ := ParseAll([]string{"status=accepted", "priority=high"})
	got, err := MatchAll(entity, filters, entityDef, mm)
	if err != nil {
		t.Fatalf("MatchAll error: %v", err)
	}
	if !got {
		t.Error("Expected both filters to match")
	}

	// One filter doesn't match
	filters, _ = ParseAll([]string{"status=accepted", "priority=low"})
	got, err = MatchAll(entity, filters, entityDef, mm)
	if err != nil {
		t.Fatalf("MatchAll error: %v", err)
	}
	if got {
		t.Error("Expected AND to fail when one filter doesn't match")
	}
}

func TestOperatorValidation(t *testing.T) {
	mm := &metamodel.Metamodel{}

	tests := []struct {
		name    string
		propDef *metamodel.PropertyDef
		filter  string
		wantErr bool
	}{
		// String doesn't support comparison operators
		{
			name:    "string less than",
			propDef: &metamodel.PropertyDef{Type: metamodel.PropertyTypeString},
			filter:  "title<abc",
			wantErr: true,
		},
		// Date doesn't support regex
		{
			name:    "date regex",
			propDef: &metamodel.PropertyDef{Type: metamodel.PropertyTypeDate, Format: "2006-01-02"},
			filter:  "date=~2025.*",
			wantErr: true,
		},
		// Boolean only supports = and !=
		{
			name:    "boolean less than",
			propDef: &metamodel.PropertyDef{Type: metamodel.PropertyTypeBoolean},
			filter:  "flag<true",
			wantErr: true,
		},
		// Enum only supports = and !=
		{
			name:    "enum greater than",
			propDef: &metamodel.PropertyDef{Type: metamodel.PropertyTypeEnum, Values: []string{"a", "b"}},
			filter:  "status>a",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			entity := &model.Entity{
				ID:         "TEST-001",
				Type:       "test",
				Properties: map[string]interface{}{"title": "test", "date": "2025-01-01", "flag": true, "status": "a"},
			}

			f, err := Parse(tt.filter)
			if err != nil {
				t.Fatalf("Parse(%q) error: %v", tt.filter, err)
			}

			_, err = Match(entity, f, tt.propDef, mm)
			if tt.wantErr && err == nil {
				t.Error("expected error for invalid operator")
			}
			if !tt.wantErr && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

// TestMatchEnumLegacy tests the matchEnumLegacy function for status and priority types
func TestMatchEnumLegacy(t *testing.T) {
	mm := &metamodel.Metamodel{}

	tests := []struct {
		name     string
		propType string
		value    interface{}
		filter   string
		want     bool
		wantErr  bool
	}{
		// Status type tests
		{"status draft match", "status", "draft", "status=draft", true, false},
		{"status draft no match", "status", "draft", "status=accepted", false, false},
		{"status not equal", "status", "draft", "status!=accepted", true, false},
		{"status invalid value", "status", "draft", "status=invalid", false, true},
		// Priority type tests
		{"priority high match", "priority", "high", "priority=high", true, false},
		{"priority high no match", "priority", "high", "priority=low", false, false},
		{"priority not equal", "priority", "high", "priority!=low", true, false},
		{"priority invalid value", "priority", "high", "priority=invalid", false, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			entity := &model.Entity{
				ID:         "TEST-001",
				Type:       "test",
				Properties: map[string]interface{}{tt.propType: tt.value},
			}

			propDef := &metamodel.PropertyDef{Type: tt.propType}

			f, err := Parse(tt.filter)
			if err != nil {
				t.Fatalf("Parse(%q) error: %v", tt.filter, err)
			}

			got, err := Match(entity, f, propDef, mm)
			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("Match error: %v", err)
			}
			if got != tt.want {
				t.Errorf("Match = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestMatchEnumLegacyWithCustomType tests that custom type overrides work for legacy types
func TestMatchEnumLegacyWithCustomType(t *testing.T) {
	mm := &metamodel.Metamodel{
		Types: map[string]metamodel.CustomType{
			"status": {
				Values: []string{"open", "closed", "pending"},
			},
		},
	}

	entity := &model.Entity{
		ID:         "TEST-001",
		Type:       "test",
		Properties: map[string]interface{}{"status": "open"},
	}

	propDef := &metamodel.PropertyDef{Type: "status"}

	// Should match with custom type value
	f, _ := Parse("status=open")
	got, err := Match(entity, f, propDef, mm)
	if err != nil {
		t.Fatalf("Match error: %v", err)
	}
	if !got {
		t.Error("Expected custom status value to match")
	}

	// Should reject old status values
	f, _ = Parse("status=draft")
	_, err = Match(entity, f, propDef, mm)
	if err == nil {
		t.Error("Expected error for value not in custom type")
	}
}

// TestMatchStringEdgeCases tests additional edge cases for matchString
func TestMatchStringEdgeCases(t *testing.T) {
	propDef := &metamodel.PropertyDef{Type: metamodel.PropertyTypeString}
	mm := &metamodel.Metamodel{}

	tests := []struct {
		name    string
		value   interface{}
		filter  string
		want    bool
		wantErr bool
	}{
		// Type error cases
		{"wrong type int", 123, "title=123", false, true},
		{"wrong type bool", true, "title=true", false, true},
		// Glob edge cases with not equal
		{"glob not equal match", "B.10.1", "title!=A.9.*", true, false},
		// Unsupported operator error
		{"unsupported operator less", "hello", "title<world", false, true},
		{"unsupported operator greater", "hello", "title>world", false, true},
		{"unsupported operator less equal", "hello", "title<=world", false, true},
		{"unsupported operator greater equal", "hello", "title>=world", false, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			entity := &model.Entity{
				ID:         "TEST-001",
				Type:       "test",
				Properties: map[string]interface{}{"title": tt.value},
			}

			f, err := Parse(tt.filter)
			if err != nil {
				t.Fatalf("Parse(%q) error: %v", tt.filter, err)
			}

			got, err := Match(entity, f, propDef, mm)
			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("Match error: %v", err)
			}
			if got != tt.want {
				t.Errorf("Match = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestMatchBooleanEdgeCases tests additional edge cases for matchBoolean
func TestMatchBooleanEdgeCases(t *testing.T) {
	propDef := &metamodel.PropertyDef{Type: metamodel.PropertyTypeBoolean}
	mm := &metamodel.Metamodel{}

	tests := []struct {
		name    string
		value   interface{}
		filter  string
		want    bool
		wantErr bool
	}{
		// Error cases - invalid value types
		{"invalid value type int", 123, "archived=true", false, true},
		{"invalid value type string", "not-a-bool", "archived=true", false, true},
		// Error cases - invalid filter values
		{"invalid filter value", true, "archived=yes", false, true},
		{"invalid filter value int", true, "archived=1", false, true},
		// Unsupported operator error
		{"unsupported operator less", true, "archived<true", false, true},
		{"unsupported operator regex", true, "archived=~true", false, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			entity := &model.Entity{
				ID:         "TEST-001",
				Type:       "test",
				Properties: map[string]interface{}{"archived": tt.value},
			}

			f, err := Parse(tt.filter)
			if err != nil {
				t.Fatalf("Parse(%q) error: %v", tt.filter, err)
			}

			got, err := Match(entity, f, propDef, mm)
			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("Match error: %v", err)
			}
			if got != tt.want {
				t.Errorf("Match = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestMatchEnumEdgeCases tests additional edge cases for matchEnum
func TestMatchEnumEdgeCases(t *testing.T) {
	propDef := &metamodel.PropertyDef{
		Type:   metamodel.PropertyTypeEnum,
		Values: []string{"draft", "proposed", "accepted"},
	}
	mm := &metamodel.Metamodel{}

	tests := []struct {
		name    string
		value   interface{}
		filter  string
		want    bool
		wantErr bool
	}{
		// Empty value tests
		{"empty filter equal to empty value", "", "status=", true, false},
		{"empty filter equal to non-empty value", "draft", "status=", false, false},
		{"empty filter not equal to empty value", "", "status!=", false, false},
		{"empty filter not equal to non-empty value", "draft", "status!=", true, false},
		// Type error cases
		{"wrong type int", 123, "status=draft", false, true},
		{"wrong type bool", true, "status=draft", false, true},
		// Unsupported operator for enum
		{"unsupported operator less", "draft", "status<accepted", false, true},
		{"unsupported operator regex", "draft", "status=~draft.*", false, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			entity := &model.Entity{
				ID:         "TEST-001",
				Type:       "test",
				Properties: map[string]interface{}{"status": tt.value},
			}

			f, err := Parse(tt.filter)
			if err != nil {
				t.Fatalf("Parse(%q) error: %v", tt.filter, err)
			}

			got, err := Match(entity, f, propDef, mm)
			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("Match error: %v", err)
			}
			if got != tt.want {
				t.Errorf("Match = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestMatchCustomType tests that custom types are properly matched
func TestMatchCustomType(t *testing.T) {
	mm := &metamodel.Metamodel{
		Types: map[string]metamodel.CustomType{
			"risk_level": {
				Values: []string{"critical", "high", "medium", "low"},
			},
		},
	}

	entity := &model.Entity{
		ID:         "TEST-001",
		Type:       "test",
		Properties: map[string]interface{}{"risk": "high"},
	}

	propDef := &metamodel.PropertyDef{Type: "risk_level"}

	// Should match with custom type value
	f, _ := Parse("risk=high")
	got, err := Match(entity, f, propDef, mm)
	if err != nil {
		t.Fatalf("Match error: %v", err)
	}
	if !got {
		t.Error("Expected custom type value to match")
	}

	// Should not match different value
	f, _ = Parse("risk=low")
	got, err = Match(entity, f, propDef, mm)
	if err != nil {
		t.Fatalf("Match error: %v", err)
	}
	if got {
		t.Error("Expected custom type value to not match")
	}

	// Should reject invalid values
	f, _ = Parse("risk=invalid")
	_, err = Match(entity, f, propDef, mm)
	if err == nil {
		t.Error("Expected error for invalid custom type value")
	}
}

// TestMatchUnknownTypeFallback tests that unknown types fall back to string matching
func TestMatchUnknownTypeFallback(t *testing.T) {
	mm := &metamodel.Metamodel{}

	entity := &model.Entity{
		ID:         "TEST-001",
		Type:       "test",
		Properties: map[string]interface{}{"unknown_prop": "some value"},
	}

	propDef := &metamodel.PropertyDef{Type: "unknown_type"}

	// Should fall back to string matching
	f, _ := Parse("unknown_prop=some value")
	got, err := Match(entity, f, propDef, mm)
	if err != nil {
		t.Fatalf("Match error: %v", err)
	}
	if !got {
		t.Error("Expected unknown type to fall back to string matching")
	}

	// Test with regex
	f, _ = Parse("unknown_prop=~some.*")
	got, err = Match(entity, f, propDef, mm)
	if err != nil {
		t.Fatalf("Match error: %v", err)
	}
	if !got {
		t.Error("Expected unknown type to support regex via string fallback")
	}
}

// TestMatchAllUnknownProperty tests MatchAll with unknown property error
func TestMatchAllUnknownProperty(t *testing.T) {
	mm := &metamodel.Metamodel{}
	entityDef := &metamodel.EntityDef{
		Properties: map[string]metamodel.PropertyDef{
			"status": {Type: metamodel.PropertyTypeEnum, Values: []string{"draft", "accepted"}},
		},
	}

	entity := &model.Entity{
		ID:   "TEST-001",
		Type: "test",
		Properties: map[string]interface{}{
			"status": "accepted",
		},
	}

	// Test with unknown property
	filters, _ := ParseAll([]string{"unknown_prop=value"})
	_, err := MatchAll(entity, filters, entityDef, mm)
	if err == nil {
		t.Error("Expected error for unknown property")
	}
}
