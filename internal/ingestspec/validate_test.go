package ingestspec

import (
	"slices"
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/metamodel"
)

// The full message set is pinned through the webhook config tests in
// internal/dataentryconfig, which assert the same strings with their prefix.
// These cases cover each branch from this package's side.
func TestMappingValidate(t *testing.T) {
	meta := &metamodel.Metamodel{Entities: map[string]metamodel.EntityDef{
		"item": {Properties: map[string]metamodel.PropertyDef{"guid": {Type: "string"}, "title": {Type: "string"}}},
		"page": {
			Properties: map[string]metamodel.PropertyDef{"title": {Type: "string"}},
			Faces:      map[string]metamodel.FaceDef{"draft": {}},
		},
	}}
	find := &Find{Type: "item", Match: []string{"guid"}}

	for _, tc := range []struct {
		name string
		m    Mapping
		want string // substring; empty means valid
	}{
		{"find-or-create", Mapping{Find: find, CreateIfMissing: &Create{}}, ""},
		{"nothing to do", Mapping{}, "neither find nor create_if_missing"},
		{"create without type", Mapping{CreateIfMissing: &Create{}}, "needs a type"},
		{"unknown create type", Mapping{CreateIfMissing: &Create{Type: "x"}}, `"x" is not a known entity type`},
		{
			"unknown create property",
			Mapping{CreateIfMissing: &Create{Type: "item", Properties: map[string]string{"nope": "v"}}},
			`unknown property "nope"`,
		},
		{"find without type", Mapping{Find: &Find{}}, "find requires a type"},
		{"unknown find type", Mapping{Find: &Find{Type: "x"}}, `find type "x" is not a known`},
		{"create without match", Mapping{Find: &Find{Type: "item"}, CreateIfMissing: &Create{}}, "a match key is required"},
		{"unknown match property", Mapping{Find: &Find{Type: "item", Match: []string{"nope"}}}, `unknown property "nope"`},
		{
			"value outside match",
			Mapping{Find: &Find{Type: "item", Match: []string{"guid"}, Values: map[string]string{"title": "x"}}},
			"not in find.match",
		},
		{"face on faceless type", Mapping{Find: &Find{Type: "item", Face: "draft"}}, "declares no faces"},
		{"missing face", Mapping{Find: &Find{Type: "page"}}, "find.face is required"},
		{"unknown face", Mapping{Find: &Find{Type: "page", Face: "live"}}, `"live" is not a face`},
		{
			"faced create of another type",
			Mapping{Find: &Find{Type: "item", Match: []string{"guid"}}, CreateIfMissing: &Create{Type: "page"}},
			`create type "page" declares faces`,
		},
		{"empty step", Mapping{Find: find, Then: []Step{{}}}, "declares no action"},
		{
			"two actions",
			Mapping{Find: find, Then: []Step{{Set: map[string]string{"title": "x"}, AppendSection: &AppendSection{}}}},
			"more than one action",
		},
		{"append without content", Mapping{Find: find, Then: []Step{{AppendSection: &AppendSection{}}}}, "requires content"},
		{"set unknown", Mapping{Find: find, Then: []Step{{Set: map[string]string{"x": "y"}}}}, `sets unknown property "x"`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			errs := tc.m.Validate(meta)
			if tc.want == "" {
				if len(errs) != 0 {
					t.Fatalf("Validate = %v, want none", errs)
				}
				return
			}
			if !slices.ContainsFunc(errs, func(e string) bool { return strings.Contains(e, tc.want) }) {
				t.Errorf("Validate = %v, want one containing %q", errs, tc.want)
			}
		})
	}
}

func TestMappingValidate_NilMetamodel(t *testing.T) {
	errs := Mapping{Find: &Find{Type: "item"}}.Validate(nil)
	if len(errs) != 1 || !strings.Contains(errs[0], "is not a known entity type") {
		t.Errorf("Validate(nil) = %v, want the type reported unknown", errs)
	}
}

func TestMappingTemplates(t *testing.T) {
	m := Mapping{
		Find: &Find{Type: "t", Values: map[string]string{"k": "{{a}}"}},
		CreateIfMissing: &Create{
			Properties: map[string]string{"p": "{{b}}"},
			Content:    "{{c}}",
		},
		Then: []Step{
			{AppendSection: &AppendSection{Section: "Log", Content: "{{d}}"}},
			{Set: map[string]string{"s": "{{e}}"}},
		},
	}
	var got []string
	m.Templates(func(path, tmpl string) { got = append(got, path+"="+tmpl) })
	want := []string{
		"find.values.k={{a}}",
		"create_if_missing.properties.p={{b}}",
		"create_if_missing.content={{c}}",
		"then[0].append_section.content={{d}}",
		"then[1].set.s={{e}}",
	}
	if !slices.Equal(got, want) {
		t.Errorf("Templates visited %v, want %v", got, want)
	}
}
