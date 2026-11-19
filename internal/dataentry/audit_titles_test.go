package dataentry

import (
	"maps"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/audit"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/visibility"
)

// TestChangeTitles_ServesOnlyWholeTitles: the Changes list names a relation's
// far end by its title only when the reader sees every property the title is
// made of. A templated title with a hidden placeholder, or a hidden single
// display property, is left out, so the list shows the id rather than half a
// name (BUG-XR1DUQA, BUG-R9EHKV). A far end the reader cannot read at all
// gets no title either.
func TestChangeTitles_ServesOnlyWholeTitles(t *testing.T) {
	meta := &metamodel.Metamodel{
		Entities: map[string]metamodel.EntityDef{
			"persoon": {
				Label: "Persoon", IDPrefix: "PERS-",
				DisplayProperty: "{voornaam} {achternaam}",
				Properties: map[string]metamodel.PropertyDef{
					"voornaam":   {Type: "string"},
					"achternaam": {Type: "string"},
				},
				PropertyOrder: []string{"voornaam", "achternaam"},
			},
			"ticket": {
				Label: "Ticket", IDPrefix: "TKT-",
				Properties: map[string]metamodel.PropertyDef{
					"title":  {Type: "string", Required: true},
					"secret": {Type: "string"},
				},
				PropertyOrder: []string{"title", "secret"},
			},
			"note": {
				Label: "Note", IDPrefix: "NOTE-", DisplayProperty: "secret",
				Properties:    map[string]metamodel.PropertyDef{"secret": {Type: "string"}},
				PropertyOrder: []string{"secret"},
			},
			"memo": {
				Label: "Memo", IDPrefix: "MEMO-",
				Properties:    map[string]metamodel.PropertyDef{"title": {Type: "string", Required: true}},
				PropertyOrder: []string{"title"},
			},
		},
	}
	app := newAppFromParts(&Config{}, meta, newFixture())
	app.broker = newEventBroker()
	setTestResolver(t, app, fakeResolver{fv: FieldVerdicts{
		Visible: map[string]bool{"achternaam": false, "secret": false},
	}})
	seedEntity(app, &entity.Entity{ID: "TKT-1", Type: "ticket", Properties: map[string]any{"title": "Self"}})
	seedEntity(app, &entity.Entity{ID: "TKT-2", Type: "ticket", Properties: map[string]any{"title": "Other"}})
	seedEntity(app, &entity.Entity{ID: "PERS-1", Type: "persoon",
		Properties: map[string]any{"voornaam": "Ada", "achternaam": "SECRET-SURNAME"}})
	seedEntity(app, &entity.Entity{ID: "NOTE-1", Type: "note", Properties: map[string]any{"secret": "SECRET-NOTE"}})
	seedEntity(app, &entity.Entity{ID: "MEMO-1", Type: "memo", Properties: map[string]any{"title": "SECRET-MEMO"}})
	d := mustNewACL(t, &acl.Policy{
		Roles:       map[string]acl.RoleDef{"viewer": {Read: []string{"ticket", "persoon", "note"}}},
		Assignments: map[string]string{"alice": "viewer"},
	}, app.store)
	setTestACL(t, app, d)

	rel := func(to string) visibility.ChangeRecord {
		return visibility.ChangeRecord{Op: audit.OpCreateRelation, Subject: &audit.Subject{
			Kind: "relation", RelationType: "refs", FromID: "TKT-1", ToID: to,
		}}
	}
	page := []visibility.ChangeRecord{rel("TKT-2"), rel("PERS-1"), rel("NOTE-1"), rel("MEMO-1")}
	got := changeTitles(gateCtxFor(aliceCtx(), t, d), app, "TKT-1", page)
	if want := map[string]string{"TKT-2": "Other"}; !maps.Equal(got, want) {
		t.Errorf("titles = %v, want %v", got, want)
	}
}
