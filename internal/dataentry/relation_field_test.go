package dataentry

import (
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/dataentryconfig"
	"github.com/Sourcehaven-BV/rela/internal/entity"
)

// relationFieldSection builds the one entry properties section of these tests:
// a property field and a relation field on `implements` (TKT-CADCFX).
func relationFieldSection(label string) []ViewSection {
	return []ViewSection{{
		Source:  "entry",
		Display: "properties",
		Fields: []dataentryconfig.ViewSectionField{
			{Property: "title"},
			{Relation: "implements", Label: label},
		},
	}}
}

func TestRelationField_CarriesReadableTargets(t *testing.T) {
	app := newTestAppV1(t)
	tkt := &entity.Entity{ID: "TKT-001", Type: "ticket", Properties: map[string]any{"title": "ticket"}}
	seedEntity(app, tkt)
	seedEntity(app, &entity.Entity{ID: "FEAT-001", Type: "feature", Properties: map[string]any{"title": "Search"}})
	seedRelation(app, entity.NewRelation("TKT-001", "implements", "FEAT-001"))

	sections := app.views.buildSections(t.Context(), relationFieldSection("Feature"),
		&viewResult{Entry: tkt})
	f := sections[0].Fields[1]
	if f.Relation != "implements" || f.Property != "" || f.Label != "Feature" {
		t.Fatalf("relation field = %+v", f)
	}
	if len(f.Targets) != 1 || f.Targets[0].ID != "FEAT-001" || f.Targets[0].Title != "Search" {
		t.Errorf("targets = %+v, want FEAT-001 Search", f.Targets)
	}
}

func TestRelationField_DefaultsLabelToRelationLabel(t *testing.T) {
	app := newTestAppV1(t)
	tkt := &entity.Entity{ID: "TKT-001", Type: "ticket", Properties: map[string]any{"title": "ticket"}}
	seedEntity(app, tkt)

	sections := app.views.buildSections(t.Context(), relationFieldSection(""), &viewResult{Entry: tkt})
	f := sections[0].Fields[1]
	want := "implements"
	if def, ok := app.State().Meta.GetRelationDef("implements"); ok && def.Label != "" {
		want = def.Label
	}
	if f.Label != want {
		t.Errorf("label = %q, want %q", f.Label, want)
	}
	if len(f.Targets) != 0 {
		t.Errorf("targets = %+v, want none", f.Targets)
	}
}

func TestRelationField_DropsUnreadableTarget(t *testing.T) {
	app := newTestAppV1(t)
	tkt := &entity.Entity{ID: "TKT-001", Type: "ticket", Properties: map[string]any{"title": "ticket"}}
	seedEntity(app, tkt)
	seedEntity(app, &entity.Entity{ID: "FEAT-001", Type: "feature",
		Properties: map[string]any{"title": "SECRET-FEATURE"}})
	seedRelation(app, entity.NewRelation("TKT-001", "implements", "FEAT-001"))

	d := mustNewACL(t, &acl.Policy{
		Roles:       map[string]acl.RoleDef{"viewer": {Read: []string{"ticket"}}},
		Assignments: map[string]string{"alice": "viewer"},
	}, app.store)
	app.acl = d

	sections := app.views.buildSections(gateCtxFor(aliceCtx(), t, d), relationFieldSection("Feature"),
		&viewResult{Entry: tkt})
	if got := sections[0].Fields[1].Targets; len(got) != 0 {
		t.Errorf("unreadable target leaked into relation field: %+v", got)
	}
}

func TestRelationField_CarriesStyleValue(t *testing.T) {
	app := newTestAppV1(t)
	tkt := &entity.Entity{ID: "TKT-001", Type: "ticket", Properties: map[string]any{"title": "ticket"}}
	seedEntity(app, tkt)
	seedEntity(app, &entity.Entity{ID: "FEAT-001", Type: "feature",
		Properties: map[string]any{"title": "Search", "status": "active"}})
	seedRelation(app, entity.NewRelation("TKT-001", "implements", "FEAT-001"))

	secs := relationFieldSection("Feature")
	secs[0].Fields[1].StyleFrom = "status"
	sections := app.views.buildSections(t.Context(), secs, &viewResult{Entry: tkt})
	f := sections[0].Fields[1]
	if f.StyleFrom != "status" {
		t.Errorf("styleFrom = %q, want status", f.StyleFrom)
	}
	if len(f.Targets) != 1 || f.Targets[0].Style != "active" {
		t.Errorf("targets = %+v, want style active", f.Targets)
	}
}
