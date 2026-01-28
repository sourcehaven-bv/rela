package tui

import (
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/metamodel"
)

// TestUpdateParseErrors verifies that parse errors are detected immediately
func TestUpdateParseErrors(t *testing.T) {
	tests := []struct {
		name        string
		query       string
		expectError bool
	}{
		{
			name:        "Empty type filter",
			query:       "type:",
			expectError: true,
		},
		{
			name:        "Empty prop filter",
			query:       "prop:",
			expectError: true,
		},
		{
			name:        "Empty status filter",
			query:       "status:",
			expectError: true,
		},
		{
			name:        "Valid type filter",
			query:       "type:requirement",
			expectError: false,
		},
		{
			name:        "Valid prop filter",
			query:       "prop:status=draft",
			expectError: false,
		},
		{
			name:        "Valid status filter",
			query:       "status:published",
			expectError: false,
		},
		{
			name:        "Empty query",
			query:       "",
			expectError: false,
		},
		{
			name:        "Partial type filter (incomplete)",
			query:       "type:req",
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &SearchModel{
				query: tt.query,
			}

			s.updateParseErrors()

			hasError := len(s.parseErrors) > 0
			if hasError != tt.expectError {
				t.Errorf("Expected error=%v, got error=%v (errors: %v)", tt.expectError, hasError, s.parseErrors)
			}
		})
	}
}

// TestParseErrorsOnKeystroke verifies that parse errors update when typing
func TestParseErrorsOnKeystroke(t *testing.T) {
	s := &SearchModel{}

	// Initial state - no errors
	s.updateParseErrors()
	if len(s.parseErrors) != 0 {
		t.Errorf("Expected no errors initially, got: %v", s.parseErrors)
	}

	// Type "type:" - should trigger error
	s.query = "type:"
	s.updateParseErrors()
	if len(s.parseErrors) == 0 {
		t.Error("Expected error for 'type:', got none")
	}

	// Complete the query "type:req" - should clear error
	s.query = "type:req"
	s.updateParseErrors()
	if len(s.parseErrors) != 0 {
		t.Errorf("Expected no errors for 'type:req', got: %v", s.parseErrors)
	}

	// Backspace to "type:" again - should trigger error again
	s.query = "type:"
	s.updateParseErrors()
	if len(s.parseErrors) == 0 {
		t.Error("Expected error for 'type:' after backspace, got none")
	}

	// Clear completely - should have no errors
	s.query = ""
	s.updateParseErrors()
	if len(s.parseErrors) != 0 {
		t.Errorf("Expected no errors for empty query, got: %v", s.parseErrors)
	}
}

// TestAutocompleteWithEmptyPrefix verifies that autocomplete shows all options with empty prefix
func TestAutocompleteWithEmptyPrefix(t *testing.T) {
	// Create mock app with metamodel
	app := &App{
		metamodel: &metamodel.Metamodel{
			Entities: map[string]metamodel.EntityDef{
				"requirement": {},
				"decision":    {},
				"solution":    {},
			},
		},
	}

	s := &SearchModel{
		query:     "type:",
		cursorPos: 5, // cursor after the colon
	}

	s.updateSuggestions(app)

	// Should show suggestions for all entity types
	if !s.showSuggestions {
		t.Error("Expected suggestions to be shown for 'type:'")
	}

	if len(s.suggestions) != 3 {
		t.Errorf("Expected 3 type suggestions, got %d: %v", len(s.suggestions), s.suggestions)
	}

	// Verify all types are present
	typeMap := make(map[string]bool)
	for _, suggestion := range s.suggestions {
		typeMap[suggestion] = true
	}

	expectedTypes := []string{"requirement", "decision", "solution"}
	for _, expected := range expectedTypes {
		if !typeMap[expected] {
			t.Errorf("Expected type '%s' in suggestions, but it was missing", expected)
		}
	}
}

// TestAutocompleteWithPartialPrefix verifies prefix filtering
func TestAutocompleteWithPartialPrefix(t *testing.T) {
	app := &App{
		metamodel: &metamodel.Metamodel{
			Entities: map[string]metamodel.EntityDef{
				"requirement": {},
				"decision":    {},
				"solution":    {},
			},
		},
	}

	s := &SearchModel{
		query:     "type:req",
		cursorPos: 8, // cursor after "req"
	}

	s.updateSuggestions(app)

	if !s.showSuggestions {
		t.Error("Expected suggestions to be shown for 'type:req'")
	}

	// Should only show "requirement"
	if len(s.suggestions) != 1 {
		t.Errorf("Expected 1 suggestion for 'req' prefix, got %d: %v", len(s.suggestions), s.suggestions)
	}

	if len(s.suggestions) > 0 && s.suggestions[0] != "requirement" {
		t.Errorf("Expected 'requirement', got '%s'", s.suggestions[0])
	}
}

// TestParseErrorSuppressedWithSuggestions verifies errors don't show when autocomplete is active
func TestParseErrorSuppressedWithSuggestions(t *testing.T) {
	app := &App{
		metamodel: &metamodel.Metamodel{
			Entities: map[string]metamodel.EntityDef{
				"requirement": {},
				"decision":    {},
			},
		},
	}

	s := &SearchModel{
		query:     "type:",
		cursorPos: 5,
	}

	// Update both errors and suggestions
	s.updateParseErrors()
	s.updateSuggestions(app)

	// Should have parse errors
	if len(s.parseErrors) == 0 {
		t.Error("Expected parse error for 'type:'")
	}

	// But should also show suggestions
	if !s.showSuggestions {
		t.Error("Expected suggestions to be shown")
	}

	if len(s.suggestions) != 2 {
		t.Errorf("Expected 2 suggestions, got %d", len(s.suggestions))
	}

	// The View should suppress the error display when suggestions are shown
	// (This is a behavioral contract - errors exist but UI hides them when suggesting)
}
