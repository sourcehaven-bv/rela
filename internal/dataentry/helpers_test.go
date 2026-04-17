package dataentry

import (
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"time"

	"golang.org/x/net/html"

	"github.com/Sourcehaven-BV/rela/internal/filter"
	"github.com/Sourcehaven-BV/rela/internal/graph"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/model"
	"github.com/Sourcehaven-BV/rela/internal/testutil"
)

// htmlHasElement checks if the HTML contains an element matching the given tag and optional attributes.
func htmlHasElement(htmlStr, tag string, attrs map[string]string) bool {
	doc, err := html.Parse(strings.NewReader(htmlStr))
	if err != nil {
		return false
	}
	return findElement(doc, tag, attrs) != nil
}

// htmlHasText checks if the HTML contains the given text content anywhere.
func htmlHasText(htmlStr, text string) bool {
	doc, err := html.Parse(strings.NewReader(htmlStr))
	if err != nil {
		return false
	}
	return findText(doc, text)
}

func findElement(n *html.Node, tag string, attrs map[string]string) *html.Node {
	if n.Type == html.ElementNode && n.Data == tag {
		if matchAttrs(n, attrs) {
			return n
		}
	}
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if found := findElement(c, tag, attrs); found != nil {
			return found
		}
	}
	return nil
}

func matchAttrs(n *html.Node, attrs map[string]string) bool {
	for key, val := range attrs {
		found := false
		for _, a := range n.Attr {
			if a.Key == key && (val == "" || a.Val == val) {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}

func findText(n *html.Node, text string) bool {
	if n.Type == html.TextNode && strings.Contains(n.Data, text) {
		return true
	}
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if findText(c, text) {
			return true
		}
	}
	return false
}

func TestPropertyContains(t *testing.T) {
	tests := []struct {
		name  string
		prop  interface{}
		value string
		want  bool
	}{
		{"nil property matches empty", nil, "", true},
		{"nil property does not match non-empty", nil, "foo", false},
		{"string exact match", "foo", "foo", true},
		{"string no match", "foo", "bar", false},
		{"[]string contains", []string{"foo", "bar"}, "bar", true},
		{"[]string does not contain", []string{"foo", "bar"}, "baz", false},
		{"[]interface{} contains", []interface{}{"foo", "bar"}, "foo", true},
		{"[]interface{} does not contain", []interface{}{"foo", "bar"}, "baz", false},
		{"empty []string does not match", []string{}, "foo", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := propertyContains(tt.prop, tt.value)
			if got != tt.want {
				t.Errorf("propertyContains(%v, %q) = %v, want %v", tt.prop, tt.value, got, tt.want)
			}
		})
	}
}

func TestPropertyIsEmpty(t *testing.T) {
	tests := []struct {
		name string
		prop interface{}
		want bool
	}{
		{"nil is empty", nil, true},
		{"empty string is empty", "", true},
		{"non-empty string is not empty", "foo", false},
		{"empty []string is empty", []string{}, true},
		{"non-empty []string is not empty", []string{"foo"}, false},
		{"empty []interface{} is empty", []interface{}{}, true},
		{"non-empty []interface{} is not empty", []interface{}{"foo"}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := propertyIsEmpty(tt.prop)
			if got != tt.want {
				t.Errorf("propertyIsEmpty(%v) = %v, want %v", tt.prop, got, tt.want)
			}
		})
	}
}

func TestApplyFilters(t *testing.T) {
	meta := testMeta()
	entities := []*model.Entity{
		testutil.EntityFor(meta, "ticket").ID("E-001").With("status", "open").With("priority", "high").Build(),
		testutil.EntityFor(meta, "ticket").ID("E-002").With("status", "closed").With("priority", "low").Build(),
		testutil.EntityFor(meta, "ticket").ID("E-003").With("status", "open").With("priority", "low").Build(),
	}

	tests := []struct {
		name    string
		filters []FilterConfig
		wantIDs []string
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
			name: "multiple filters (AND)",
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

func TestApplyFiltersMultiSelect(t *testing.T) {
	// Note: Using Entity() here because "clause" type is not in testMeta()
	// and the test is specifically testing multi-select property filtering logic
	entities := []*model.Entity{
		testutil.Entity("clause").ID("E-001").With("applies_to", "client").Build(),
		testutil.Entity("clause").ID("E-002").WithList("applies_to", "client", "provider").Build(),
		testutil.Entity("clause").ID("E-003").WithList("applies_to", "provider", "employee").Build(),
		testutil.Entity("clause").ID("E-004").With("applies_to", "employee").Build(),
		testutil.Entity("clause").ID("E-005").With("applies_to", []interface{}{"client", "provider"}).Build(), // from YAML
	}

	tests := []struct {
		name    string
		filters []FilterConfig
		wantIDs []string
	}{
		{
			name:    "= client matches single and list values",
			filters: []FilterConfig{{Property: "applies_to", Operator: "=", Value: "client"}},
			wantIDs: []string{"E-001", "E-002", "E-005"},
		},
		{
			name:    "= provider matches list values",
			filters: []FilterConfig{{Property: "applies_to", Operator: "=", Value: "provider"}},
			wantIDs: []string{"E-002", "E-003", "E-005"},
		},
		{
			name:    "= employee matches list and single",
			filters: []FilterConfig{{Property: "applies_to", Operator: "=", Value: "employee"}},
			wantIDs: []string{"E-003", "E-004"},
		},
		{
			name:    "!= client excludes all entries containing client",
			filters: []FilterConfig{{Property: "applies_to", Operator: "!=", Value: "client"}},
			wantIDs: []string{"E-003", "E-004"},
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

func TestSortEntitiesMulti(t *testing.T) {
	meta := &metamodel.Metamodel{
		Entities: map[string]metamodel.EntityDef{
			"item": {
				Properties: map[string]metamodel.PropertyDef{
					"name": {Type: "string"},
				},
			},
		},
	}

	makeEntities := func() []*model.Entity {
		// Note: Using Entity() here because "item" type is not in testMeta()
		// and the test is specifically testing sorting logic, not entity creation
		return []*model.Entity{
			testutil.Entity("item").ID("E-003").With("name", "Charlie").Build(),
			testutil.Entity("item").ID("E-001").With("name", "Alice").Build(),
			testutil.Entity("item").ID("E-002").With("name", "Bob").Build(),
		}
	}

	app := newAppFromParts(nil, meta, nil)

	t.Run("nil specs does nothing", func(t *testing.T) {
		entities := makeEntities()
		app.sortEntitiesMulti(entities, nil)
		if entities[0].ID != "E-003" {
			t.Errorf("expected no reorder, got %s first", entities[0].ID)
		}
	})

	t.Run("empty specs does nothing", func(t *testing.T) {
		entities := makeEntities()
		app.sortEntitiesMulti(entities, []filter.SortSpec{})
		if entities[0].ID != "E-003" {
			t.Errorf("expected no reorder, got %s first", entities[0].ID)
		}
	})

	t.Run("ascending sort", func(t *testing.T) {
		entities := makeEntities()
		app.sortEntitiesMulti(entities, []filter.SortSpec{{Property: "name", Direction: "asc"}})
		if entities[0].ID != "E-001" || entities[1].ID != "E-002" || entities[2].ID != "E-003" {
			t.Errorf("expected Alice, Bob, Charlie; got %s, %s, %s",
				entities[0].Properties["name"], entities[1].Properties["name"], entities[2].Properties["name"])
		}
	})

	t.Run("descending sort", func(t *testing.T) {
		entities := makeEntities()
		app.sortEntitiesMulti(entities, []filter.SortSpec{{Property: "name", Direction: "desc"}})
		if entities[0].ID != "E-003" || entities[1].ID != "E-002" || entities[2].ID != "E-001" {
			t.Errorf("expected Charlie, Bob, Alice; got %s, %s, %s",
				entities[0].Properties["name"], entities[1].Properties["name"], entities[2].Properties["name"])
		}
	})

	t.Run("nil property values sort to end", func(t *testing.T) {
		// Note: Using Entity() here because "item" type is not in testMeta()
		entities := []*model.Entity{
			testutil.Entity("item").ID("E-001").With("name", "Bob").Build(),
			testutil.Entity("item").ID("E-002").Build(),
			testutil.Entity("item").ID("E-003").With("name", "Alice").Build(),
		}
		app.sortEntitiesMulti(entities, []filter.SortSpec{{Property: "name", Direction: "asc"}})
		// With type-aware sorting, nil values sort to end
		if entities[0].ID != "E-003" {
			t.Errorf("expected Alice first, got %s", entities[0].ID)
		}
		if entities[1].ID != "E-001" {
			t.Errorf("expected Bob second, got %s", entities[1].ID)
		}
		if entities[2].ID != "E-002" {
			t.Errorf("expected nil property last, got %s", entities[2].ID)
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
		name string
		prop metamodel.PropertyDef
		want string
	}{
		{"string type", metamodel.PropertyDef{Type: metamodel.PropertyTypeString}, WidgetText},
		{"date type", metamodel.PropertyDef{Type: metamodel.PropertyTypeDate}, WidgetDate},
		{"integer type", metamodel.PropertyDef{Type: metamodel.PropertyTypeInteger}, WidgetNumber},
		{"boolean type", metamodel.PropertyDef{Type: metamodel.PropertyTypeBoolean}, WidgetCheckbox},
		{"enum type", metamodel.PropertyDef{Type: metamodel.PropertyTypeEnum}, WidgetSelect},
		{"custom type", metamodel.PropertyDef{Type: "priority_type"}, WidgetSelect},
		{"unknown type", metamodel.PropertyDef{Type: "something_else"}, WidgetText},
		{"list enum type", metamodel.PropertyDef{Type: metamodel.PropertyTypeEnum, List: true}, WidgetMultiSelect},
		{"list custom type", metamodel.PropertyDef{Type: "priority_type", List: true}, WidgetMultiSelect},
		{"list string type (not multi-select)", metamodel.PropertyDef{Type: metamodel.PropertyTypeString, List: true}, WidgetText},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := resolveWidget(tt.prop, meta)
			if got != tt.want {
				t.Errorf("resolveWidget(%v) = %q, want %q", tt.prop.Type, got, tt.want)
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

func TestContainsString(t *testing.T) {
	tests := []struct {
		name  string
		slice []string
		s     string
		want  bool
	}{
		{"found", []string{"a", "b", "c"}, "b", true},
		{"not found", []string{"a", "b", "c"}, "d", false},
		{"empty slice", []string{}, "a", false},
		{"nil slice", nil, "a", false},
		{"empty string found", []string{"", "b"}, "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := containsString(tt.slice, tt.s)
			if got != tt.want {
				t.Errorf("containsString(%v, %q) = %v, want %v", tt.slice, tt.s, got, tt.want)
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
		name     string
		md       string
		elements []struct {
			tag   string
			attrs map[string]string
		}
		texts []string
	}{
		{
			name: "empty",
			md:   "",
		},
		{
			name:  "plain text",
			md:    "Hello world",
			texts: []string{"Hello world"},
			elements: []struct {
				tag   string
				attrs map[string]string
			}{{"p", nil}},
		},
		{
			name:  "headings",
			md:    "# H1\n## H2\n### H3",
			texts: []string{"H1", "H2", "H3"},
			elements: []struct {
				tag   string
				attrs map[string]string
			}{{"h1", nil}, {"h2", nil}, {"h3", nil}},
		},
		{
			name:  "bold and italic",
			md:    "Some **bold** and *italic* text",
			texts: []string{"bold", "italic"},
			elements: []struct {
				tag   string
				attrs map[string]string
			}{{"strong", nil}, {"em", nil}},
		},
		{
			name:  "inline code",
			md:    "Use `code` here",
			texts: []string{"code"},
			elements: []struct {
				tag   string
				attrs map[string]string
			}{{"code", nil}},
		},
		{
			name:  "unordered list",
			md:    "- item one\n- item two",
			texts: []string{"item one", "item two"},
			elements: []struct {
				tag   string
				attrs map[string]string
			}{{"ul", nil}, {"li", nil}},
		},
		{
			name:  "ordered list",
			md:    "1. first\n2. second",
			texts: []string{"first", "second"},
			elements: []struct {
				tag   string
				attrs map[string]string
			}{{"ol", nil}, {"li", nil}},
		},
		{
			name:  "code block",
			md:    "```\nfoo\nbar\n```",
			texts: []string{"foo", "bar"},
			elements: []struct {
				tag   string
				attrs map[string]string
			}{{"pre", nil}, {"code", nil}},
		},
		{
			name:  "mermaid block",
			md:    "```mermaid\ngraph TD\n```",
			texts: []string{"graph TD"},
			elements: []struct {
				tag   string
				attrs map[string]string
			}{{"pre", map[string]string{"class": "mermaid"}}},
		},
		{
			name:  "checkbox unchecked",
			md:    "- [ ] task one",
			texts: []string{"task one"},
			elements: []struct {
				tag   string
				attrs map[string]string
			}{{"input", map[string]string{"type": "checkbox", "data-cb-idx": ""}}},
		},
		{
			name: "checkbox checked",
			md:   "- [x] done task",
			elements: []struct {
				tag   string
				attrs map[string]string
			}{{"input", map[string]string{"type": "checkbox", "checked": ""}}},
		},
		{
			name: "multiple checkboxes have indices",
			md:   "- [ ] first\n- [x] second\n- [ ] third",
			elements: []struct {
				tag   string
				attrs map[string]string
			}{
				{"input", map[string]string{"data-cb-idx": "0"}},
				{"input", map[string]string{"data-cb-idx": "1"}},
				{"input", map[string]string{"data-cb-idx": "2"}},
			},
		},
		{
			name:  "table",
			md:    "| Name | Age |\n|------|-----|\n| Alice | 30 |",
			texts: []string{"Name", "Age", "Alice", "30"},
			elements: []struct {
				tag   string
				attrs map[string]string
			}{
				{"table", map[string]string{"class": "md-table"}},
				{"thead", nil},
				{"tbody", nil},
				{"th", nil},
				{"td", nil},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := string(simpleMarkdownToHTML(tt.md))

			for _, elem := range tt.elements {
				if !htmlHasElement(got, elem.tag, elem.attrs) {
					t.Errorf("missing element <%s %v> in:\n%s", elem.tag, elem.attrs, got)
				}
			}

			for _, text := range tt.texts {
				if !htmlHasText(got, text) {
					t.Errorf("missing text %q in:\n%s", text, got)
				}
			}
		})
	}
}

func TestToggleCheckbox(t *testing.T) {
	tests := []struct {
		name    string
		content string
		index   int
		want    string
		wantErr bool
	}{
		{
			"check unchecked",
			"- [ ] task one",
			0,
			"- [x] task one",
			false,
		},
		{
			"uncheck checked",
			"- [x] task one",
			0,
			"- [ ] task one",
			false,
		},
		{
			"uncheck uppercase",
			"- [X] task one",
			0,
			"- [ ] task one",
			false,
		},
		{
			"toggle second of three",
			"- [ ] first\n- [ ] second\n- [x] third",
			1,
			"- [ ] first\n- [x] second\n- [x] third",
			false,
		},
		{
			"index out of range",
			"- [ ] only one",
			1,
			"",
			true,
		},
		{
			"no checkboxes",
			"just text",
			0,
			"",
			true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := toggleCheckbox(tt.content, tt.index)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Errorf("toggleCheckbox(%q, %d)\n  got:  %q\n  want: %q", tt.content, tt.index, got, tt.want)
			}
		})
	}
}

func TestCheckboxStats(t *testing.T) {
	tests := []struct {
		name    string
		content string
		checked int
		total   int
	}{
		{"empty", "", 0, 0},
		{"no checkboxes", "just text\n- list item", 0, 0},
		{"one unchecked", "- [ ] task", 0, 1},
		{"one checked", "- [x] task", 1, 1},
		{"mixed", "- [ ] first\n- [x] second\n- [ ] third", 1, 3},
		{"all checked", "- [x] a\n- [X] b", 2, 2},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stats := checkboxStats(tt.content)
			if stats.Checked != tt.checked || stats.Total != tt.total {
				t.Errorf("checkboxStats(%q) = {Checked:%d, Total:%d}, want {Checked:%d, Total:%d}",
					tt.content, stats.Checked, stats.Total, tt.checked, tt.total)
			}
		})
	}
}

func TestResolveRelationColumnValue(t *testing.T) {
	meta := &metamodel.Metamodel{
		Entities: map[string]metamodel.EntityDef{
			"assessment": {
				Properties: map[string]metamodel.PropertyDef{
					"title": {Type: "string"},
				},
			},
			"person": {
				Properties: map[string]metamodel.PropertyDef{
					"name": {Type: "string", Required: true},
				},
			},
		},
	}

	g := graph.New()
	assessment := testutil.EntityFor(meta, "assessment").ID("ASS-001").With("title", "Q1 Review").Build()
	person1 := testutil.EntityFor(meta, "person").ID("PER-001").With("name", "Alice").Build()
	person2 := testutil.EntityFor(meta, "person").ID("PER-002").With("name", "Bob").Build()
	g.AddNode(assessment)
	g.AddNode(person1)
	g.AddNode(person2)

	g.AddEdge(testutil.NewRelation(assessment.ID, "assessmentBy", person1.ID).Build())
	g.AddEdge(testutil.NewRelation(assessment.ID, "assessmentBy", person2.ID).Build())
	g.AddEdge(testutil.NewRelation(assessment.ID, "otherRel", person1.ID).Build())

	app := newAppFromParts(nil, meta, g)

	t.Run("resolves multiple targets", func(t *testing.T) {
		got := app.resolveRelationColumnValues(assessment.ID, "assessmentBy", "")
		want := []string{"Alice", "Bob"}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("got %v, want %v", got, want)
		}
	})

	t.Run("filters by relation type", func(t *testing.T) {
		got := app.resolveRelationColumnValues(assessment.ID, "otherRel", "")
		want := []string{"Alice"}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("got %v, want %v", got, want)
		}
	})

	t.Run("returns empty for no matching relations", func(t *testing.T) {
		got := app.resolveRelationColumnValues(assessment.ID, "nonexistent", "")
		if len(got) != 0 {
			t.Errorf("got %v, want empty slice", got)
		}
	})

	t.Run("returns empty for unknown entity", func(t *testing.T) {
		got := app.resolveRelationColumnValues("UNKNOWN", "assessmentBy", "")
		if len(got) != 0 {
			t.Errorf("got %v, want empty slice", got)
		}
	})

	t.Run("direction outgoing explicit", func(t *testing.T) {
		got := app.resolveRelationColumnValues(assessment.ID, "assessmentBy", "outgoing")
		want := []string{"Alice", "Bob"}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("got %v, want %v", got, want)
		}
	})

	t.Run("direction incoming returns sources", func(t *testing.T) {
		// PER-001 has an incoming edge from ASS-001 via assessmentBy
		// Assessment title is not required, so falls back to ID
		got := app.resolveRelationColumnValues(person1.ID, "assessmentBy", "incoming")
		want := []string{assessment.ID}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("got %v, want %v", got, want)
		}
	})

	t.Run("direction incoming returns multiple sources", func(t *testing.T) {
		// PER-001 is target of both assessmentBy and otherRel from ASS-001
		got := app.resolveRelationColumnValues(person1.ID, "otherRel", "incoming")
		want := []string{assessment.ID}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("got %v, want %v", got, want)
		}
	})

	t.Run("direction incoming no matches", func(t *testing.T) {
		got := app.resolveRelationColumnValues(assessment.ID, "assessmentBy", "incoming")
		if len(got) != 0 {
			t.Errorf("got %v, want empty slice", got)
		}
	})
}

func TestIsRelationLinked(t *testing.T) {
	meta := &metamodel.Metamodel{
		Relations: map[string]metamodel.RelationDef{
			"assessedBy": {
				Label:   "assessed by",
				From:    []string{"annex_a_control"},
				To:      []string{"iso_control_assessment"},
				Inverse: &metamodel.InverseDef{ID: "assesses"},
			},
			"depends_on": {
				Label: "depends on",
				From:  []string{"ticket"},
				To:    []string{"ticket"},
			},
		},
	}
	app := newAppFromParts(nil, meta, nil)

	tests := []struct {
		name     string
		formRel  string
		linkRel  string
		expected bool
	}{
		{
			name:     "direct match",
			formRel:  "depends_on",
			linkRel:  "depends_on",
			expected: true,
		},
		{
			name:     "inverse of link relation matches form relation",
			formRel:  "assesses",
			linkRel:  "assessedBy",
			expected: true,
		},
		{
			name:     "no match",
			formRel:  "assesses",
			linkRel:  "depends_on",
			expected: false,
		},
		{
			name:     "unknown relations",
			formRel:  "unknown_a",
			linkRel:  "unknown_b",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := app.isRelationLinked(tt.formRel, tt.linkRel)
			if got != tt.expected {
				t.Errorf("isRelationLinked(%q, %q) = %v, want %v",
					tt.formRel, tt.linkRel, got, tt.expected)
			}
		})
	}
}

func TestFilterByRelation(t *testing.T) {
	meta := &metamodel.Metamodel{
		Entities: map[string]metamodel.EntityDef{
			"ticket": {
				Properties: map[string]metamodel.PropertyDef{
					"title": {Type: "string", Required: true},
				},
			},
			"component": {
				Properties: map[string]metamodel.PropertyDef{
					"name": {Type: "string", Required: true},
				},
			},
		},
		Relations: map[string]metamodel.RelationDef{
			"belongs_to": {
				Label: "belongs to",
				From:  []string{"ticket"},
				To:    []string{"component"},
			},
		},
	}

	g := graph.New()

	// Create components
	cmpFrontend := testutil.EntityFor(meta, "component").ID("CMP-001").With("name", "Frontend").Build()
	cmpBackend := testutil.EntityFor(meta, "component").ID("CMP-002").With("name", "Backend").Build()
	g.AddNode(cmpFrontend)
	g.AddNode(cmpBackend)

	// Create tickets with relations to components
	tkt1 := testutil.EntityFor(meta, "ticket").ID("TKT-001").With("title", "Frontend bug").Build()
	g.AddNode(tkt1)
	g.AddEdge(testutil.NewRelation(tkt1.ID, "belongs_to", cmpFrontend.ID).Build())

	tkt2 := testutil.EntityFor(meta, "ticket").ID("TKT-002").With("title", "Backend bug").Build()
	g.AddNode(tkt2)
	g.AddEdge(testutil.NewRelation(tkt2.ID, "belongs_to", cmpBackend.ID).Build())

	tkt3 := testutil.EntityFor(meta, "ticket").ID("TKT-003").With("title", "Another frontend bug").Build()
	g.AddNode(tkt3)
	g.AddEdge(testutil.NewRelation(tkt3.ID, "belongs_to", cmpFrontend.ID).Build())

	tkt4 := testutil.EntityFor(meta, "ticket").ID("TKT-004").With("title", "No component ticket").Build()
	g.AddNode(tkt4)
	// No relation for TKT-004

	app := newAppFromParts(nil, meta, g)
	allTickets := g.NodesByType("ticket")

	t.Run("filters by relation target title", func(t *testing.T) {
		got := app.filterByRelation(allTickets, "belongs_to", "Frontend")
		gotIDs := collectIDs(got)
		if len(got) != 2 {
			t.Fatalf("expected 2 results, got %d: %v", len(got), gotIDs)
		}
		if !sliceContainsStr(gotIDs, tkt1.ID) || !sliceContainsStr(gotIDs, tkt3.ID) {
			t.Errorf("expected %s and %s, got %v", tkt1.ID, tkt3.ID, gotIDs)
		}
	})

	t.Run("filters by different relation target", func(t *testing.T) {
		got := app.filterByRelation(allTickets, "belongs_to", "Backend")
		gotIDs := collectIDs(got)
		if len(got) != 1 {
			t.Fatalf("expected 1 result, got %d: %v", len(got), gotIDs)
		}
		if gotIDs[0] != tkt2.ID {
			t.Errorf("expected %s, got %v", tkt2.ID, gotIDs)
		}
	})

	t.Run("returns empty for non-matching value", func(t *testing.T) {
		got := app.filterByRelation(allTickets, "belongs_to", "Nonexistent")
		if len(got) != 0 {
			t.Errorf("expected 0 results, got %d", len(got))
		}
	})

	t.Run("returns empty for unknown relation type", func(t *testing.T) {
		got := app.filterByRelation(allTickets, "unknown_relation", "Frontend")
		if len(got) != 0 {
			t.Errorf("expected 0 results, got %d", len(got))
		}
	})

	t.Run("handles entities without relations", func(t *testing.T) {
		got := app.filterByRelation(allTickets, "belongs_to", "Frontend")
		for _, e := range got {
			if e.ID == tkt4.ID {
				t.Errorf("%s should not be in results (has no belongs_to relation)", tkt4.ID)
			}
		}
	})
}

func TestResolveRelationFilterValues(t *testing.T) {
	meta := &metamodel.Metamodel{
		Entities: map[string]metamodel.EntityDef{
			"ticket": {
				Properties: map[string]metamodel.PropertyDef{
					"title": {Type: "string", Required: true},
				},
			},
			"component": {
				Properties: map[string]metamodel.PropertyDef{
					"name": {Type: "string", Required: true},
				},
			},
		},
		Relations: map[string]metamodel.RelationDef{
			"belongs_to": {
				Label: "belongs to",
				From:  []string{"ticket"},
				To:    []string{"component"},
			},
		},
	}

	g := graph.New()

	// Create components
	cmpFrontend := testutil.EntityFor(meta, "component").ID("CMP-001").With("name", "Frontend").Build()
	cmpBackend := testutil.EntityFor(meta, "component").ID("CMP-002").With("name", "Backend").Build()
	cmpAPI := testutil.EntityFor(meta, "component").ID("CMP-003").With("name", "API").Build()
	g.AddNode(cmpFrontend)
	g.AddNode(cmpBackend)
	g.AddNode(cmpAPI)

	// Create tickets with relations
	tkt1 := testutil.EntityFor(meta, "ticket").ID("TKT-001").With("title", "Ticket 1").Build()
	g.AddNode(tkt1)
	g.AddEdge(testutil.NewRelation(tkt1.ID, "belongs_to", cmpFrontend.ID).Build())

	tkt2 := testutil.EntityFor(meta, "ticket").ID("TKT-002").With("title", "Ticket 2").Build()
	g.AddNode(tkt2)
	g.AddEdge(testutil.NewRelation(tkt2.ID, "belongs_to", cmpBackend.ID).Build())

	tkt3 := testutil.EntityFor(meta, "ticket").ID("TKT-003").With("title", "Ticket 3").Build()
	g.AddNode(tkt3)
	g.AddEdge(testutil.NewRelation(tkt3.ID, "belongs_to", cmpFrontend.ID).Build()) // duplicate Frontend

	// TKT-004 has no relation
	tkt4 := testutil.EntityFor(meta, "ticket").ID("TKT-004").With("title", "Ticket 4").Build()
	g.AddNode(tkt4)

	app := newAppFromParts(nil, meta, g)
	allTickets := g.NodesByType("ticket")

	t.Run("returns unique sorted values", func(t *testing.T) {
		got := app.resolveRelationFilterValues(allTickets, "belongs_to")
		// Only Frontend and Backend are referenced, API is not
		// Should be sorted: Backend, Frontend
		if len(got) != 2 {
			t.Fatalf("expected 2 values, got %d: %v", len(got), got)
		}
		if got[0] != "Backend" {
			t.Errorf("expected first value 'Backend', got %q", got[0])
		}
		if got[1] != "Frontend" {
			t.Errorf("expected second value 'Frontend', got %q", got[1])
		}
	})

	t.Run("returns empty for unknown relation type", func(t *testing.T) {
		got := app.resolveRelationFilterValues(allTickets, "unknown_relation")
		if len(got) != 0 {
			t.Errorf("expected 0 values, got %d", len(got))
		}
	})

	t.Run("returns empty for empty entities list", func(t *testing.T) {
		got := app.resolveRelationFilterValues([]*model.Entity{}, "belongs_to")
		if len(got) != 0 {
			t.Errorf("expected 0 values, got %d", len(got))
		}
	})
}

func sliceContainsStr(slice []string, s string) bool {
	for _, v := range slice {
		if v == s {
			return true
		}
	}
	return false
}

func TestResolveScope(t *testing.T) {
	meta := &metamodel.Metamodel{
		Entities: map[string]metamodel.EntityDef{
			"ticket": {
				Properties: map[string]metamodel.PropertyDef{
					"status":   {Type: "string"},
					"priority": {Type: "string"},
				},
			},
		},
	}

	makeGraph := func() *graph.Graph {
		g := graph.New()
		g.AddNode(testutil.EntityFor(meta, "ticket").ID("T-001").With("status", "open").With("priority", "high").Build())
		g.AddNode(testutil.EntityFor(meta, "ticket").ID("T-002").With("status", "closed").With("priority", "low").Build())
		g.AddNode(testutil.EntityFor(meta, "ticket").ID("T-003").With("status", "open").With("priority", "medium").Build())
		g.AddNode(testutil.EntityFor(meta, "ticket").ID("T-004").With("status", "open").With("priority", "low").Build())
		return g
	}

	makeApp := func() *App {
		return newAppFromParts(&Config{
			Lists: map[string]List{
				"tickets": {
					EntityType: "ticket",
					Title:      "Tickets",
					Sort:       []SortSpec{{Property: "priority", Direction: "asc"}},
					Filters:    nil,
					FilterControls: []FilterControl{
						{Property: "status"},
					},
				},
			},
		}, meta, makeGraph())
	}

	makeRequest := func(urlStr string) *http.Request {
		return httptest.NewRequest(http.MethodGet, urlStr, http.NoBody)
	}

	t.Run("no scope param returns nil", func(t *testing.T) {
		app := makeApp()
		r := makeRequest("/entity/ticket/T-002")
		got := app.resolveScope("T-002", r)
		if got != nil {
			t.Errorf("expected nil, got %+v", got)
		}
	})

	t.Run("empty scope param returns nil", func(t *testing.T) {
		app := makeApp()
		r := makeRequest("/entity/ticket/T-002?scope=")
		got := app.resolveScope("T-002", r)
		if got != nil {
			t.Errorf("expected nil, got %+v", got)
		}
	})

	t.Run("invalid scope prefix returns nil", func(t *testing.T) {
		app := makeApp()
		r := makeRequest("/entity/ticket/T-002?scope=bogus:foo")
		got := app.resolveScope("T-002", r)
		if got != nil {
			t.Errorf("expected nil, got %+v", got)
		}
	})

	t.Run("unknown list returns nil", func(t *testing.T) {
		app := makeApp()
		r := makeRequest("/entity/ticket/T-002?scope=list:nonexistent")
		got := app.resolveScope("T-002", r)
		if got != nil {
			t.Errorf("expected nil, got %+v", got)
		}
	})

	t.Run("entity not in scope returns nil", func(t *testing.T) {
		app := makeApp()
		r := makeRequest("/entity/ticket/T-999?scope=list:tickets")
		got := app.resolveScope("T-999", r)
		if got != nil {
			t.Errorf("expected nil, got %+v", got)
		}
	})

	t.Run("list scope middle item has prev and next", func(t *testing.T) {
		app := makeApp()
		// Sort by status asc: closed(T-002), open(T-001), open(T-003), open(T-004)
		// T-001 has status=open which sorts after closed.
		r := makeRequest("/entity/ticket/T-001?scope=list:tickets&sort=status&sort_dir=asc")
		got := app.resolveScope("T-001", r)
		if got == nil {
			t.Fatal("expected non-nil scope")
		}
		if got.PrevURL == "" {
			t.Error("expected PrevURL to be set for non-first item")
		}
		if got.Label != "4 Tickets" {
			t.Errorf("Label = %q, want %q", got.Label, "4 Tickets")
		}
	})

	t.Run("list scope first item has no prev", func(t *testing.T) {
		app := makeApp()
		r := makeRequest("/entity/ticket/T-001?scope=list:tickets&sort=status&sort_dir=asc")
		got := app.resolveScope("T-001", r)
		if got == nil {
			t.Fatal("expected non-nil scope")
		}
		if got.Progress == "[1/4]" && got.PrevURL != "" {
			t.Error("first item should not have PrevURL")
		}
	})

	t.Run("list scope last item has no next", func(t *testing.T) {
		app := makeApp()
		r := makeRequest("/entity/ticket/T-001?scope=list:tickets&sort=priority&sort_dir=desc")
		got := app.resolveScope("T-001", r)
		if got == nil {
			t.Fatal("expected non-nil scope")
		}
		if got.Progress == "[4/4]" && got.NextURL != "" {
			t.Error("last item should not have NextURL")
		}
	})

	t.Run("list scope with filter narrows results", func(t *testing.T) {
		app := makeApp()
		// Filter to status=open should give T-001, T-003, T-004 (3 items)
		r := makeRequest("/entity/ticket/T-003?scope=list:tickets&filter_status=open")
		got := app.resolveScope("T-003", r)
		if got == nil {
			t.Fatal("expected non-nil scope")
		}
		if got.Label != "3 Tickets" {
			t.Errorf("Label = %q, want %q", got.Label, "3 Tickets")
		}
	})

	t.Run("list scope preserves query params in prev/next URLs", func(t *testing.T) {
		app := makeApp()
		r := makeRequest("/entity/ticket/T-003?from=tickets&scope=list:tickets&filter_status=open")
		got := app.resolveScope("T-003", r)
		if got == nil {
			t.Fatal("expected non-nil scope")
		}
		checkURL := got.NextURL
		if checkURL == "" {
			checkURL = got.PrevURL
		}
		if checkURL == "" {
			t.Fatal("expected at least one nav URL")
		}
		for _, param := range []string{"scope=list%3Atickets", "filter_status=open", "from=tickets"} {
			if !strings.Contains(checkURL, param) {
				t.Errorf("URL %q missing expected param %q", checkURL, param)
			}
		}
	})

	t.Run("single item scope has no prev or next", func(t *testing.T) {
		app := makeApp()
		// Filter to status=closed should give only T-002
		r := makeRequest("/entity/ticket/T-002?scope=list:tickets&filter_status=closed")
		got := app.resolveScope("T-002", r)
		if got == nil {
			t.Fatal("expected non-nil scope")
		}
		if got.PrevURL != "" {
			t.Errorf("single item should have empty PrevURL, got %q", got.PrevURL)
		}
		if got.NextURL != "" {
			t.Errorf("single item should have empty NextURL, got %q", got.NextURL)
		}
		if got.Progress != "[1/1]" {
			t.Errorf("Progress = %q, want [1/1]", got.Progress)
		}
	})

	t.Run("search scope finds entity", func(t *testing.T) {
		app := makeApp()
		r := makeRequest("/entity/ticket/T-002?scope=search:type:ticket")
		got := app.resolveScope("T-002", r)
		if got == nil {
			t.Fatal("expected non-nil scope for search")
		}
		if got.Label == "" {
			t.Error("expected non-empty label for search scope")
		}
	})

	t.Run("search scope with no results returns nil", func(t *testing.T) {
		app := makeApp()
		r := makeRequest("/entity/ticket/T-002?scope=search:type:nonexistent")
		got := app.resolveScope("T-002", r)
		if got != nil {
			t.Errorf("expected nil for search with no results, got %+v", got)
		}
	})

	t.Run("list scope with relation filter narrows results", func(t *testing.T) {
		// Create an app with relation-based filter
		relMeta := &metamodel.Metamodel{
			Entities: map[string]metamodel.EntityDef{
				"ticket": {
					Properties: map[string]metamodel.PropertyDef{
						"title": {Type: "string", Required: true},
					},
				},
				"component": {
					Properties: map[string]metamodel.PropertyDef{
						"name": {Type: "string", Required: true},
					},
				},
			},
			Relations: map[string]metamodel.RelationDef{
				"belongs_to": {From: []string{"ticket"}, To: []string{"component"}},
			},
		}

		relGraph := graph.New()
		cmp := testutil.EntityFor(relMeta, "component").ID("CMP-001").With("name", "Frontend").Build()
		relGraph.AddNode(cmp)

		t1 := testutil.EntityFor(relMeta, "ticket").ID("T-001").With("title", "Ticket 1").Build()
		t2 := testutil.EntityFor(relMeta, "ticket").ID("T-002").With("title", "Ticket 2").Build()
		t3 := testutil.EntityFor(relMeta, "ticket").ID("T-003").With("title", "Ticket 3").Build()
		relGraph.AddNode(t1)
		relGraph.AddNode(t2)
		relGraph.AddNode(t3)

		// Only T-001 and T-002 belong to Frontend
		relGraph.AddEdge(testutil.NewRelation(t1.ID, "belongs_to", cmp.ID).Build())
		relGraph.AddEdge(testutil.NewRelation(t2.ID, "belongs_to", cmp.ID).Build())

		relApp := newAppFromParts(&Config{
			Lists: map[string]List{
				"tickets": {
					EntityType: "ticket",
					Title:      "Tickets",
					FilterControls: []FilterControl{
						{Relation: "belongs_to"},
					},
				},
			},
		}, relMeta, relGraph)

		r := makeRequest("/entity/ticket/" + t1.ID + "?scope=list:tickets&filter_belongs_to=Frontend")
		got := relApp.resolveScope(t1.ID, r)
		if got == nil {
			t.Fatal("expected non-nil scope")
		}
		// Filter to Frontend should give only T-001 and T-002 (2 tickets)
		if got.Label != "2 Tickets" {
			t.Errorf("Label = %q, want %q", got.Label, "2 Tickets")
		}
	})

	t.Run("view path scope replaces entity ID correctly", func(t *testing.T) {
		app := makeApp()
		r := makeRequest("/view/ticket-detail/T-002?scope=list:tickets&sort=priority&sort_dir=asc")
		got := app.resolveScope("T-002", r)
		if got == nil {
			t.Fatal("expected non-nil scope")
		}
		if got.PrevURL != "" && !strings.Contains(got.PrevURL, "/view/ticket-detail/") {
			t.Errorf("PrevURL should preserve view path prefix, got %q", got.PrevURL)
		}
		if got.NextURL != "" && !strings.Contains(got.NextURL, "/view/ticket-detail/") {
			t.Errorf("NextURL should preserve view path prefix, got %q", got.NextURL)
		}
	})
}

func TestResolveFilterVariable(t *testing.T) {
	// Pin the clock so date variables are deterministic.
	pinned := time.Date(2026, 4, 7, 12, 0, 0, 0, time.UTC)
	prev := nowFunc
	nowFunc = func() time.Time { return pinned }
	defer func() { nowFunc = prev }()

	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"plain string passes through", "plain value", "plain value"},
		{"date string passes through", "2026-04-07", "2026-04-07"},
		{"empty string passes through", "", ""},
		{"unknown $variable passes through", "$unknown", "$unknown"},
		{"$today resolves", "$today", "2026-04-07"},
		{"$tomorrow resolves", "$tomorrow", "2026-04-08"},
		{"$yesterday resolves", "$yesterday", "2026-04-06"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := resolveFilterVariable(tt.input)
			if got != tt.want {
				t.Errorf("resolveFilterVariable(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestResolveFilterVariablesInList(t *testing.T) {
	pinned := time.Date(2026, 4, 7, 12, 0, 0, 0, time.UTC)
	prev := nowFunc
	nowFunc = func() time.Time { return pinned }
	defer func() { nowFunc = prev }()

	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"single value", "$today", "2026-04-07"},
		{"multiple variables", "$yesterday,$today,$tomorrow", "2026-04-06,2026-04-07,2026-04-08"},
		{"mixed variables and literals", "$today,2026-12-31", "2026-04-07,2026-12-31"},
		{"trims whitespace", "$today, $tomorrow", "2026-04-07,2026-04-08"},
		{"plain list passes through", "open,closed,wip", "open,closed,wip"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := resolveFilterVariablesInList(tt.input)
			if got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestCompareValues_Date(t *testing.T) {
	earlier := "2026-01-01"
	threshold := "2026-04-07"
	later := "2026-12-31"

	tests := []struct {
		name            string
		left, right, op string
		want            bool
	}{
		{"lt earlier than threshold", earlier, threshold, "lt", true},
		{"lt equal", threshold, threshold, "lt", false},
		{"lt later", later, threshold, "lt", false},
		{"lte earlier", earlier, threshold, "lte", true},
		{"lte equal", threshold, threshold, "lte", true},
		{"lte later", later, threshold, "lte", false},
		{"gt later", later, threshold, "gt", true},
		{"gt equal", threshold, threshold, "gt", false},
		{"gte equal", threshold, threshold, "gte", true},
		{"gte later", later, threshold, "gte", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := compareValues(tt.left, tt.right, tt.op)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Errorf("compareValues(%q, %q, %q) = %v, want %v",
					tt.left, tt.right, tt.op, got, tt.want)
			}
		})
	}
}

func TestCompareValues_Numeric(t *testing.T) {
	tests := []struct {
		name            string
		left, right, op string
		want            bool
	}{
		{"int lt", "5", "10", "lt", true},
		{"int gt", "10", "5", "gt", true},
		{"float lt", "3.14", "3.15", "lt", true},
		{"int gte equal", "42", "42", "gte", true},
		{"int lte equal", "42", "42", "lte", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := compareValues(tt.left, tt.right, tt.op)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Errorf("compareValues(%q, %q, %q) = %v, want %v",
					tt.left, tt.right, tt.op, got, tt.want)
			}
		})
	}
}

func TestCompareValues_String(t *testing.T) {
	got, err := compareValues("apple", "banana", "lt")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !got {
		t.Error("'apple' < 'banana' should be true")
	}
}

// TestCompareValues_TypeMismatch verifies that mixing types returns an error
// instead of silently producing wrong answers via lexicographic fallback.
func TestCompareValues_TypeMismatch(t *testing.T) {
	tests := []struct {
		name        string
		left, right string
	}{
		{"date vs non-date right", "2026-04-07", "tomorrow"},
		{"date vs non-date left", "tomorrow", "2026-04-07"},
		{"date vs different format", "2026-04-07", "07/04/2026"},
		{"number vs non-number right", "42", "high"},
		{"number vs non-number left", "high", "42"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			match, err := compareValues(tt.left, tt.right, "lt")
			if err == nil {
				t.Errorf("expected error for compareValues(%q, %q, lt), got match=%v",
					tt.left, tt.right, match)
			}
			if match {
				t.Errorf("type mismatch should return match=false, got true")
			}
		})
	}
}

// TestCompareOrdered_UnknownOperator confirms unknown operators return false.
func TestCompareOrdered_UnknownOperator(t *testing.T) {
	if compareOrdered(1, 2, "bogus") {
		t.Error("unknown operator should return false")
	}
}
