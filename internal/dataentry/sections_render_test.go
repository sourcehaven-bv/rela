package dataentry

import (
	"context"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/dataentryconfig"
	"github.com/Sourcehaven-BV/rela/internal/entity"
)

// TKT-HOIX1: the section builders resolve each field's render mode
// server-side, so the SPA never reimplements the section→field inheritance.

func TestBuildSectionEntityData_ResolvesRender(t *testing.T) {
	tests := []struct {
		name          string
		sectionRender string
		fieldRenders  []string // per field, "" = unset
		want          []string
	}{
		{
			name:         "unset defaults to display",
			fieldRenders: []string{"", ""},
			want:         []string{"display", "display"},
		},
		{
			name:         "field opts in",
			fieldRenders: []string{"input", ""},
			want:         []string{"input", "display"},
		},
		{
			name:          "section default applies to both",
			sectionRender: "input",
			fieldRenders:  []string{"", ""},
			want:          []string{"input", "input"},
		},
		{
			name:          "field overrides the section default",
			sectionRender: "input",
			fieldRenders:  []string{"display", ""},
			want:          []string{"display", "input"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			app := testViewApp()
			st := app.State()
			e := &entity.Entity{
				ID:         "TKT-001",
				Type:       "ticket",
				Properties: map[string]any{"title": "First", "status": "open"},
			}
			eDef, _ := st.Meta.GetEntityDef(e.Type)
			props := []string{"title", "status"}
			secFields := make([]ViewSectionField, len(props))
			for i, p := range props {
				secFields[i] = ViewSectionField{Property: p, Render: tc.fieldRenders[i]}
			}

			sed := app.views.buildSectionEntityData(
				context.Background(), e, secFields, eDef, tc.sectionRender, defaultViewWorld())

			if len(sed.Fields) != len(tc.want) {
				t.Fatalf("got %d fields, want %d", len(sed.Fields), len(tc.want))
			}
			for i, want := range tc.want {
				if got := sed.Fields[i].Render; got != want {
					t.Errorf("field %q: Render = %q, want %q", props[i], got, want)
				}
			}
		})
	}
}

// A synthesized default view (for an entity type with no `views:` entry)
// renders its properties as input. The operator authored no view, so there is
// no display choice to honor, and the ACL verdict still decides writability.
// An operator who wants such a type read-only authors an explicit view.
func TestBuildDefaultViewConfig_RendersInputByDefault(t *testing.T) {
	view, ok := buildDefaultViewConfig(newDefaultViewMetamodel(), "feature")
	if !ok {
		t.Fatal("buildDefaultViewConfig(feature) returned !ok")
	}
	var propsSection *ViewSection
	for i := range view.Sections {
		if view.Sections[i].Display == "properties" {
			propsSection = &view.Sections[i]
			break
		}
	}
	if propsSection == nil {
		t.Fatal("no properties section in the synthesized view")
	}
	for _, f := range propsSection.Fields {
		if got := resolveFieldRender(propsSection.Render, f.Render); got != dataentryconfig.RenderInput {
			t.Errorf("field %q resolves to %q, want %q", f.Property, got, dataentryconfig.RenderInput)
		}
	}
}

// The resolved mode must survive the unnamed struct conversion to the wire
// type. That conversion (`v1.SectionField(f)`) exists at four call sites and
// requires SectionFieldData and v1.SectionField to stay field-for-field
// identical — this asserts the value actually lands on the wire (RR-1V04ZD).
func TestSectionEntityToV1_CarriesRender(t *testing.T) {
	app := testViewApp()
	st := app.State()
	e := &entity.Entity{
		ID:         "TKT-001",
		Type:       "ticket",
		Properties: map[string]any{"title": "First", "status": "open"},
	}
	eDef, _ := st.Meta.GetEntityDef(e.Type)
	secFields := []ViewSectionField{
		{Property: "title", Render: "input"},
		{Property: "status"},
	}

	sed := app.views.buildSectionEntityData(context.Background(), e, secFields, eDef, "", defaultViewWorld())
	v1Ent := sectionEntityToV1(sed)

	if len(v1Ent.Fields) != 2 {
		t.Fatalf("got %d wire fields, want 2", len(v1Ent.Fields))
	}
	if got := v1Ent.Fields[0].Render; got != "input" {
		t.Errorf("title: wire Render = %q, want %q", got, "input")
	}
	if got := v1Ent.Fields[1].Render; got != "display" {
		t.Errorf("status: wire Render = %q, want %q", got, "display")
	}
}
