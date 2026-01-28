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

func TestMatchValue(t *testing.T) {
	tests := []struct {
		name     string
		value    interface{}
		filter   string
		expected bool
	}{
		// Equal operator
		{"equal string match", "published", "status=published", true},
		{"equal string no match", "draft", "status=published", false},
		{"equal int match", 5, "priority=5", true},
		{"equal int no match", 3, "priority=5", false},

		// Not equal operator
		{"not equal match", "draft", "status!=published", true},
		{"not equal no match", "published", "status!=published", false},

		// Less than
		{"less than true", "2", "priority<3", true},
		{"less than false", "5", "priority<3", false},
		{"less than equal", "3", "priority<3", false},

		// Less or equal
		{"less or equal true", "3", "priority<=3", true},
		{"less or equal false", "5", "priority<=3", false},

		// Greater than
		{"greater than true", "5", "priority>3", true},
		{"greater than false", "2", "priority>3", false},
		{"greater than equal", "3", "priority>3", false},

		// Greater or equal
		{"greater or equal true", "3", "priority>=3", true},
		{"greater or equal false", "2", "priority>=3", false},

		// Glob patterns
		{"glob match", "authentication", "title=*auth*", true},
		{"glob no match", "login", "title=*auth*", false},

		// Regex patterns
		{"regex match", "Authentication", "title=~^[A-Z].*", true},
		{"regex no match", "authentication", "title=~^[A-Z].*", false},

		// Type conversions
		{"int to string", 42, "value=42", true},
		{"bool to string", true, "flag=true", true},
		{"bool false to string", false, "flag=false", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f, err := Parse(tt.filter)
			if err != nil {
				t.Fatalf("Parse(%q) error: %v", tt.filter, err)
			}

			result := MatchValue(tt.value, f)
			if result != tt.expected {
				t.Errorf("MatchValue(%v, %v): expected %v, got %v", tt.value, tt.filter, tt.expected, result)
			}
		})
	}
}
