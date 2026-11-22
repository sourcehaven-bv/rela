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

// TestChangeTitles_NamesActorsByTitle: a record's user that is an entity id
// the reader may read is served by its title, the entity on screen included,
// in the same batch as the relation ends (BUG-9WCZYOX). A user the reader
// cannot read, and reserved, integration and webhook identities, get none.
func TestChangeTitles_NamesActorsByTitle(t *testing.T) {
	meta := &metamodel.Metamodel{
		Entities: map[string]metamodel.EntityDef{
			"person": {
				Label: "Person", IDPrefix: "PSN-",
				Properties:    map[string]metamodel.PropertyDef{"title": {Type: "string", Required: true}},
				PropertyOrder: []string{"title"},
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
	setTestResolver(t, app, fakeResolver{})
	seedEntity(app, &entity.Entity{ID: "PSN-1", Type: "person", Properties: map[string]any{"title": "Ada"}})
	seedEntity(app, &entity.Entity{ID: "PSN-2", Type: "person", Properties: map[string]any{"title": "Self"}})
	seedEntity(app, &entity.Entity{ID: "MEMO-1", Type: "memo", Properties: map[string]any{"title": "SECRET-MEMO"}})
	d := mustNewACL(t, &acl.Policy{
		Roles:       map[string]acl.RoleDef{"viewer": {Read: []string{"person"}}},
		Assignments: map[string]string{"alice": "viewer"},
	}, app.store)
	setTestACL(t, app, d)

	by := func(user string) visibility.ChangeRecord {
		return visibility.ChangeRecord{Op: audit.OpUpdateEntity, User: user}
	}
	page := []visibility.ChangeRecord{
		by("PSN-1"), by("PSN-2"), by("MEMO-1"), by("system:scheduler"),
		by("integration:hub"), by("webhook:membership.created"), by("unknown"), by(""),
	}
	got := changeTitles(gateCtxFor(aliceCtx(), t, d), app, "PSN-2", page)
	if want := map[string]string{"PSN-1": "Ada", "PSN-2": "Self"}; !maps.Equal(got, want) {
		t.Errorf("titles = %v, want %v", got, want)
	}
}

func TestActorMayBeEntity(t *testing.T) {
	cases := map[string]bool{
		"PSN-1": true, "alice": true, "": false, "unknown": false,
		"system:scheduler": false, "system:webhook:hub": false,
		"integration:hub": false, "webhook:membership.created": false,
	}
	for user, want := range cases {
		if got := actorMayBeEntity(user); got != want {
			t.Errorf("actorMayBeEntity(%q) = %v, want %v", user, got, want)
		}
	}
}
