package dataentryconfig

import (
	"strings"
	"testing"

	"gopkg.in/yaml.v3"

	"github.com/Sourcehaven-BV/rela/internal/metamodel"
)

func assignMetamodel() *metamodel.Metamodel {
	return &metamodel.Metamodel{
		Types: map[string]metamodel.CustomType{},
		Entities: map[string]metamodel.EntityDef{
			"todo": {Label: "To-do", IDPrefix: "TD-", Properties: map[string]metamodel.PropertyDef{
				"title":      {Type: "string"},
				"when":       {Type: metamodel.PropertyTypeEnum, Values: []string{"today", "someday"}},
				"start_date": {Type: "date"},
				"rank":       {Type: metamodel.PropertyTypeInteger},
				"flagged":    {Type: metamodel.PropertyTypeBoolean},
				"label":      {Type: "string", Computed: "entity.title"},
			}},
			"project": {Label: "Project", IDPrefix: "PRJ-", Properties: map[string]metamodel.PropertyDef{
				"title": {Type: "string"},
			}},
		},
		Relations: map[string]metamodel.RelationDef{
			"in_project": {From: []string{"todo"}, To: []string{"project"}},
		},
	}
}

func TestValidateAssigns(t *testing.T) {
	list := func(a *Assign) *Config {
		return &Config{Lists: map[string]List{"today": {EntityType: "todo", Assign: a}}}
	}
	page := func(a *Assign) *Config {
		return &Config{Pages: map[string]Page{"project": {
			PageHeader: PageHeader{Label: "Project", EntityType: "project"}, Assign: a,
		}}}
	}
	tests := []struct {
		name    string
		cfg     *Config
		wantErr string
	}{
		{"literal and expression", list(&Assign{Properties: map[string]AssignValue{
			"when": {Literal: "today"}, "start_date": {Expr: "today()"},
		}}), ""},
		{"empty block", list(&Assign{}), "assigns nothing"},
		{"unknown property", list(&Assign{Properties: map[string]AssignValue{"nope": {Literal: "x"}}}), "not a property"},
		{"computed property", list(&Assign{Properties: map[string]AssignValue{"label": {Literal: "x"}}}), "computed"},
		{"enum literal", list(&Assign{Properties: map[string]AssignValue{"when": {Literal: "later"}}}), "not one of"},
		{"unknown relation", list(&Assign{Relations: map[string]string{"nope": "PRJ-1"}}), "unknown relation"},
		{"target on a list", list(&Assign{Relations: map[string]string{"in_project": "target"}}), "only available on a page"},
		{"relation id on a list", list(&Assign{Relations: map[string]string{"in_project": "PRJ-1"}}), ""},
		{"page target", page(&Assign{EntityType: "todo", Relations: map[string]string{"in_project": "target"}}), ""},
		{"page without item type", page(&Assign{Relations: map[string]string{"in_project": "target"}}),
			"entity_type is required"},
		{"page wrong source", page(&Assign{EntityType: "project", Relations: map[string]string{"in_project": "target"}}),
			"does not start at"},
		{"action names list without assign", &Config{
			Lists:   map[string]List{"inbox": {EntityType: "todo"}},
			Actions: map[string]Action{"go": {Assign: "inbox"}},
		}, "has no assign"},
		{"null literal", list(&Assign{Properties: map[string]AssignValue{"when": {}}}), "has no value"},
		{"typed literals", list(&Assign{Properties: map[string]AssignValue{
			"rank": {Literal: "3"}, "flagged": {Literal: "true"}, "start_date": {Literal: "2026-11-24"},
		}}), ""},
		{"integer literal", list(&Assign{Properties: map[string]AssignValue{"rank": {Literal: "high"}}}), "not an integer"},
		{"boolean literal", list(&Assign{Properties: map[string]AssignValue{"flagged": {Literal: "maybe"}}}),
			"not a boolean"},
		{"date literal", list(&Assign{Properties: map[string]AssignValue{"start_date": {Literal: "soon"}}}), "not a date"},
		{"assign action with permission", &Config{
			Lists:   map[string]List{"today": {EntityType: "todo", Assign: &Assign{Properties: map[string]AssignValue{"when": {Literal: "today"}}}}},
			Actions: map[string]Action{"go": {Assign: "today", Permission: "triage"}},
		}, "not available on an assign action"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			errs := validateAssigns(tc.cfg, assignMetamodel())
			got := strings.Join(errs, "\n")
			if tc.wantErr == "" && got != "" {
				t.Fatalf("unexpected errors: %s", got)
			}
			if tc.wantErr != "" && !strings.Contains(got, tc.wantErr) {
				t.Fatalf("want %q in %q", tc.wantErr, got)
			}
		})
	}
}

func TestAssignValue_YAML(t *testing.T) {
	var a Assign
	src := "properties:\n  when: today\n  start_date: { expr: \"today()\" }\n"
	if err := yaml.Unmarshal([]byte(src), &a); err != nil {
		t.Fatal(err)
	}
	if a.Properties["when"].Literal != "today" || a.Properties["start_date"].Expr != "today()" {
		t.Fatalf("got %+v", a.Properties)
	}
	if err := yaml.Unmarshal([]byte("properties:\n  when: { expr: \"\" }\n"), &a); err == nil {
		t.Fatal("empty expr accepted")
	}
}
