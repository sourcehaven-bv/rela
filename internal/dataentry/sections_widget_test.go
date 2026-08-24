package dataentry

import (
	"context"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/entity"
)

// TKT-3R7RF3: the section builders carry the config's `widget:` override
// through to the wire verbatim. Resolving it is the SPA's job — the server
// only validates it at config load and passes it along.

func TestBuildSectionEntityData_CarriesWidget(t *testing.T) {
	tests := []struct {
		name    string
		widgets []string // per field, "" = unset
		want    []string
	}{
		{
			name:    "unset stays empty (SPA applies its type default)",
			widgets: []string{"", ""},
			want:    []string{"", ""},
		},
		{
			name:    "override is carried verbatim",
			widgets: []string{"textarea", ""},
			want:    []string{"textarea", ""},
		},
		{
			name:    "per-field, not per-section",
			widgets: []string{"textarea", "select"},
			want:    []string{"textarea", "select"},
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
				secFields[i] = ViewSectionField{Property: p, Widget: tc.widgets[i]}
			}

			sed := app.views.buildSectionEntityData(
				context.Background(), e, secFields, eDef, "input")

			if len(sed.Fields) != len(tc.want) {
				t.Fatalf("got %d fields, want %d", len(sed.Fields), len(tc.want))
			}
			for i, want := range tc.want {
				if got := sed.Fields[i].Widget; got != want {
					t.Errorf("field %q: Widget = %q, want %q", props[i], got, want)
				}
			}
		})
	}
}

// The entry-source `properties` branch is a SECOND construction site. Both go
// through buildSectionFieldData, which is exactly why a new field like this
// one cannot be wired into one and silently dropped from the other — the
// failure mode that helper's godoc exists to prevent.
func TestBuildSectionFieldData_CarriesWidget(t *testing.T) {
	e := &entity.Entity{
		ID:         "TKT-001",
		Type:       "ticket",
		Properties: map[string]any{"title": "First"},
	}
	got := buildSectionFieldData(
		ViewSectionField{Property: "title", Label: "Title", Widget: "textarea"},
		e, nil, "input")
	if got.Widget != "textarea" {
		t.Errorf("Widget = %q, want %q", got.Widget, "textarea")
	}
	// The two axes are independent: a widget override must not disturb render.
	if got.Render != "input" {
		t.Errorf("Render = %q, want %q (widget must not affect it)", got.Render, "input")
	}
}
