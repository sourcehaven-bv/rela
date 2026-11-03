package dataentryconfig

import (
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/metamodel"
)

func TestValidateSectionRelationFields(t *testing.T) {
	tests := []struct {
		name    string
		section ViewSection
		wantErr string
	}{
		{
			name: "valid",
			section: ViewSection{Source: "entry", Display: "properties", Fields: []ViewSectionField{
				{Property: "title"}, {Relation: "in-category", Label: "Category"},
			}},
		},
		{
			name: "property and relation",
			section: ViewSection{Source: "entry", Display: "properties", Fields: []ViewSectionField{
				{Property: "title", Relation: "in-category"},
			}},
			wantErr: "sets both property and relation",
		},
		{
			name: "cards section",
			section: ViewSection{Source: "entry", Display: "cards", Fields: []ViewSectionField{
				{Relation: "in-category"},
			}},
			wantErr: "only valid in a section with source: entry and display: properties",
		},
		{
			name: "unknown relation",
			section: ViewSection{Source: "entry", Display: "properties", Fields: []ViewSectionField{
				{Relation: "nope"},
			}},
			wantErr: `unknown relation "nope"`,
		},
		{
			name: "relation from another type",
			section: ViewSection{Source: "entry", Display: "properties", Fields: []ViewSectionField{
				{Relation: "offers-category"},
			}},
			wantErr: `does not start at entity type "ticket"`,
		},
		{
			name: "widget",
			section: ViewSection{Source: "entry", Display: "properties", Fields: []ViewSectionField{
				{Relation: "in-category", Widget: "textarea"},
			}},
			wantErr: "widget does not apply to a relation field",
		},
	}
	// style_from needs an enum property on the target; testMetamodel's ticket
	// has `status`, and offers-category's target is category (no enum).
	tests = append(tests,
		struct {
			name    string
			section ViewSection
			wantErr string
		}{
			name: "style_from on a non-enum property",
			section: ViewSection{Source: "entry", Display: "properties", Fields: []ViewSectionField{
				{Relation: "in-category", StyleFrom: "name"},
			}},
			wantErr: `style_from "name" on "category" is not an enum property`,
		},
		struct {
			name    string
			section ViewSection
			wantErr string
		}{
			name: "style_from unknown property",
			section: ViewSection{Source: "entry", Display: "properties", Fields: []ViewSectionField{
				{Relation: "in-category", StyleFrom: "color"},
			}},
			wantErr: `style_from "color" is not a property of "category"`,
		},
		struct {
			name    string
			section ViewSection
			wantErr string
		}{
			name: "style_from on a property field",
			section: ViewSection{Source: "entry", Display: "properties", Fields: []ViewSectionField{
				{Property: "title", StyleFrom: "status"},
			}},
			wantErr: "style_from applies only to a relation field",
		},
	)
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			errs := validateSectionRelationFields("v", 0, tt.section, "ticket", columnsFromMetamodel())
			got := strings.Join(errs, "\n")
			if tt.wantErr == "" {
				if got != "" {
					t.Fatalf("unexpected errors: %s", got)
				}
				return
			}
			if !strings.Contains(got, tt.wantErr) {
				t.Fatalf("errors = %q, want %q", got, tt.wantErr)
			}
		})
	}
}

// TestValidateConfig_ViewRelationField checks the rule is wired into
// validateViews and that a relation field passes the property checks.
func TestValidateConfig_ViewRelationField(t *testing.T) {
	cfg := &Config{Views: map[string]ViewConfig{"ticket": {
		Entry: ViewEntry{Type: "ticket"},
		Sections: []ViewSection{{Source: "entry", Display: "properties", Fields: []ViewSectionField{
			{Property: "title"}, {Relation: "in-category"},
		}}},
	}}}
	if errs := validateViews(cfg, columnsFromMetamodel()); len(errs) > 0 {
		t.Errorf("unexpected view errors: %v", errs)
	}
	cfg.Views["ticket"].Sections[0].Fields[1].Relation = "nope"
	if got := strings.Join(validateViews(cfg, columnsFromMetamodel()), "\n"); !strings.Contains(got, `unknown relation "nope"`) {
		t.Errorf("ValidateConfig did not report the unknown relation: %s", got)
	}
}

func TestValidateSectionRelationFields_StyleFromEnum(t *testing.T) {
	m := columnsFromMetamodel()
	cat := m.Entities["category"]
	cat.Properties["state"] = metamodel.PropertyDef{Type: "status"}
	m.Entities["category"] = cat
	s := ViewSection{Source: "entry", Display: "properties", Fields: []ViewSectionField{
		{Relation: "in-category", StyleFrom: "state"},
	}}
	if errs := validateSectionRelationFields("v", 0, s, "ticket", m); len(errs) > 0 {
		t.Errorf("unexpected errors: %v", errs)
	}
}
