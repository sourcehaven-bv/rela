package searchparser

import (
	"testing"
)

func TestGetAutocompleteContext_Empty(t *testing.T) {
	ctx := GetAutocompleteContext("", 0)
	if ctx.Type != "" {
		t.Errorf("Expected empty type for empty query, got %s", ctx.Type)
	}
}

func TestGetAutocompleteContext_Type(t *testing.T) {
	tests := []struct {
		name       string
		query      string
		cursorPos  int
		wantType   string
		wantPrefix string
	}{
		{
			name:       "type: with empty prefix",
			query:      "type:",
			cursorPos:  5,
			wantType:   "type",
			wantPrefix: "",
		},
		{
			name:       "type:req with partial prefix",
			query:      "type:req",
			cursorPos:  8,
			wantType:   "type",
			wantPrefix: "req",
		},
		{
			name:       "type:requirement with full prefix",
			query:      "type:requirement",
			cursorPos:  16,
			wantType:   "type",
			wantPrefix: "requirement",
		},
		{
			name:       "cursor in middle of prefix",
			query:      "type:requirement",
			cursorPos:  8, // After "type:req"
			wantType:   "type",
			wantPrefix: "req",
		},
		{
			name:       "type filter after text",
			query:      "something type:req",
			cursorPos:  18,
			wantType:   "type",
			wantPrefix: "req",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := GetAutocompleteContext(tt.query, tt.cursorPos)
			if ctx.Type != tt.wantType {
				t.Errorf("Expected type %q, got %q", tt.wantType, ctx.Type)
			}
			if ctx.Prefix != tt.wantPrefix {
				t.Errorf("Expected prefix %q, got %q", tt.wantPrefix, ctx.Prefix)
			}
		})
	}
}

func TestGetAutocompleteContext_Prop(t *testing.T) {
	tests := []struct {
		name       string
		query      string
		cursorPos  int
		wantType   string
		wantPrefix string
	}{
		{
			name:       "prop: with empty prefix",
			query:      "prop:",
			cursorPos:  5,
			wantType:   "prop",
			wantPrefix: "",
		},
		{
			name:       "prop:sta with partial prefix",
			query:      "prop:sta",
			cursorPos:  8,
			wantType:   "prop",
			wantPrefix: "sta",
		},
		{
			name:       "prop:status without operator",
			query:      "prop:status",
			cursorPos:  11,
			wantType:   "prop",
			wantPrefix: "status",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := GetAutocompleteContext(tt.query, tt.cursorPos)
			if ctx.Type != tt.wantType {
				t.Errorf("Expected type %q, got %q", tt.wantType, ctx.Type)
			}
			if ctx.Prefix != tt.wantPrefix {
				t.Errorf("Expected prefix %q, got %q", tt.wantPrefix, ctx.Prefix)
			}
		})
	}
}

func TestGetAutocompleteContext_PropValue(t *testing.T) {
	tests := []struct {
		name         string
		query        string
		cursorPos    int
		wantType     string
		wantPrefix   string
		wantProperty string
	}{
		{
			name:         "prop:status= with empty prefix",
			query:        "prop:status=",
			cursorPos:    12,
			wantType:     "propvalue",
			wantPrefix:   "",
			wantProperty: "status",
		},
		{
			name:         "prop:status=dr with partial prefix",
			query:        "prop:status=dr",
			cursorPos:    14,
			wantType:     "propvalue",
			wantPrefix:   "dr",
			wantProperty: "status",
		},
		{
			name:         "prop:status=draft with full prefix",
			query:        "prop:status=draft",
			cursorPos:    17,
			wantType:     "propvalue",
			wantPrefix:   "draft",
			wantProperty: "status",
		},
		{
			name:         "prop:priority>3 with greater than",
			query:        "prop:priority>3",
			cursorPos:    15,
			wantType:     "propvalue",
			wantPrefix:   "3",
			wantProperty: "priority",
		},
		{
			name:         "prop:priority>=4 with greater than or equal",
			query:        "prop:priority>=4",
			cursorPos:    16,
			wantType:     "propvalue",
			wantPrefix:   "4",
			wantProperty: "priority",
		},
		{
			name:         "prop:priority<5 with less than",
			query:        "prop:priority<5",
			cursorPos:    15,
			wantType:     "propvalue",
			wantPrefix:   "5",
			wantProperty: "priority",
		},
		{
			name:         "prop:status!=draft with not equal",
			query:        "prop:status!=draft",
			cursorPos:    18,
			wantType:     "propvalue",
			wantPrefix:   "draft",
			wantProperty: "status",
		},
		{
			name:         "prop:pattern=~.* with regex operator",
			query:        "prop:pattern=~.*",
			cursorPos:    16,
			wantType:     "propvalue",
			wantPrefix:   ".*",
			wantProperty: "pattern",
		},
		{
			name:         "cursor after operator, empty value",
			query:        "prop:status=",
			cursorPos:    12,
			wantType:     "propvalue",
			wantPrefix:   "",
			wantProperty: "status",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := GetAutocompleteContext(tt.query, tt.cursorPos)
			if ctx.Type != tt.wantType {
				t.Errorf("Expected type %q, got %q", tt.wantType, ctx.Type)
			}
			if ctx.Prefix != tt.wantPrefix {
				t.Errorf("Expected prefix %q, got %q", tt.wantPrefix, ctx.Prefix)
			}
			if ctx.PropertyName != tt.wantProperty {
				t.Errorf("Expected property name %q, got %q", tt.wantProperty, ctx.PropertyName)
			}
		})
	}
}

func TestGetAutocompleteContext_Status(t *testing.T) {
	tests := []struct {
		name       string
		query      string
		cursorPos  int
		wantType   string
		wantPrefix string
	}{
		{
			name:       "status: with empty prefix",
			query:      "status:",
			cursorPos:  7,
			wantType:   "status",
			wantPrefix: "",
		},
		{
			name:       "status:dr with partial prefix",
			query:      "status:dr",
			cursorPos:  9,
			wantType:   "status",
			wantPrefix: "dr",
		},
		{
			name:       "status:draft with full prefix",
			query:      "status:draft",
			cursorPos:  12,
			wantType:   "status",
			wantPrefix: "draft",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := GetAutocompleteContext(tt.query, tt.cursorPos)
			if ctx.Type != tt.wantType {
				t.Errorf("Expected type %q, got %q", tt.wantType, ctx.Type)
			}
			if ctx.Prefix != tt.wantPrefix {
				t.Errorf("Expected prefix %q, got %q", tt.wantPrefix, ctx.Prefix)
			}
		})
	}
}

func TestGetAutocompleteContext_CursorPositions(t *testing.T) {
	tests := []struct {
		name      string
		query     string
		cursorPos int
		wantType  string
	}{
		{
			name:      "cursor at start of query",
			query:     "type:requirement",
			cursorPos: 0,
			wantType:  "",
		},
		{
			name:      "cursor before colon",
			query:     "type:requirement",
			cursorPos: 4,
			wantType:  "",
		},
		{
			name:      "cursor on colon",
			query:     "type:requirement",
			cursorPos: 5,
			wantType:  "type",
		},
		{
			name:      "cursor at end",
			query:     "type:requirement",
			cursorPos: 16,
			wantType:  "type",
		},
		{
			name:      "cursor beyond query length",
			query:     "type:req",
			cursorPos: 100,
			wantType:  "type",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := GetAutocompleteContext(tt.query, tt.cursorPos)
			if ctx.Type != tt.wantType {
				t.Errorf("Expected type %q, got %q", tt.wantType, ctx.Type)
			}
		})
	}
}

func TestGetAutocompleteContext_MultipleTokens(t *testing.T) {
	tests := []struct {
		name         string
		query        string
		cursorPos    int
		wantType     string
		wantPrefix   string
		wantProperty string
	}{
		{
			name:       "cursor in second token",
			query:      "type:req status:dr",
			cursorPos:  18,
			wantType:   "status",
			wantPrefix: "dr",
		},
		{
			name:       "cursor in first token with second present",
			query:      "type:req status:draft",
			cursorPos:  8,
			wantType:   "type",
			wantPrefix: "req",
		},
		{
			name:         "cursor in prop value",
			query:        "type:req prop:status=draft status:verified",
			cursorPos:    26, // After "prop:status=draft"
			wantType:     "propvalue",
			wantPrefix:   "draft",
			wantProperty: "status",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := GetAutocompleteContext(tt.query, tt.cursorPos)
			if ctx.Type != tt.wantType {
				t.Errorf("Expected type %q, got %q", tt.wantType, ctx.Type)
			}
			if ctx.Prefix != tt.wantPrefix {
				t.Errorf("Expected prefix %q, got %q", tt.wantPrefix, ctx.Prefix)
			}
		})
	}
}

func TestGetAutocompleteContext_NoAutocomplete(t *testing.T) {
	tests := []struct {
		name      string
		query     string
		cursorPos int
	}{
		{
			name:      "plain text",
			query:     "hello world",
			cursorPos: 5,
		},
		{
			name:      "cursor in whitespace",
			query:     "type:req  ",
			cursorPos: 9,
		},
		{
			name:      "cursor before type:",
			query:     "hello type:req",
			cursorPos: 3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := GetAutocompleteContext(tt.query, tt.cursorPos)
			if ctx.Type != "" {
				t.Errorf("Expected no autocomplete (empty type), got %q", ctx.Type)
			}
		})
	}
}
