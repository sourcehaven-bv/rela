package dataentry

import (
	"context"
	"encoding/json"
	"net/http"
	"slices"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/dataentryconfig"
	"github.com/Sourcehaven-BV/rela/internal/entity"
)

// Tests for sections the reader gets nothing from: a `permission:` they do
// not hold, or an entry properties section whose every field `visible:`
// withholds from them.

// withheldView configures a view on TKT-001 with one section per case.
func withheldView(t *testing.T) *App {
	t.Helper()
	app := newTestAppV1(t)
	app.State().Cfg.Forms["create_feature"] = dataentryconfig.Form{EntityType: "feature"}
	seedEntity(app, &entity.Entity{
		ID: "TKT-001", Type: "ticket", Content: "body",
		Properties: map[string]any{"title": "entry", "status": "open", "points": 3},
	})
	seedEntity(app, &entity.Entity{
		ID: "FEAT-001", Type: "feature", Properties: map[string]any{"title": "other"},
	})
	seedRelation(app, &entity.Relation{From: "TKT-001", Type: "implements", To: "FEAT-001"})
	app.State().Cfg.Views["v"] = ViewConfig{
		Entry:    ViewEntry{Type: "ticket"},
		Traverse: []ViewTraverse{{From: "entry", Follow: "implements", CollectAs: "features"}},
		Sections: []ViewSection{
			{Heading: "Redacted", Source: "entry", Display: "properties", Fields: []ViewSectionField{
				{Property: "status"}, {Property: "points"},
			}},
			{Heading: "Mixed", Source: "entry", Display: "properties", Fields: []ViewSectionField{
				{Property: "title"}, {Property: "status"},
			}},
			{Heading: "With relation", Source: "entry", Display: "properties", Fields: []ViewSectionField{
				{Property: "status"}, {Relation: "implements"},
			}},
			{Heading: "Body", Source: "entry", Display: "content"},
			{Heading: "Staff", Source: "entry", Display: "properties", SectionPresentation: dataentryconfig.SectionPresentation{SectionVisibility: dataentryconfig.SectionVisibility{Permission: "staff.manage"}},
				Fields: []ViewSectionField{{Property: "title"}}},
			{Heading: "Staff features", Source: "features", Display: "cards", SectionPresentation: dataentryconfig.SectionPresentation{SectionVisibility: dataentryconfig.SectionVisibility{Permission: "staff.manage"}},
				Create: &dataentryconfig.SectionCreate{}},
			{Heading: "Features", Source: "features", Display: "cards", Create: &dataentryconfig.SectionCreate{}},
		},
	}
	return app
}

// TestWithheldSections_PerReader pins which sections each reader gets. The
// reader holds no permission and may see only the title; staff hold the
// permission and have no `visible:` block, so they see every field.
func TestWithheldSections_PerReader(t *testing.T) {
	all := []string{"Redacted", "Mixed", "With relation", "Body", "Staff", "Staff features", "Features"}
	tests := []struct {
		name      string
		principal string
		want      []string
	}{
		{name: "reader", principal: "bob", want: []string{"Mixed", "With relation", "Body", "Features"}},
		{name: "staff", principal: "alice", want: all},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			app := withheldView(t)
			d := mustNewACL(t, &acl.Policy{
				Roles: map[string]acl.RoleDef{
					"reader": {
						Read: []string{"*"}, Create: []string{"feature"},
						Visible: map[string][]acl.FieldGrant{"ticket": {{Field: "title"}}},
					},
					"staff": {Read: []string{"*"}, Create: []string{"feature"}, Permissions: []string{"staff.manage"}},
				},
				Assignments: map[string]string{"bob": "reader", "alice": "staff"},
			}, app.store)
			setTestACL(t, app, d)
			res, err := ResolverFromProfile("", app.Meta(), app.store, d, nil)
			if err != nil {
				t.Fatal(err)
			}
			setTestResolver(t, app, res)

			rec := viewsAs(principalCtx(tc.principal), t, app, d, "ticket", "TKT-001")
			if rec.Code != http.StatusOK {
				t.Fatalf("status = %d, want 200; body=%s", rec.Code, rec.Body)
			}
			var body v1ViewBody
			if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
				t.Fatal(err)
			}
			if got := sectionHeadings(body); !slices.Equal(got, tc.want) {
				t.Fatalf("sections = %v, want %v", got, tc.want)
			}
			// The create affordance stays on the section it belongs to after
			// a gated section before it is dropped.
			for _, sec := range body.Sections {
				if sec.Heading == "Features" && sec.Create == nil {
					t.Error("Features lost its create affordance")
				}
			}
		})
	}
}

// TestWithheldSections_NoPolicyShowsAll: without acl.yaml, and under
// --read-only, nothing is redacted and every permission counts as held.
func TestWithheldSections_NoPolicyShowsAll(t *testing.T) {
	want := []string{"Redacted", "Mixed", "With relation", "Body", "Staff", "Staff features", "Features"}
	tests := []struct {
		name     string
		readOnly bool
	}{
		{name: "no policy"},
		{name: "read-only", readOnly: true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			app := withheldView(t)
			if tc.readOnly {
				setTestACL(t, app, acl.ReadOnlyACL{})
			}
			body := fetchView(context.Background(), t, app)
			if got := sectionHeadings(body); !slices.Equal(got, want) {
				t.Fatalf("sections = %v, want %v", got, want)
			}
		})
	}
}

func TestAllFieldsRedacted(t *testing.T) {
	redacted := &entity.Entity{Redacted: []string{"points", "status"}}
	tests := []struct {
		name   string
		fields []ViewSectionField
		e      *entity.Entity
		want   bool
	}{
		{name: "every field redacted", fields: []ViewSectionField{{Property: "status"}, {Property: "points"}},
			e: redacted, want: true},
		{name: "one field readable", fields: []ViewSectionField{{Property: "status"}, {Property: "title"}},
			e: redacted, want: false},
		{name: "relation field is content", fields: []ViewSectionField{{Property: "status"}, {Relation: "implements"}},
			e: redacted, want: false},
		{name: "no fields", fields: nil, e: redacted, want: false},
		{name: "nothing redacted", fields: []ViewSectionField{{Property: "status"}}, e: &entity.Entity{}, want: false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := allFieldsRedacted(tc.fields, tc.e); got != tc.want {
				t.Errorf("allFieldsRedacted = %v, want %v", got, tc.want)
			}
		})
	}
}
