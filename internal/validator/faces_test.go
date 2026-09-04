package validator_test

import (
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/lua"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/store/memstore"
	"github.com/Sourcehaven-BV/rela/internal/validator"
)

// facedMeta declares `guide` with two content states and a required title.
func facedMeta(rules ...metamodel.ValidationRule) *metamodel.Metamodel {
	return &metamodel.Metamodel{
		Entities: map[string]metamodel.EntityDef{"guide": {
			Label: "Guide", BareFace: "en",
			Faces:      map[string]metamodel.FaceDef{"en": {}, "nl": {}},
			Properties: map[string]metamodel.PropertyDef{"title": {Type: "string", Required: true}},
		}},
		Validations: rules,
	}
}

// A validation rule must evaluate every content state, not only the bare row.
// Before this, loadCandidates left EntityQuery.AllStates at its zero value —
// "default-state rows only" — so a face missing a required property produced
// ZERO violations and `rela validate` reported a clean run (TKT-4Y6CMV).
func TestValidate_SeesNonBareFaces(t *testing.T) {
	ctx := t.Context()
	st := memstore.New()
	// Bare row is fine; the nl face is missing the required title.
	mustCreate(t, st, &entity.Entity{ID: "G-1", Type: "guide",
		Properties: map[string]any{"title": "ok"}})
	mustCreate(t, st, &entity.Entity{ID: "G-1", Type: "guide", Face: entity.Face("nl"),
		Properties: map[string]any{}})

	rule := metamodel.ValidationRule{Name: "title-required", EntityType: "guide", Then: []string{"title!="}}
	meta := facedMeta(rule)
	v := validator.New(st, meta, lua.ReadDeps{Meta: meta})

	full, err := v.CheckRuleFull(ctx, rule)
	if err != nil {
		t.Fatalf("CheckRuleFull: %v", err)
	}
	if len(full.Violations) != 1 {
		t.Fatalf("got %d violations, want 1 — the invalid nl face must be caught", len(full.Violations))
	}
	got := full.Violations[0]
	if got.EntityID != "G-1" {
		t.Errorf("EntityID = %q, want G-1", got.EntityID)
	}
	// The operator has to be able to tell WHICH state is wrong; a bare id names
	// two rows here and only one of them is in violation.
	if got.Face != "nl" {
		t.Errorf("Face = %q, want nl — a violation that cannot name its state is unactionable", got.Face)
	}
}

// The bare face is stored as the EMPTY coordinate, so a violation on it must
// report the type's declared bare_face name rather than an empty string —
// otherwise the two rows are indistinguishable in the output.
func TestValidate_BareFaceViolationNamesTheDeclaredFace(t *testing.T) {
	ctx := t.Context()
	st := memstore.New()
	mustCreate(t, st, &entity.Entity{ID: "G-2", Type: "guide", Properties: map[string]any{}})

	rule := metamodel.ValidationRule{Name: "title-required", EntityType: "guide", Then: []string{"title!="}}
	meta := facedMeta(rule)
	v := validator.New(st, meta, lua.ReadDeps{Meta: meta})

	full, err := v.CheckRuleFull(ctx, rule)
	if err != nil {
		t.Fatalf("CheckRuleFull: %v", err)
	}
	if len(full.Violations) != 1 {
		t.Fatalf("got %d violations, want 1", len(full.Violations))
	}
	if got := full.Violations[0].Face; got != "en" {
		t.Errorf("Face = %q, want en (the declared bare_face), not the empty stored coordinate", got)
	}
}

// A type with no faces must be unaffected: no face is reported, because there
// is no state to name.
func TestValidate_UnfacedTypeReportsNoFace(t *testing.T) {
	ctx := t.Context()
	st := memstore.New()
	mustCreate(t, st, &entity.Entity{ID: "P-1", Type: "plain", Properties: map[string]any{}})

	rule := metamodel.ValidationRule{Name: "title-required", EntityType: "plain", Then: []string{"title!="}}
	meta := &metamodel.Metamodel{
		Entities: map[string]metamodel.EntityDef{"plain": {Label: "Plain",
			Properties: map[string]metamodel.PropertyDef{"title": {Type: "string", Required: true}}}},
		Validations: []metamodel.ValidationRule{rule},
	}
	v := validator.New(st, meta, lua.ReadDeps{Meta: meta})

	full, err := v.CheckRuleFull(ctx, rule)
	if err != nil {
		t.Fatalf("CheckRuleFull: %v", err)
	}
	if len(full.Violations) != 1 {
		t.Fatalf("got %d violations, want 1", len(full.Violations))
	}
	if got := full.Violations[0].Face; got != "" {
		t.Errorf("Face = %q, want empty for an unfaced type", got)
	}
}

// `faces:` narrows a rule to specific states. A rule about the published face
// should not fire on every work-in-progress draft — a validator that cries wolf
// trains people to ignore it.
func TestValidate_RuleScopedToOneFace(t *testing.T) {
	ctx := t.Context()
	st := memstore.New()
	// Both states are missing the title; only the scoped one should be reported.
	mustCreate(t, st, &entity.Entity{ID: "G-3", Type: "guide", Properties: map[string]any{}})
	mustCreate(t, st, &entity.Entity{ID: "G-3", Type: "guide", Face: entity.Face("nl"),
		Properties: map[string]any{}})

	rule := metamodel.ValidationRule{
		Name: "nl-title-required", EntityType: "guide",
		Faces: []string{"nl"}, Then: []string{"title!="},
	}
	meta := facedMeta(rule)
	v := validator.New(st, meta, lua.ReadDeps{Meta: meta})

	full, err := v.CheckRuleFull(ctx, rule)
	if err != nil {
		t.Fatalf("CheckRuleFull: %v", err)
	}
	if len(full.Violations) != 1 {
		t.Fatalf("got %d violations, want exactly 1 — the scope must exclude the bare face", len(full.Violations))
	}
	if got := full.Violations[0].Face; got != "nl" {
		t.Errorf("Face = %q, want nl", got)
	}
}

// Scoping to the BARE face works too, and must be written as its declared
// name — the empty stored coordinate is not the spelling an operator uses.
func TestValidate_RuleScopedToTheBareFace(t *testing.T) {
	ctx := t.Context()
	st := memstore.New()
	mustCreate(t, st, &entity.Entity{ID: "G-4", Type: "guide", Properties: map[string]any{}})
	mustCreate(t, st, &entity.Entity{ID: "G-4", Type: "guide", Face: entity.Face("nl"),
		Properties: map[string]any{}})

	rule := metamodel.ValidationRule{
		Name: "en-title-required", EntityType: "guide",
		Faces: []string{"en"}, Then: []string{"title!="},
	}
	meta := facedMeta(rule)
	v := validator.New(st, meta, lua.ReadDeps{Meta: meta})

	full, err := v.CheckRuleFull(ctx, rule)
	if err != nil {
		t.Fatalf("CheckRuleFull: %v", err)
	}
	if len(full.Violations) != 1 {
		t.Fatalf("got %d violations, want 1", len(full.Violations))
	}
	if got := full.Violations[0].Face; got != "en" {
		t.Errorf("Face = %q, want en", got)
	}
}

// A scope naming a face the type does not declare is a LOAD error: the rule
// would match nothing and pass forever while appearing to guard something.
func TestValidate_UndeclaredFaceInScopeIsALoadError(t *testing.T) {
	doc := `version: "1"
entities:
  guide:
    label: Guide
    id_prefix: GUIDE
    bare_face: en
    faces: {en: {}, nl: {}}
    properties:
      title: {type: string}
validations:
  - name: typo-scope
    entity_type: guide
    faces: [nl-BE]
    then: ["title!="]
`
	if _, err := metamodel.Parse([]byte(doc)); err == nil {
		t.Fatal("a rule scoped to an undeclared face must fail the load rather than " +
			"silently match nothing")
	}
}

// A `faces:` scope on a type that declares NO faces is a different mistake
// from a misspelled face, and needs a different fix — drop the key, rather
// than correct the spelling. The general message would end in "(declares: )",
// which reads like a bug in the error itself.
func TestValidate_FacesScopeOnUnfacedTypeExplainsItself(t *testing.T) {
	doc := `version: "1"
entities:
  note:
    label: Note
    id_prefix: NOTE
    properties:
      title: {type: string}
validations:
  - name: needs-title
    entity_type: note
    faces: [published]
    then: ["title!="]
`
	_, err := metamodel.Parse([]byte(doc))
	if err == nil {
		t.Fatal("a scope on a type with no faces must fail the load")
	}
	if !strings.Contains(err.Error(), "declares no `faces:`") {
		t.Errorf("the error must say the TYPE has no faces, not print an empty "+
			"declared list; got: %v", err)
	}
	if strings.Contains(err.Error(), "(declares: )") {
		t.Errorf("empty declared-list message leaked through: %v", err)
	}
}

func mustCreate(t *testing.T, st *memstore.MemStore, e *entity.Entity) {
	t.Helper()
	if err := st.CreateEntity(t.Context(), e); err != nil {
		t.Fatalf("seed %s@%s: %v", e.ID, e.Face, err)
	}
}
