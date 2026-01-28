package searchparser

import (
	"testing"
)

func TestParseQuery_Empty(t *testing.T) {
	sq := ParseQuery("")
	if !sq.IsEmpty() {
		t.Error("Expected empty query to be empty")
	}
	if sq.HasFilters() {
		t.Error("Expected no filters")
	}
	if sq.HasFreeText() {
		t.Error("Expected no free text")
	}
}

func TestParseQuery_SimpleText(t *testing.T) {
	sq := ParseQuery("authentication")
	if sq.HasFilters() {
		t.Error("Expected no filters")
	}
	if !sq.HasFreeText() {
		t.Error("Expected free text")
	}
	if len(sq.FreeTextWords) != 1 || sq.FreeTextWords[0] != "authentication" {
		t.Errorf("Expected free text word 'authentication', got %v", sq.FreeTextWords)
	}
}

func TestParseQuery_MultipleWords(t *testing.T) {
	sq := ParseQuery("authentication api security")
	if len(sq.FreeTextWords) != 3 {
		t.Errorf("Expected 3 free text words, got %d", len(sq.FreeTextWords))
	}
	expected := []string{"authentication", "api", "security"}
	for i, word := range expected {
		if sq.FreeTextWords[i] != word {
			t.Errorf("Expected word %d to be %s, got %s", i, word, sq.FreeTextWords[i])
		}
	}
}

func TestParseQuery_QuotedPhrase(t *testing.T) {
	sq := ParseQuery(`"REST API"`)
	if len(sq.FreeTextPhrases) != 1 {
		t.Errorf("Expected 1 phrase, got %d", len(sq.FreeTextPhrases))
	}
	if sq.FreeTextPhrases[0] != "REST API" {
		t.Errorf("Expected phrase 'REST API', got %s", sq.FreeTextPhrases[0])
	}
}

func TestParseQuery_TypeFilter(t *testing.T) {
	sq := ParseQuery("type:requirement")
	if len(sq.EntityTypes) != 1 {
		t.Errorf("Expected 1 entity type, got %d", len(sq.EntityTypes))
	}
	if sq.EntityTypes[0] != "requirement" {
		t.Errorf("Expected type 'requirement', got %s", sq.EntityTypes[0])
	}
}

func TestParseQuery_MultipleTypes(t *testing.T) {
	sq := ParseQuery("type:requirement,decision,solution")
	if len(sq.EntityTypes) != 3 {
		t.Errorf("Expected 3 entity types, got %d", len(sq.EntityTypes))
	}
	expected := []string{"requirement", "decision", "solution"}
	for i, typ := range expected {
		if sq.EntityTypes[i] != typ {
			t.Errorf("Expected type %d to be %s, got %s", i, typ, sq.EntityTypes[i])
		}
	}
}

func TestParseQuery_PropertyFilter(t *testing.T) {
	sq := ParseQuery("prop:status=published")
	if len(sq.PropertyFilters) != 1 {
		t.Errorf("Expected 1 property filter, got %d", len(sq.PropertyFilters))
	}
	f := sq.PropertyFilters[0]
	if f.Property != "status" {
		t.Errorf("Expected property 'status', got %s", f.Property)
	}
	if f.Value != "published" {
		t.Errorf("Expected value 'published', got %s", f.Value)
	}
}

func TestParseQuery_PropertyFilterGreaterThan(t *testing.T) {
	sq := ParseQuery("prop:priority>3")
	if len(sq.PropertyFilters) != 1 {
		t.Errorf("Expected 1 property filter, got %d", len(sq.PropertyFilters))
	}
	f := sq.PropertyFilters[0]
	if f.Property != "priority" {
		t.Errorf("Expected property 'priority', got %s", f.Property)
	}
	if f.Value != "3" {
		t.Errorf("Expected value '3', got %s", f.Value)
	}
}

func TestParseQuery_StatusShortcut(t *testing.T) {
	sq := ParseQuery("status:draft")
	if len(sq.PropertyFilters) != 1 {
		t.Errorf("Expected 1 property filter, got %d", len(sq.PropertyFilters))
	}
	f := sq.PropertyFilters[0]
	if f.Property != "status" {
		t.Errorf("Expected property 'status', got %s", f.Property)
	}
	if f.Value != "draft" {
		t.Errorf("Expected value 'draft', got %s", f.Value)
	}
}

func TestParseQuery_Combined(t *testing.T) {
	sq := ParseQuery(`type:requirement prop:status=published authentication "REST API"`)

	// Check entity type
	if len(sq.EntityTypes) != 1 || sq.EntityTypes[0] != "requirement" {
		t.Errorf("Expected type 'requirement', got %v", sq.EntityTypes)
	}

	// Check property filter
	if len(sq.PropertyFilters) != 1 {
		t.Errorf("Expected 1 property filter, got %d", len(sq.PropertyFilters))
	}

	// Check free text word
	if len(sq.FreeTextWords) != 1 || sq.FreeTextWords[0] != "authentication" {
		t.Errorf("Expected free text word 'authentication', got %v", sq.FreeTextWords)
	}

	// Check phrase
	if len(sq.FreeTextPhrases) != 1 || sq.FreeTextPhrases[0] != "REST API" {
		t.Errorf("Expected phrase 'REST API', got %v", sq.FreeTextPhrases)
	}
}

func TestParseQuery_InvalidPropertyFilter(t *testing.T) {
	sq := ParseQuery("prop:status")
	if len(sq.ParseErrors) == 0 {
		t.Error("Expected parse error for invalid property filter")
	}
}

func TestParseQuery_EmptyType(t *testing.T) {
	sq := ParseQuery("type:")
	if len(sq.ParseErrors) == 0 {
		t.Error("Expected parse error for empty type filter")
	}
}

func TestParseQuery_EmptyStatus(t *testing.T) {
	sq := ParseQuery("status:")
	if len(sq.ParseErrors) == 0 {
		t.Error("Expected parse error for empty status filter")
	}
}

func TestParseQuery_MultiplePropertyFilters(t *testing.T) {
	sq := ParseQuery("prop:status=published prop:priority>=2")
	if len(sq.PropertyFilters) != 2 {
		t.Errorf("Expected 2 property filters, got %d", len(sq.PropertyFilters))
	}
}

func TestTokenize(t *testing.T) {
	tests := []struct {
		input    string
		expected []string
	}{
		{"", []string{}},
		{"word", []string{"word"}},
		{"word1 word2", []string{"word1", "word2"}},
		{`"quoted phrase"`, []string{`"quoted phrase"`}},
		{`word "quoted phrase" word2`, []string{"word", `"quoted phrase"`, "word2"}},
		{`type:requirement prop:status=published`, []string{"type:requirement", "prop:status=published"}},
	}

	for _, tt := range tests {
		result := tokenize(tt.input)
		if len(result) != len(tt.expected) {
			t.Errorf("tokenize(%q): expected %d tokens, got %d", tt.input, len(tt.expected), len(result))
			continue
		}
		for i := range result {
			if result[i] != tt.expected[i] {
				t.Errorf("tokenize(%q): token %d: expected %q, got %q", tt.input, i, tt.expected[i], result[i])
			}
		}
	}
}

func TestErrorString(t *testing.T) {
	sq := &SearchQuery{
		ParseErrors: []string{"error1", "error2"},
	}
	errStr := sq.ErrorString()
	if errStr != "error1; error2" {
		t.Errorf("Expected 'error1; error2', got %s", errStr)
	}

	sq2 := &SearchQuery{}
	if sq2.ErrorString() != "" {
		t.Errorf("Expected empty error string, got %s", sq2.ErrorString())
	}
}
