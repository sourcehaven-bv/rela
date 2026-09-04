package automation

import (
	"context"
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/testutil"
)

func facedTicket(face string) *entity.Entity {
	e := buildEntity(testutil.Entity("ticket"))
	e.Face = entity.Face(face)
	return e
}

func facedMeta() *metamodel.Metamodel {
	return &metamodel.Metamodel{Entities: map[string]metamodel.EntityDef{
		"ticket": {BareFace: "en", Faces: map[string]metamodel.FaceDef{"en": {}, "nl": {}}},
	}}
}

// The pre-existing behavior, pinned: an unscoped automation fires on EVERY
// content state. This is the default `faces:` preserves, so it must not drift.
func TestEngine_UnscopedFiresOnEveryFace(t *testing.T) {
	t.Parallel()
	engine := NewEngine([]Automation{
		newAutomation("flag").OnCreate("ticket").Set("flag", "yes").Build(),
	})
	for _, face := range []string{"", "nl"} {
		res := engine.Process(context.Background(), Event{
			Type: EventEntityCreated, Entity: facedTicket(face),
		})
		if res.PropertiesSet["flag"] != "yes" {
			t.Errorf("face %q: unscoped automation must fire, got %v", face, res.PropertiesSet)
		}
	}
}

// `faces:` narrows the trigger. The comparison is against the DECLARED name,
// so scoping to the bare face uses its declared spelling rather than "".
func TestEngine_ScopedToOneFace(t *testing.T) {
	t.Parallel()
	auto := newAutomation("nl-only").OnCreate("ticket").Set("flag", "yes").Build()
	auto.On.Faces = []string{"nl"}
	engine := NewEngineWithMeta(t, []Automation{auto})

	res := engine.Process(context.Background(), Event{
		Type: EventEntityCreated, Entity: facedTicket("nl"),
	})
	if res.PropertiesSet["flag"] != "yes" {
		t.Errorf("scoped face must fire, got %v", res.PropertiesSet)
	}

	res = engine.Process(context.Background(), Event{
		Type: EventEntityCreated, Entity: facedTicket(""),
	})
	if len(res.PropertiesSet) != 0 {
		t.Errorf("bare face is out of scope and must NOT fire, got %v", res.PropertiesSet)
	}
}

// Scoping to the bare face: the operator writes its declared name, and the row
// is stored at the empty coordinate. The engine maps between the two.
func TestEngine_ScopedToTheBareFace(t *testing.T) {
	t.Parallel()
	auto := newAutomation("en-only").OnCreate("ticket").Set("flag", "yes").Build()
	auto.On.Faces = []string{"en"}
	engine := NewEngineWithMeta(t, []Automation{auto})

	res := engine.Process(context.Background(), Event{
		Type: EventEntityCreated, Entity: facedTicket(""),
	})
	if res.PropertiesSet["flag"] != "yes" {
		t.Errorf("the bare row IS the `en` face and must fire, got %v", res.PropertiesSet)
	}

	res = engine.Process(context.Background(), Event{
		Type: EventEntityCreated, Entity: facedTicket("nl"),
	})
	if len(res.PropertiesSet) != 0 {
		t.Errorf("nl is out of scope and must NOT fire, got %v", res.PropertiesSet)
	}
}

// An `on.faces:` naming a face the type does not declare is a LOAD error: the
// trigger would never fire, which is a constraint that silently disables the
// automation it was meant to narrow.
func TestAutomationFaces_UndeclaredIsALoadError(t *testing.T) {
	t.Parallel()
	doc := `version: "1"
entities:
  ticket:
    label: Ticket
    id_prefix: TKT
    bare_face: en
    faces: {en: {}, nl: {}}
    properties:
      title: {type: string}
automations:
  - name: typo-scope
    on: {entity: ticket, created: true, faces: [nl-BE]}
    do:
      - set: flag
        value: "yes"
`
	if _, err := metamodel.Parse([]byte(doc)); err == nil {
		t.Fatal("a trigger scoped to an undeclared face must fail the load rather " +
			"than silently never firing")
	}
}

// NewEngineWithMeta builds an engine that can resolve declared face names.
func NewEngineWithMeta(t *testing.T, autos []Automation) *Engine {
	t.Helper()
	e := NewEngine(autos)
	e.meta = facedMeta()
	return e
}

// Same distinction as the validator's: a scope on a type with no faces is a
// different mistake from a misspelled one.
func TestAutomationFaces_ScopeOnUnfacedTypeExplainsItself(t *testing.T) {
	t.Parallel()
	doc := `version: "1"
entities:
  note:
    label: Note
    id_prefix: NOTE
    properties:
      title: {type: string}
automations:
  - name: scope-on-unfaced
    on: {entity: note, created: true, faces: [published]}
    do:
      - set: flag
        value: "yes"
`
	_, err := metamodel.Parse([]byte(doc))
	if err == nil {
		t.Fatal("a scope on a type with no faces must fail the load")
	}
	if !strings.Contains(err.Error(), "declares no `faces:`") {
		t.Errorf("the error must say the TYPE has no faces; got: %v", err)
	}
}
