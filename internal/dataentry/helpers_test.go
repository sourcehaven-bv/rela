package dataentry

import (
	"html/template"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/model"
)

func TestApplyFilters(t *testing.T) {
	entities := []*model.Entity{
		{ID: "E-001", Type: "ticket", Properties: map[string]interface{}{"status": "open", "priority": "high"}},
		{ID: "E-002", Type: "ticket", Properties: map[string]interface{}{"status": "closed", "priority": "low"}},
		{ID: "E-003", Type: "ticket", Properties: map[string]interface{}{"status": "open", "priority": "low"}},
	}

	tests := []struct {
		name     string
		filters  []FilterConfig
		wantIDs  []string
	}{
		{
			name:    "no filters returns all",
			filters: nil,
			wantIDs: []string{"E-001", "E-002", "E-003"},
		},
		{
			name:    "equal filter",
			filters: []FilterConfig{{Property: "status", Operator: "=", Value: "open"}},
			wantIDs: []string{"E-001", "E-003"},
		},
		{
			name:    "not-equal filter",
			filters: []FilterConfig{{Property: "status", Operator: "!=", Value: "closed"}},
			wantIDs: []string{"E-001", "E-003"},
		},
		{
			name:    "multiple filters (AND)",
			filters: []FilterConfig{
				{Property: "status", Operator: "=", Value: "open"},
				{Property: "priority", Operator: "=", Value: "high"},
			},
			wantIDs: []string{"E-001"},
		},
		{
			name:    "variable substitution skipped",
			filters: []FilterConfig{{Property: "status", Operator: "=", Value: "$current_user"}},
			wantIDs: []string{"E-001", "E-002", "E-003"},
		},
		{
			name:    "nil property treated as empty string",
			filters: []FilterConfig{{Property: "missing", Operator: "=", Value: ""}},
			wantIDs: []string{"E-001", "E-002", "E-003"},
		},
		{
			name:    "nil property not equal to non-empty",
			filters: []FilterConfig{{Property: "missing", Operator: "=", Value: "something"}},
			wantIDs: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := applyFilters(entities, tt.filters)
			gotIDs := make([]string, len(got))
			for i, e := range got {
				gotIDs[i] = e.ID
			}
			if len(gotIDs) != len(tt.wantIDs) {
				t.Fatalf("got %v, want %v", gotIDs, tt.wantIDs)
			}
			for i, id := range gotIDs {
				if id != tt.wantIDs[i] {
					t.Errorf("got[%d] = %s, want %s", i, id, tt.wantIDs[i])
				}
			}
		})
	}
}

func TestSortEntities(t *testing.T) {
	makeEntities := func() []*model.Entity {
		return []*model.Entity{
			{ID: "E-003", Properties: map[string]interface{}{"name": "Charlie"}},
			{ID: "E-001", Properties: map[string]interface{}{"name": "Alice"}},
			{ID: "E-002", Properties: map[string]interface{}{"name": "Bob"}},
		}
	}

	t.Run("nil config does nothing", func(t *testing.T) {
		entities := makeEntities()
		sortEntities(entities, nil)
		if entities[0].ID != "E-003" {
			t.Errorf("expected no reorder, got %s first", entities[0].ID)
		}
	})

	t.Run("empty property does nothing", func(t *testing.T) {
		entities := makeEntities()
		sortEntities(entities, &SortConfig{Property: ""})
		if entities[0].ID != "E-003" {
			t.Errorf("expected no reorder, got %s first", entities[0].ID)
		}
	})

	t.Run("ascending sort", func(t *testing.T) {
		entities := makeEntities()
		sortEntities(entities, &SortConfig{Property: "name", Direction: "asc"})
		if entities[0].ID != "E-001" || entities[1].ID != "E-002" || entities[2].ID != "E-003" {
			t.Errorf("expected Alice, Bob, Charlie; got %s, %s, %s",
				entities[0].Properties["name"], entities[1].Properties["name"], entities[2].Properties["name"])
		}
	})

	t.Run("descending sort", func(t *testing.T) {
		entities := makeEntities()
		sortEntities(entities, &SortConfig{Property: "name", Direction: "desc"})
		if entities[0].ID != "E-003" || entities[1].ID != "E-002" || entities[2].ID != "E-001" {
			t.Errorf("expected Charlie, Bob, Alice; got %s, %s, %s",
				entities[0].Properties["name"], entities[1].Properties["name"], entities[2].Properties["name"])
		}
	})

	t.Run("nil property values sort as empty", func(t *testing.T) {
		entities := []*model.Entity{
			{ID: "E-001", Properties: map[string]interface{}{"name": "Bob"}},
			{ID: "E-002", Properties: map[string]interface{}{}},
			{ID: "E-003", Properties: map[string]interface{}{"name": "Alice"}},
		}
		sortEntities(entities, &SortConfig{Property: "name", Direction: "asc"})
		// empty string sorts before "Alice"
		if entities[0].ID != "E-002" {
			t.Errorf("expected nil property to sort first, got %s", entities[0].ID)
		}
	})
}

func TestResolvePropertyValues(t *testing.T) {
	meta := &metamodel.Metamodel{
		Types: map[string]metamodel.CustomType{
			"priority_type": {Values: []string{"low", "medium", "high"}},
		},
	}

	t.Run("inline values returned directly", func(t *testing.T) {
		prop := metamodel.PropertyDef{Values: []string{"a", "b", "c"}}
		got := resolvePropertyValues(prop, meta)
		if len(got) != 3 || got[0] != "a" {
			t.Errorf("expected inline values, got %v", got)
		}
	})

	t.Run("custom type values resolved", func(t *testing.T) {
		prop := metamodel.PropertyDef{Type: "priority_type"}
		got := resolvePropertyValues(prop, meta)
		if len(got) != 3 || got[0] != "low" {
			t.Errorf("expected custom type values, got %v", got)
		}
	})

	t.Run("unknown type returns nil", func(t *testing.T) {
		prop := metamodel.PropertyDef{Type: "string"}
		got := resolvePropertyValues(prop, meta)
		if got != nil {
			t.Errorf("expected nil, got %v", got)
		}
	})
}

func TestResolveWidget(t *testing.T) {
	meta := &metamodel.Metamodel{
		Types: map[string]metamodel.CustomType{
			"priority_type": {Values: []string{"low", "high"}},
		},
	}

	tests := []struct {
		name     string
		explicit string
		prop     metamodel.PropertyDef
		want     string
	}{
		{"explicit override", "textarea", metamodel.PropertyDef{Type: "string"}, "textarea"},
		{"string type", "", metamodel.PropertyDef{Type: "string"}, "text"},
		{"date type", "", metamodel.PropertyDef{Type: "date"}, "date"},
		{"integer type", "", metamodel.PropertyDef{Type: "integer"}, "number"},
		{"boolean type", "", metamodel.PropertyDef{Type: "boolean"}, "checkbox"},
		{"enum type", "", metamodel.PropertyDef{Type: "enum"}, "select"},
		{"custom type", "", metamodel.PropertyDef{Type: "priority_type"}, "select"},
		{"unknown type", "", metamodel.PropertyDef{Type: "something_else"}, "text"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := resolveWidget(tt.explicit, tt.prop, meta)
			if got != tt.want {
				t.Errorf("resolveWidget(%q, %v) = %q, want %q", tt.explicit, tt.prop.Type, got, tt.want)
			}
		})
	}
}

func TestWidgetToInputType(t *testing.T) {
	tests := []struct {
		widget string
		want   string
	}{
		{"textarea", "textarea"},
		{"select", "select"},
		{"multi-select", "select"},
		{"text", "text"},
		{"date", "date"},
		{"number", "number"},
		{"checkbox", "checkbox"},
	}

	for _, tt := range tests {
		t.Run(tt.widget, func(t *testing.T) {
			got := widgetToInputType(tt.widget)
			if got != tt.want {
				t.Errorf("widgetToInputType(%q) = %q, want %q", tt.widget, got, tt.want)
			}
		})
	}
}

func TestCoalesce(t *testing.T) {
	tests := []struct {
		name string
		vals []string
		want string
	}{
		{"first non-empty", []string{"", "b", "c"}, "b"},
		{"all empty", []string{"", "", ""}, ""},
		{"first is non-empty", []string{"a", "b"}, "a"},
		{"no args", nil, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := coalesce(tt.vals...)
			if got != tt.want {
				t.Errorf("coalesce(%v) = %q, want %q", tt.vals, got, tt.want)
			}
		})
	}
}

func TestSlugify(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"Hello World", "hello-world"},
		{"SLA Documents", "sla-documents"},
		{"already-slugged", "already-slugged"},
		{"  spaces  everywhere  ", "spaces-everywhere"},
		{"special!@#chars", "special-chars"},
		{"MiXeD CaSe 123", "mixed-case-123"},
		{"", ""},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := slugify(tt.input)
			if got != tt.want {
				t.Errorf("slugify(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestTitleCase(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"snake_case", "Snake Case"},
		{"kebab-case", "Kebab Case"},
		{"already Title", "Already Title"},
		{"single", "Single"},
		{"", ""},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := titleCase(tt.input)
			if got != tt.want {
				t.Errorf("titleCase(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestResolvePropertyType(t *testing.T) {
	meta := &metamodel.Metamodel{
		Entities: map[string]metamodel.EntityDef{
			"ticket": {
				Properties: map[string]metamodel.PropertyDef{
					"status":   {Type: "status_type"},
					"priority": {Type: "priority_type"},
				},
			},
		},
	}

	tests := []struct {
		name       string
		prop       string
		entityType string
		want       string
	}{
		{"known property", "status", "ticket", "status_type"},
		{"unknown property", "missing", "ticket", ""},
		{"unknown entity type", "status", "nonexistent", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := resolvePropertyType(tt.prop, tt.entityType, meta)
			if got != tt.want {
				t.Errorf("resolvePropertyType(%q, %q) = %q, want %q", tt.prop, tt.entityType, got, tt.want)
			}
		})
	}
}

func TestSimpleMarkdownToHTML(t *testing.T) {
	tests := []struct {
		name string
		md   string
		want string
	}{
		{"empty", "", ""},
		{"plain text", "Hello world", "<p>Hello world</p>"},
		{"h1", "# Title", "<h3>Title</h3>"},
		{"h2", "## Subtitle", "<h4>Subtitle</h4>"},
		{"h3", "### Section", "<h5>Section</h5>"},
		{"bold", "Some **bold** text", "<p>Some <strong>bold</strong> text</p>"},
		{"italic", "Some *italic* text", "<p>Some <em>italic</em> text</p>"},
		{"inline code", "Use `code` here", "<p>Use <code>code</code> here</p>"},
		{"unordered list", "- item one\n- item two", "<ul>\n<li>item one</li>\n<li>item two</li>\n</ul>"},
		{"ordered list", "1. first\n2. second", "<ol>\n<li>first</li>\n<li>second</li>\n</ol>"},
		{"code block", "```\nfoo\nbar\n```", "<pre><code>\nfoo\nbar\n</code></pre>"},
		{
			"unclosed code block",
			"```\ncode here",
			"<pre><code>\ncode here\n</code></pre>",
		},
		{
			"paragraph break",
			"First paragraph.\n\nSecond paragraph.",
			"<p>First paragraph.</p>\n<p>Second paragraph.</p>",
		},
		{
			"list followed by text",
			"- item\n\nText after list.",
			"<ul>\n<li>item</li>\n</ul>\n<p>Text after list.</p>",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := string(simpleMarkdownToHTML(tt.md))
			if got != tt.want {
				t.Errorf("simpleMarkdownToHTML(%q)\n  got:  %q\n  want: %q", tt.md, got, tt.want)
			}
		})
	}
}

func TestInlineFormat(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"bold", "**bold**", "<strong>bold</strong>"},
		{"italic", "*italic*", "<em>italic</em>"},
		{"code", "`code`", "<code>code</code>"},
		{"HTML escaping", "<script>alert('xss')</script>", "&lt;script&gt;alert(&#39;xss&#39;)&lt;/script&gt;"},
		{"no formatting", "plain text", "plain text"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := inlineFormat(tt.input)
			if got != tt.want {
				t.Errorf("inlineFormat(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestTemplateFuncs(t *testing.T) {
	styleMap := map[string]map[string]string{
		"status_type": {"open": "badge-green", "closed": "badge-red"},
	}
	styledTypes := map[string]bool{"status_type": true}

	funcs := templateFuncs(styleMap, styledTypes)

	t.Run("join", func(t *testing.T) {
		fn := funcs["join"].(func([]string, string) string)
		got := fn([]string{"a", "b", "c"}, ", ")
		if got != "a, b, c" {
			t.Errorf("join = %q, want %q", got, "a, b, c")
		}
	})

	t.Run("json", func(t *testing.T) {
		fn := funcs["json"].(func(interface{}) string)
		got := fn(map[string]string{"key": "value"})
		if got != `{"key":"value"}` {
			t.Errorf("json = %q", got)
		}
	})

	t.Run("contains true", func(t *testing.T) {
		fn := funcs["contains"].(func([]string, string) bool)
		if !fn([]string{"a", "b"}, "b") {
			t.Error("expected true")
		}
	})

	t.Run("contains false", func(t *testing.T) {
		fn := funcs["contains"].(func([]string, string) bool)
		if fn([]string{"a", "b"}, "c") {
			t.Error("expected false")
		}
	})

	t.Run("badgeClass known", func(t *testing.T) {
		fn := funcs["badgeClass"].(func(string, string) string)
		got := fn("status_type", "open")
		if got != "badge-green" {
			t.Errorf("badgeClass = %q, want badge-green", got)
		}
	})

	t.Run("badgeClass unknown falls back", func(t *testing.T) {
		fn := funcs["badgeClass"].(func(string, string) string)
		got := fn("unknown_type", "whatever")
		if got != "badge-gray" {
			t.Errorf("badgeClass = %q, want badge-gray", got)
		}
	})

	t.Run("isBadgeType true", func(t *testing.T) {
		fn := funcs["isBadgeType"].(func(string) bool)
		if !fn("status_type") {
			t.Error("expected true")
		}
	})

	t.Run("isBadgeType false", func(t *testing.T) {
		fn := funcs["isBadgeType"].(func(string) bool)
		if fn("unknown") {
			t.Error("expected false")
		}
	})

	t.Run("renderMarkdown", func(t *testing.T) {
		fn := funcs["renderMarkdown"].(func(string) template.HTML)
		got := fn("**bold**")
		if string(got) != "<p><strong>bold</strong></p>" {
			t.Errorf("renderMarkdown = %q", got)
		}
	})

	t.Run("formatValue RFC3339 date", func(t *testing.T) {
		fn := funcs["formatValue"].(func(string) string)
		got := fn("2024-01-15T10:30:00Z")
		if got != "2024-01-15" {
			t.Errorf("formatValue = %q, want 2024-01-15", got)
		}
	})

	t.Run("formatValue plain string", func(t *testing.T) {
		fn := funcs["formatValue"].(func(string) string)
		got := fn("just text")
		if got != "just text" {
			t.Errorf("formatValue = %q, want 'just text'", got)
		}
	})
}

func TestAppendToastParam(t *testing.T) {
	tests := []struct {
		name     string
		url      string
		message  string
		want     string
	}{
		{
			"simple path",
			"/list/tickets",
			"Created TKT-001",
			"/list/tickets?_toast=Created+TKT-001",
		},
		{
			"existing query params",
			"/list/tickets?sort=name",
			"Saved",
			"/list/tickets?sort=name&_toast=Saved",
		},
		{
			"with fragment",
			"/view/ticket/TKT-001#section",
			"Updated",
			"/view/ticket/TKT-001?_toast=Updated#section",
		},
		{
			"query and fragment",
			"/view/ticket/TKT-001?from=list#section",
			"Done",
			"/view/ticket/TKT-001?from=list&_toast=Done#section",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := appendToastParam(tt.url, tt.message)
			if got != tt.want {
				t.Errorf("appendToastParam(%q, %q) = %q, want %q", tt.url, tt.message, got, tt.want)
			}
		})
	}
}
