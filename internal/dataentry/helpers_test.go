package dataentry

import (
	"context"
	"reflect"
	"strings"
	"testing"
	"time"

	"golang.org/x/net/html"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/filter"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/store"
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
		prop  any
		value string
		want  bool
	}{
		{"nil property matches empty", nil, "", true},
		{"nil property does not match non-empty", nil, "foo", false},
		{"string exact match", "foo", "foo", true},
		{"string no match", "foo", "bar", false},
		{"[]string contains", []string{"foo", "bar"}, "bar", true},
		{"[]string does not contain", []string{"foo", "bar"}, "baz", false},
		{"[]interface{} contains", []any{"foo", "bar"}, "foo", true},
		{"[]interface{} does not contain", []any{"foo", "bar"}, "baz", false},
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
		prop any
		want bool
	}{
		{"nil is empty", nil, true},
		{"empty string is empty", "", true},
		{"non-empty string is not empty", "foo", false},
		{"empty []string is empty", []string{}, true},
		{"non-empty []string is not empty", []string{"foo"}, false},
		{"empty []interface{} is empty", []any{}, true},
		{"non-empty []interface{} is not empty", []any{"foo"}, false},
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
	entities := []*entity.Entity{
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
	entities := []*entity.Entity{
		testutil.Entity("clause").ID("E-001").With("applies_to", "client").Build(),
		testutil.Entity("clause").ID("E-002").WithList("applies_to", "client", "provider").Build(),
		testutil.Entity("clause").ID("E-003").WithList("applies_to", "provider", "employee").Build(),
		testutil.Entity("clause").ID("E-004").With("applies_to", "employee").Build(),
		testutil.Entity("clause").ID("E-005").With("applies_to", []any{"client", "provider"}).Build(), // from YAML
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

	makeEntities := func() []*entity.Entity {
		// Note: Using Entity() here because "item" type is not in testMeta()
		// and the test is specifically testing sorting logic, not entity creation
		return []*entity.Entity{
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
		entities := []*entity.Entity{
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
		{"datetime type", metamodel.PropertyDef{Type: metamodel.PropertyTypeDatetime}, WidgetDatetime},
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

	g := newFixture()
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
		got := app.views.resolveRelationColumnValues(context.Background(), assessment.ID, "assessmentBy", "")
		want := []string{"Alice", "Bob"}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("got %v, want %v", got, want)
		}
	})

	t.Run("filters by relation type", func(t *testing.T) {
		got := app.views.resolveRelationColumnValues(context.Background(), assessment.ID, "otherRel", "")
		want := []string{"Alice"}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("got %v, want %v", got, want)
		}
	})

	t.Run("returns empty for no matching relations", func(t *testing.T) {
		got := app.views.resolveRelationColumnValues(context.Background(), assessment.ID, "nonexistent", "")
		if len(got) != 0 {
			t.Errorf("got %v, want empty slice", got)
		}
	})

	t.Run("returns empty for unknown entity", func(t *testing.T) {
		got := app.views.resolveRelationColumnValues(context.Background(), "UNKNOWN", "assessmentBy", "")
		if len(got) != 0 {
			t.Errorf("got %v, want empty slice", got)
		}
	})

	t.Run("direction outgoing explicit", func(t *testing.T) {
		got := app.views.resolveRelationColumnValues(context.Background(), assessment.ID, "assessmentBy", "outgoing")
		want := []string{"Alice", "Bob"}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("got %v, want %v", got, want)
		}
	})

	t.Run("direction incoming returns sources", func(t *testing.T) {
		// PER-001 has an incoming edge from ASS-001 via assessmentBy
		// Assessment title is not required, so falls back to ID
		got := app.views.resolveRelationColumnValues(context.Background(), person1.ID, "assessmentBy", "incoming")
		want := []string{assessment.ID}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("got %v, want %v", got, want)
		}
	})

	t.Run("direction incoming returns multiple sources", func(t *testing.T) {
		// PER-001 is target of both assessmentBy and otherRel from ASS-001
		got := app.views.resolveRelationColumnValues(context.Background(), person1.ID, "otherRel", "incoming")
		want := []string{assessment.ID}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("got %v, want %v", got, want)
		}
	})

	t.Run("direction incoming no matches", func(t *testing.T) {
		got := app.views.resolveRelationColumnValues(context.Background(), assessment.ID, "assessmentBy", "incoming")
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

// pushdownTestMeta declares `status` as a string (push-eligible) and `count`
// as an integer (never eligible — see stringComparableOnEveryType).
func pushdownTestMeta() *metamodel.Metamodel {
	return &metamodel.Metamodel{
		Entities: map[string]metamodel.EntityDef{
			"ticket": {Properties: map[string]metamodel.PropertyDef{
				"status": {Type: metamodel.PropertyTypeString},
				"title":  {Type: metamodel.PropertyTypeString},
				"due":    {Type: metamodel.PropertyTypeDate},
				"count":  {Type: metamodel.PropertyTypeInteger},
			}},
		},
	}
}

// TestPushdownPrefilters pins which property filters may be handed to the
// store as a PRE-FILTER.
//
// The pushdown is not a replacement for the Go pass — executeQuery still runs
// every filter through the metamodel-aware filter.MatchAll, and the store can
// only ever remove rows that pass would also remove. That belt-and-braces is
// what makes the result provably identical to the pre-pushdown behavior,
// because store.PropPredicate compares by STRING FORM and disagrees with the
// typed comparison on integers, booleans and undeclared enum values.
func TestPushdownPrefilters(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		in         *filter.Filter
		wantPushed bool
	}{
		{
			name:       "equality pushes down",
			in:         &filter.Filter{Property: "status", Operator: filter.OpEqual, Value: "open"},
			wantPushed: true,
		},
		{
			name:       "is-empty pushes down",
			in:         &filter.Filter{Property: "status", Operator: filter.OpEqual},
			wantPushed: true,
		},
		{
			// Excluded on purpose. It is the one operator where a string-form
			// disagreement WIDENS: `flag!=yes` on a boolean errors in Go
			// (excluding the row) but matches in the store, so as a pre-filter
			// it admits rows the Go pass must then reject — no saving, and a
			// bug in the pairing would leak them.
			name:       "not-equal is excluded even though the store supports it",
			in:         &filter.Filter{Property: "status", Operator: filter.OpNotEqual, Value: "done"},
			wantPushed: false,
		},
		{
			// `status=in-*` rides on OpEqual but means pattern-match; pushed
			// down it would compare against the literal string "in-*".
			name:       "glob stays in Go despite riding on OpEqual",
			in:         &filter.Filter{Property: "status", Operator: filter.OpEqual, Value: "in-*", IsGlob: true},
			wantPushed: false,
		},
		{
			name:       "ordered comparison stays in Go",
			in:         &filter.Filter{Property: "due", Operator: filter.OpLess, Value: "2026-01-01"},
			wantPushed: false,
		},
		{
			name:       "regex stays in Go",
			in:         &filter.Filter{Property: "title", Operator: filter.OpRegex, Value: "^spike"},
			wantPushed: false,
		},
		{
			name:       "fuzzy stays in Go",
			in:         &filter.Filter{Property: "title", Operator: filter.OpFuzzy, Value: "retro"},
			wantPushed: false,
		},
		{
			// `count=03` matches the integer 3 when typed and misses as
			// strings, so pushing it would DROP a row that belongs in the
			// result — the precondition a pre-filter must never break.
			name:       "typed property is never pushed",
			in:         &filter.Filter{Property: "count", Operator: filter.OpEqual, Value: "03"},
			wantPushed: false,
		},
		{
			// filter.matchEnum errors on an undeclared value, surfacing an
			// operator typo. Pushed down the same typo silently matches
			// nothing and the source goes quiet with no diagnostic.
			name:       "undeclared property is never pushed",
			in:         &filter.Filter{Property: "nope", Operator: filter.OpEqual, Value: "x"},
			wantPushed: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			pushed := pushdownPrefilters([]*filter.Filter{tc.in}, pushdownTestMeta(), []string{"ticket"})

			if !tc.wantPushed {
				if len(pushed) != 0 {
					t.Errorf("expected no pushdown, got %+v", pushed)
				}
				return
			}
			if len(pushed) != 1 {
				t.Fatalf("expected one pushed predicate, got %d", len(pushed))
			}
			if pushed[0].Op != store.PropEqual {
				t.Errorf("op = %v, want PropEqual", pushed[0].Op)
			}
			if pushed[0].Property != tc.in.Property || pushed[0].Value != tc.in.Value {
				t.Errorf("predicate = %+v, want property/value from %+v", pushed[0], tc.in)
			}
			if pushed[0].Scalar != (tc.in.Value != "") {
				t.Errorf("Scalar = %v, want %v", pushed[0].Scalar, tc.in.Value != "")
			}
		})
	}
}

// A mixed query pushes only the equality; the rest is left for the Go pass,
// which evaluates ALL of them regardless.
func TestPushdownPrefilters_Mixed(t *testing.T) {
	t.Parallel()
	pushed := pushdownPrefilters([]*filter.Filter{
		{Property: "status", Operator: filter.OpEqual, Value: "open"},
		{Property: "due", Operator: filter.OpLess, Value: "2026-01-01"},
	}, pushdownTestMeta(), []string{"ticket"})
	if len(pushed) != 1 || pushed[0].Property != "status" {
		t.Errorf("expected only the equality pushed, got %+v", pushed)
	}
}

func TestPushdownPrefilters_Empty(t *testing.T) {
	t.Parallel()
	if got := pushdownPrefilters(nil, pushdownTestMeta(), []string{"ticket"}); len(got) != 0 {
		t.Errorf("nil filters should push nothing, got %+v", got)
	}
}

// TestCompareValues_Datetime covers the defect found in TKT-IG54YO design
// review: compareValues parsed only "2006-01-02", so a datetime-typed property
// (stored as RFC3339) compared against a window bound either errored — and the
// entity was excluded — or fell through to lexicographic string comparison,
// which is wrong the moment offsets differ. A calendar over a datetime source
// rendered empty with only a per-entity log line as diagnosis.
func TestCompareValues_Datetime(t *testing.T) {
	tests := []struct {
		name        string
		left, right string
		op          string
		want        bool
	}{
		// The exact failing case: stored RFC3339 vs a bare-date window bound.
		{"rfc3339 gte bare date, after", "2026-08-22T14:30:00Z", "2026-08-01", "gte", true},
		{"rfc3339 gte bare date, before", "2026-07-22T14:30:00Z", "2026-08-01", "gte", false},
		{"rfc3339 lt bare date, before", "2026-08-22T14:30:00Z", "2026-09-01", "lt", true},
		{"rfc3339 lt bare date, after", "2026-09-22T14:30:00Z", "2026-09-01", "lt", false},

		// A bare date denotes midnight, so an instant later that same day is
		// after it. This is why the calendar uses a half-open window.
		{"same day after midnight is gt", "2026-08-22T00:00:01Z", "2026-08-22", "gt", true},
		{"midnight equals bare date", "2026-08-22T00:00:00Z", "2026-08-22", "gte", true},

		// Offsets must normalize to instants, not compare as strings.
		// 12:00+02:00 == 10:00Z, so neither is strictly before the other.
		{"equal instants across offsets, lt", "2026-08-22T12:00:00+02:00", "2026-08-22T10:00:00Z", "lt", false},
		{"equal instants across offsets, lte", "2026-08-22T12:00:00+02:00", "2026-08-22T10:00:00Z", "lte", true},
		// Lexicographically "2026-08-22T09:00:00+02:00" > "2026-08-22T08:00:00Z",
		// but as instants 07:00Z < 08:00Z. The old code got this backwards.
		{"offset ordering beats string ordering", "2026-08-22T09:00:00+02:00", "2026-08-22T08:00:00Z", "lt", true},

		// Sub-second precision must not be truncated away.
		{"sub-second precision honored", "2026-08-22T10:00:00.500Z", "2026-08-22T10:00:00.100Z", "gt", true},

		// Zone-less stored values (the "naive datetime" form the SPA tolerates).
		{"naive datetime compares", "2026-08-22T14:30:00", "2026-08-22T09:00:00", "gt", true},

		// Plain dates keep working exactly as before.
		{"date vs date unchanged", "2026-08-22", "2026-08-01", "gt", true},

		// Far-future dates must still order correctly. Nanoseconds since the
		// epoch overflow an int64 outside roughly 1678-2262, so comparing on
		// UnixNano would wrap these negative and sort them before everything.
		{"far future is after near future", "2300-01-01", "2026-08-22", "gt", true},
		{"far future vs far future", "2400-01-01", "2300-01-01", "gt", true},
		{"distant past is before now", "1600-01-01", "2026-08-22", "lt", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := compareValues(tt.left, tt.right, tt.op)
			if err != nil {
				t.Fatalf("compareValues(%q, %q, %q) returned error: %v",
					tt.left, tt.right, tt.op, err)
			}
			if got != tt.want {
				t.Errorf("compareValues(%q, %q, %q) = %v, want %v",
					tt.left, tt.right, tt.op, got, tt.want)
			}
		})
	}
}

// TestCompareValues_DatetimeMismatch confirms that widening to datetimes did
// not weaken the type-mismatch guard: a genuine non-temporal string compared
// against a datetime must still error rather than fall back to lexicographic
// comparison.
func TestCompareValues_DatetimeMismatch(t *testing.T) {
	tests := []struct{ name, left, right string }{
		{"datetime vs word", "2026-08-22T14:30:00Z", "tomorrow"},
		{"word vs datetime", "tomorrow", "2026-08-22T14:30:00Z"},
		{"datetime vs number", "2026-08-22T14:30:00Z", "42"},
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
