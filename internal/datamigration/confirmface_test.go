package datamigration

import (
	"maps"
	"slices"
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/audit"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/store/memstore"
)

// facedV1 is metaV1 with `task` gaining a draft/published pair and `published`
// as the bare face — the flat→faced change that classifies as
// bare_face_introduced. The `status` enum (open/wip/done) is untouched, so it
// can key the confirmation.
func facedV1() *metamodel.Metamodel {
	m := metaV1()
	def := m.Entities["task"]
	def.Faces = map[string]metamodel.FaceDef{"draft": {}, "published": {}}
	def.BareFace = "published"
	m.Entities["task"] = def
	return m
}

const confirmAllPublished = `  - confirm_face:
      entity: task
      property: status
      mapping:
        open: published
        wip: published
        done: published
`

// The premise the whole step is built on: an entity ALWAYS occupies the bare
// coordinate. A named-face row cannot exist without a default row, and a
// default row cannot be deleted while a sibling remains — so a row can never
// be moved off the bare face, in either order.
//
// This is the store invariant (TKT-DOFYR1) that makes "assign each row to the
// face its status implies" unimplementable, and it is why confirm_face
// confirms rather than moves. If this test ever fails, the step's whole
// rationale is back in play.
func TestFaces_RowCannotLeaveTheBareCoordinate(t *testing.T) {
	st := memstore.New()
	ctx := t.Context()
	if err := st.CreateEntity(ctx, &entity.Entity{
		ID: "TSK-9", Type: "task", Properties: map[string]any{"title": "one"},
	}); err != nil {
		t.Fatalf("seed bare row: %v", err)
	}

	// Create-then-delete: the sibling makes the bare row undeletable.
	if err := st.CreateEntity(ctx, &entity.Entity{
		ID: "TSK-9", Type: "task", Face: entity.Face("draft"), Properties: map[string]any{"title": "one"},
	}); err != nil {
		t.Fatalf("named face alongside the bare row should be allowed: %v", err)
	}
	if _, err := st.DeleteEntityState(ctx, "TSK-9", entity.Face("")); err == nil {
		t.Fatal("expected the store to refuse deleting the default row while a sibling exists")
	}

	// Delete-then-create: with no default row, a named row is headless.
	st2 := memstore.New()
	if err := st2.CreateEntity(ctx, &entity.Entity{
		ID: "TSK-9", Type: "task", Properties: map[string]any{"title": "one"},
	}); err != nil {
		t.Fatalf("seed: %v", err)
	}
	if _, err := st2.DeleteEntityState(ctx, "TSK-9", entity.Face("")); err != nil {
		t.Fatalf("deleting the only row should be allowed: %v", err)
	}
	if err := st2.CreateEntity(ctx, &entity.Entity{
		ID: "TSK-9", Type: "task", Face: entity.Face("draft"), Properties: map[string]any{"title": "one"},
	}); err == nil {
		t.Fatal("expected the store to refuse a headless named-face row")
	}
}

// The step writes nothing — the relabel is the schema change itself — but it
// must account for the rows it is confirming.
func TestConfirmFace_ReportsTheRowsItConfirms(t *testing.T) {
	st := seedStore(t)
	ctx := t.Context()

	r := newTestRunner(t, Deps{Store: st, State: newFakeKV(), Audit: audit.NewMemory()})
	f := mustParse(t, "0001-faces.yaml",
		mustFileYAML(t, metaV1(), facedV1(), confirmAllPublished))
	res, err := r.Run(ctx, []*File{f}, true)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	// Affected must stay zero: the run report renders it as "changed N
	// record(s)" and this step writes nothing.
	if got := res.Files[0].Steps[0].Affected; got != 0 {
		t.Errorf("affected = %d, want 0 — confirm_face writes nothing", got)
	}
	// The count belongs in a note. Three tasks; PER-1 is a person.
	notes := strings.Join(res.Files[0].Steps[0].Notes, "\n")
	if !strings.Contains(notes, "confirmed 3 row(s)") {
		t.Errorf("expected a note counting the confirmed rows, got: %q", notes)
	}
	// Nothing moved: every task is still at the bare coordinate.
	for _, id := range []string{"TSK-1", "TSK-2", "TSK-3"} {
		if _, err := st.GetEntityState(ctx, id, entity.Face("")); err != nil {
			t.Errorf("%s should still be at the bare coordinate: %v", id, err)
		}
	}
}

// Re-running a migration is the engine's crash-recovery story, so every step
// must converge. A step that writes nothing is trivially idempotent; this pins
// that it also reports the same thing twice.
func TestConfirmFace_IsIdempotent(t *testing.T) {
	st := seedStore(t)
	ctx := t.Context()
	body := mustFileYAML(t, metaV1(), facedV1(), confirmAllPublished)
	r := newTestRunner(t, Deps{Store: st, State: newFakeKV(), Audit: audit.NewMemory()})

	first, err := r.Run(ctx, []*File{mustParse(t, "0001-faces.yaml", body)}, true)
	if err != nil {
		t.Fatalf("first Run: %v", err)
	}
	second, err := r.Run(ctx, []*File{mustParse(t, "0001-faces.yaml", body)}, true)
	if err != nil {
		t.Fatalf("second Run: %v", err)
	}
	if !slices.Equal(first.Files[0].Steps[0].Notes, second.Files[0].Steps[0].Notes) {
		t.Errorf("re-run reported %v, first run %v — not idempotent",
			second.Files[0].Steps[0].Notes, first.Files[0].Steps[0].Notes)
	}
}

// The safety property this step exists for. A value with no entry is exactly
// the case whose rows silently became something they were not, so it is an
// authoring error rather than an implied default.
func TestConfirmFace_RefusesNonExhaustiveMapping(t *testing.T) {
	steps := `  - confirm_face:
      entity: task
      property: status
      mapping:
        open: published
        wip: published
`
	_, err := ParseFile("0001-faces.yaml",
		mustFileYAML(t, metaV1(), facedV1(), steps))
	if err == nil {
		t.Fatal("expected a parse error: `done` has no entry")
	}
	if !strings.Contains(err.Error(), "not exhaustive") || !strings.Contains(err.Error(), "done") {
		t.Errorf("error should name the unmapped value and say the mapping is not exhaustive, got: %v", err)
	}
}

// Mapping a value to a face that is NOT the bare face is a promise the store
// cannot keep. The error must say so, and say what to do instead — this is the
// moment the operator learns the schema does not fit the data.
func TestConfirmFace_RefusesNonBareTargetWithGuidance(t *testing.T) {
	steps := `  - confirm_face:
      entity: task
      property: status
      mapping:
        open: draft
        wip: published
        done: published
`
	_, err := ParseFile("0001-faces.yaml",
		mustFileYAML(t, metaV1(), facedV1(), steps))
	if err == nil {
		t.Fatal("expected a parse error: rows cannot land on a non-bare face")
	}
	msg := err.Error()
	if !strings.Contains(msg, "cannot be moved off the bare coordinate") {
		t.Errorf("error should explain the store constraint, got: %v", err)
	}
	if !strings.Contains(msg, "bare_face") {
		t.Errorf("error should point at the schema fix, got: %v", err)
	}
}

func TestConfirmFace_RejectsBadMappingEntries(t *testing.T) {
	tests := []struct {
		name  string
		steps string
		want  string
	}{
		{
			name: "value is not a declared face",
			steps: `  - confirm_face:
      entity: task
      property: status
      mapping:
        open: nonesuch
        wip: published
        done: published
`,
			want: "is not a face declared",
		},
		{
			name: "key is not a value of the enum",
			steps: `  - confirm_face:
      entity: task
      property: status
      mapping:
        open: published
        wip: published
        done: published
        bogus: published
`,
			want: "is not a value of",
		},
		{
			name: "property is not an enum",
			steps: `  - confirm_face:
      entity: task
      property: title
      mapping:
        anything: published
`,
			want: "is not an enum",
		},
		{
			name: "property absent from the from-schema",
			steps: `  - confirm_face:
      entity: task
      property: nosuchprop
      mapping:
        open: published
`,
			want: "is not in the from-schema",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := ParseFile("0001-faces.yaml",
				mustFileYAML(t, metaV1(), facedV1(), tc.steps))
			if err == nil {
				t.Fatalf("expected a parse error containing %q", tc.want)
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Errorf("error = %v, want it to contain %q", err, tc.want)
			}
		})
	}
}

// A drop_property before the confirm_face that reads it leaves the step
// reporting on nothing, which turns an informed confirmation back into the
// empty ceremony it exists to replace. Refused at parse time.
func TestConfirmFace_RefusesDropPropertyBeforeIt(t *testing.T) {
	// A to-schema that both adds faces and removes `status`, so the
	// drop_property is itself valid.
	to := facedV1()
	def := to.Entities["task"]
	props := map[string]metamodel.PropertyDef{}
	for k, v := range def.Properties {
		if k != "status" {
			props[k] = v
		}
	}
	def.Properties = props
	to.Entities["task"] = def

	drop := "  - drop_property: {entity: task, property: status}\n"

	_, err := ParseFile("0001-faces.yaml", mustFileYAML(t, metaV1(), to, drop+confirmAllPublished))
	if err == nil {
		t.Fatal("expected a parse error: drop_property precedes the confirm_face that reads it")
	}
	if !strings.Contains(err.Error(), "must come first") {
		t.Errorf("error should explain the ordering, got: %v", err)
	}

	// The correct order parses.
	if _, err := ParseFile("0001-faces.yaml", mustFileYAML(t, metaV1(), to, confirmAllPublished+drop)); err != nil {
		t.Errorf("confirm_face before drop_property should parse, got: %v", err)
	}
}

// Rows whose value is unset, or outside the declared enum, cannot be covered by
// a mapping proven total over the DECLARED set. They are pre-existing invalid
// data: reported, never silently folded into the confirmation.
func TestConfirmFace_ReportsRowsOutsideTheDeclaredValueSet(t *testing.T) {
	st := seedStore(t)
	ctx := t.Context()
	if err := st.CreateEntity(ctx, &entity.Entity{
		ID: "TSK-4", Type: "task", Properties: map[string]any{"title": "four"},
	}); err != nil {
		t.Fatalf("seed: %v", err)
	}

	r := newTestRunner(t, Deps{Store: st, State: newFakeKV(), Audit: audit.NewMemory()})
	f := mustParse(t, "0001-faces.yaml",
		mustFileYAML(t, metaV1(), facedV1(), confirmAllPublished))
	res, err := r.Run(ctx, []*File{f}, true)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	notes := strings.Join(res.Files[0].Steps[0].Notes, "\n")
	if !strings.Contains(notes, "not covered by the mapping") {
		t.Errorf("expected a note about the uncovered row, got: %q", notes)
	}
}

// The generator must draft a confirm_face skeleton for a bare_face delta rather
// than the do-nothing file that caused BUG-TMGWIN.
func TestGenerate_DraftsConfirmFaceForBareFaceIntroduced(t *testing.T) {
	draft, err := Generate(metaV1().ShapeProjection(),
		facedV1().ShapeProjection(), nil, "adopt faces")
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if draft == nil {
		t.Fatal("expected a draft for a bare_face_introduced delta")
	}
	got := string(draft.Content)
	if !strings.Contains(got, "confirm_face:") {
		t.Fatalf("draft has no confirm_face step:\n%s", got)
	}
	// Every value is pre-listed so the operator must look at each one.
	for _, v := range []string{"open", "wip", "done"} {
		if !strings.Contains(got, v+": published") {
			t.Errorf("draft does not pre-list %q with the face its rows become:\n%s", v, got)
		}
	}
	if !strings.Contains(got, "TODO") {
		t.Errorf("the face step must be marked TODO, not applied blind:\n%s", got)
	}
	// Emitted LIVE, not commented. validateDeltasResolved refuses a file
	// spanning this delta without a confirm_face step, so a commented skeleton
	// would be a draft that cannot parse.
	if strings.Contains(got, "# - confirm_face:") {
		t.Errorf("the step must be live, not commented — a commented draft cannot parse:\n%s", got)
	}
}

// Generate round-trips its own draft through ParseFile, so the emitted skeleton
// must never make the generator itself fail.
func TestGenerate_FaceDraftRoundTripsThroughParse(t *testing.T) {
	draft, err := Generate(metaV1().ShapeProjection(),
		facedV1().ShapeProjection(), nil, "adopt faces")
	if err != nil {
		t.Fatalf("Generate must not fail on its own draft: %v", err)
	}
	if _, err := ParseFile(draft.FileName, draft.Content); err != nil {
		t.Fatalf("generated draft does not parse: %v\n%s", err, draft.Content)
	}
}

// When no enum property can key the confirmation, the generator declines rather
// than guessing, and says what the operator must check by hand.
func TestGenerate_FaceDraftWithoutEnumExplainsItself(t *testing.T) {
	toMeta := metaV1()
	pdef := toMeta.Entities["person"] // person has only `name`, no enum
	pdef.Faces = map[string]metamodel.FaceDef{"draft": {}, "published": {}}
	pdef.BareFace = "published"
	toMeta.Entities["person"] = pdef

	draft, err := Generate(metaV1().ShapeProjection(), toMeta.ShapeProjection(), nil, "adopt faces")
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if draft == nil {
		t.Fatal("expected a draft")
	}
	got := string(draft.Content)
	if !strings.Contains(got, "No enum property was found") {
		t.Errorf("draft should explain why there is no per-value check:\n%s", got)
	}
	// It must still emit a real step: the file would not parse without one.
	if !strings.Contains(got, "- confirm_face: {entity: person}") {
		t.Errorf("draft should emit the whole-type confirm_face form:\n%s", got)
	}
	if _, err := ParseFile(draft.FileName, draft.Content); err != nil {
		t.Errorf("the no-enum draft must parse: %v", err)
	}
}

// The enforcement that actually closes BUG-TMGWIN. A file spanning the delta
// with no confirm_face step is the do-nothing migration that caused the bug:
// it parsed, applied, advanced the marker and reported the schema in sync.
func TestParseFile_RefusesUnconfirmedBareFaceAdoption(t *testing.T) {
	_, err := ParseFile("0001-faces.yaml", mustFileYAML(t, metaV1(), facedV1(), "  []\n"))
	if err == nil {
		t.Fatal("expected a parse error: the file spans a bare_face_introduced edge and confirms nothing")
	}
	if !strings.Contains(err.Error(), "confirm_face") {
		t.Errorf("error should name the missing step, got: %v", err)
	}

	// A step for a DIFFERENT entity does not satisfy the delta either.
	toMeta := facedV1()
	pdef := toMeta.Entities["person"]
	pdef.Faces = map[string]metamodel.FaceDef{"draft": {}, "published": {}}
	pdef.BareFace = "published"
	toMeta.Entities["person"] = pdef
	_, err = ParseFile("0002-faces.yaml", mustFileYAML(t, metaV1(), toMeta, confirmAllPublished))
	if err == nil {
		t.Fatal("expected a parse error: person's adoption is unconfirmed")
	}
	if !strings.Contains(err.Error(), `"person"`) {
		t.Errorf("error should name the unconfirmed entity, got: %v", err)
	}
}

// The mapping between what CompareShapes can DEMAND and what the step
// vocabulary can DELIVER must stay complete. A new TierMigration delta kind
// that nobody lists here would otherwise ship detection with no remediation
// and no exemption — the shape of BUG-TMGWIN's root cause
// (AM-migration-delta-kinds-have-resolving-steps).
func TestResolvingSteps_CoversEveryMigrationDeltaKind(t *testing.T) {
	for _, kind := range metamodel.MigrationDeltaKinds() {
		if _, listed := resolvingSteps[kind]; !listed {
			t.Errorf("delta kind %q is classified TierMigration but is not in resolvingSteps — "+
				"either name a step that resolves it, or list it with an empty value and say why", kind)
		}
	}
}

// A list-typed enum cannot key a confirmation: Run reads the property as a
// string, so every row would land in the unaccounted bucket and be reported as
// uncovered. Refuse the question rather than answer it wrongly.
func TestConfirmFace_RefusesListTypedKey(t *testing.T) {
	from := metaV1()
	def := from.Entities["task"]
	props := maps.Clone(def.Properties)
	props["status"] = metamodel.PropertyDef{Type: "status", List: true}
	def.Properties = props
	from.Entities["task"] = def

	to := facedV1()
	tdef := to.Entities["task"]
	tprops := maps.Clone(tdef.Properties)
	tprops["status"] = metamodel.PropertyDef{Type: "status", List: true}
	tdef.Properties = tprops
	to.Entities["task"] = tdef

	_, err := ParseFile("0001-faces.yaml", mustFileYAML(t, from, to, confirmAllPublished))
	if err == nil {
		t.Fatal("expected a parse error: a list-typed property cannot key a confirmation")
	}
	if !strings.Contains(err.Error(), "list-typed") {
		t.Errorf("error should say the property is list-typed, got: %v", err)
	}
}

// The whole-type form, for a type with no enum to key on.
func TestConfirmFace_WholeTypeForm(t *testing.T) {
	steps := "  - confirm_face: {entity: task}\n"
	f, err := ParseFile("0001-faces.yaml", mustFileYAML(t, metaV1(), facedV1(), steps))
	if err != nil {
		t.Fatalf("the whole-type form should parse: %v", err)
	}

	st := seedStore(t)
	r := newTestRunner(t, Deps{Store: st, State: newFakeKV(), Audit: audit.NewMemory()})
	res, err := r.Run(t.Context(), []*File{f}, true)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	notes := strings.Join(res.Files[0].Steps[0].Notes, "\n")
	if !strings.Contains(notes, "confirmed 3 row(s)") {
		t.Errorf("expected the confirmed count, got: %q", notes)
	}

	// property without mapping, and vice versa, are both incoherent.
	for _, bad := range []string{
		"  - confirm_face: {entity: task, property: status}\n",
		"  - confirm_face:\n      entity: task\n      mapping:\n        open: published\n",
	} {
		if _, err := ParseFile("0001-faces.yaml", mustFileYAML(t, metaV1(), facedV1(), bad)); err == nil {
			t.Errorf("expected a parse error for a half-specified step: %s", bad)
		}
	}
}

// A confirmation covering no rows is vacuous and must not look like a
// successful one.
func TestConfirmFace_EmptyStoreSaysNothingToConfirm(t *testing.T) {
	st := memstore.New()
	r := newTestRunner(t, Deps{Store: st, State: newFakeKV(), Audit: audit.NewMemory()})
	f := mustParse(t, "0001-faces.yaml", mustFileYAML(t, metaV1(), facedV1(), confirmAllPublished))
	res, err := r.Run(t.Context(), []*File{f}, true)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	notes := strings.Join(res.Files[0].Steps[0].Notes, "\n")
	if !strings.Contains(notes, "nothing to confirm") {
		t.Errorf("an empty store should say so plainly, got: %q", notes)
	}
}
