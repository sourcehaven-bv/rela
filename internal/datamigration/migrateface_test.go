package datamigration

import (
	"maps"
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/audit"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/store/memstore"
)

// facedV1 is metaV1 with `task` gaining a draft/published pair — the flat→faced
// change that classifies as faces_introduced. The `status` enum
// (open/wip/done) is untouched, so it can key the migration.
func facedV1() *metamodel.Metamodel {
	m := metaV1()
	def := m.Entities["task"]
	def.Faces = map[string]metamodel.FaceDef{"draft": {}, "published": {}}
	m.Entities["task"] = def
	return m
}

const migrateTaskFaces = `  - migrate_face:
      entity: task
      property: status
      mapping:
        open: draft
        wip: draft
        done: published
`

// The defect BUG-TMGWIN reports, end to end: adopting faces on a populated type
// must be able to put each row on the face its data says it belongs to, rather
// than leaving every row at the zero coordinate that names no face.
func TestMigrateFace_MovesRowsByPropertyValue(t *testing.T) {
	st := seedStore(t)
	ctx := t.Context()

	r := newTestRunner(t, Deps{Store: st, State: newFakeKV(), Audit: audit.NewMemory()})
	f := mustParse(t, "0001-faces.yaml", mustFileYAML(t, metaV1(), facedV1(), migrateTaskFaces))
	res, err := r.Run(ctx, []*File{f}, true)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	// Three tasks move; PER-1 is a person and must not be touched.
	if got := res.Files[0].Steps[0].Affected; got != 3 {
		t.Errorf("affected = %d, want 3", got)
	}

	// Each row landed on the face its status mapped to, content intact.
	for _, tc := range []struct{ id, face, title string }{
		{"TSK-1", "draft", "one"},       // open
		{"TSK-2", "draft", "two"},       // wip
		{"TSK-3", "published", "three"}, // done
	} {
		got, err := st.GetEntityState(ctx, tc.id, entity.Face(tc.face))
		if err != nil {
			t.Errorf("%s should exist at face %q: %v", tc.id, tc.face, err)
			continue
		}
		if got.Properties["title"] != tc.title {
			t.Errorf("%s lost content on the move: %v", tc.id, got.Properties)
		}
		// And it is no longer at the zero coordinate.
		if _, err := st.GetEntityState(ctx, tc.id, entity.Face("")); err == nil {
			t.Errorf("%s still exists at the zero coordinate — the move left a duplicate", tc.id)
		}
	}
}

// Re-running a migration is the engine's crash-recovery story, so every step
// must converge. A second Run must move nothing and change nothing.
func TestMigrateFace_IsIdempotent(t *testing.T) {
	st := seedStore(t)
	ctx := t.Context()
	body := mustFileYAML(t, metaV1(), facedV1(), migrateTaskFaces)
	r := newTestRunner(t, Deps{Store: st, State: newFakeKV(), Audit: audit.NewMemory()})

	if _, err := r.Run(ctx, []*File{mustParse(t, "0001-faces.yaml", body)}, true); err != nil {
		t.Fatalf("first Run: %v", err)
	}
	res, err := r.Run(ctx, []*File{mustParse(t, "0001-faces.yaml", body)}, true)
	if err != nil {
		t.Fatalf("second Run: %v", err)
	}
	if got := res.Files[0].Steps[0].Affected; got != 0 {
		t.Errorf("second run affected = %d, want 0 — the step is not idempotent", got)
	}
	if _, err := st.GetEntityState(ctx, "TSK-1", entity.Face("draft")); err != nil {
		t.Errorf("TSK-1 lost its face on re-run: %v", err)
	}
}

// The safety property. A value with no face leaves its rows at the zero
// coordinate, where they name no face and become unreachable once the keying
// property is dropped — so it is an authoring error, not an implied default.
func TestMigrateFace_RefusesNonExhaustiveMapping(t *testing.T) {
	steps := `  - migrate_face:
      entity: task
      property: status
      mapping:
        open: draft
        wip: draft
`
	_, err := ParseFile("0001-faces.yaml", mustFileYAML(t, metaV1(), facedV1(), steps))
	if err == nil {
		t.Fatal("expected a parse error: `done` has no face")
	}
	if !strings.Contains(err.Error(), "not exhaustive") || !strings.Contains(err.Error(), "done") {
		t.Errorf("error should name the unmapped value and say the mapping is not exhaustive, got: %v", err)
	}
}

func TestMigrateFace_RejectsBadMappingEntries(t *testing.T) {
	tests := []struct {
		name  string
		steps string
		want  string
	}{
		{
			name: "value is not a declared face",
			steps: `  - migrate_face:
      entity: task
      property: status
      mapping:
        open: nonesuch
        wip: draft
        done: published
`,
			want: "is not a face declared",
		},
		{
			name: "key is not a value of the enum",
			steps: `  - migrate_face:
      entity: task
      property: status
      mapping:
        open: draft
        wip: draft
        done: published
        bogus: draft
`,
			want: "is not a value of",
		},
		{
			name: "property is not an enum",
			steps: `  - migrate_face:
      entity: task
      property: title
      mapping:
        anything: draft
`,
			want: "is not an enum",
		},
		{
			name: "property absent from the from-schema",
			steps: `  - migrate_face:
      entity: task
      property: nosuchprop
      mapping:
        open: draft
`,
			want: "is not in the from-schema",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := ParseFile("0001-faces.yaml", mustFileYAML(t, metaV1(), facedV1(), tc.steps))
			if err == nil {
				t.Fatalf("expected a parse error containing %q", tc.want)
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Errorf("error = %v, want it to contain %q", err, tc.want)
			}
		})
	}
}

// A list-typed enum cannot key the move: a row holding several values has no
// single destination, and Run reads the property as a string, so every row
// would be reported as uncovered. Refuse the question rather than answer it
// wrongly.
func TestMigrateFace_RefusesListTypedKey(t *testing.T) {
	withListStatus := func(m *metamodel.Metamodel) *metamodel.Metamodel {
		def := m.Entities["task"]
		props := maps.Clone(def.Properties)
		props["status"] = metamodel.PropertyDef{Type: "status", List: true}
		def.Properties = props
		m.Entities["task"] = def
		return m
	}
	_, err := ParseFile("0001-faces.yaml",
		mustFileYAML(t, withListStatus(metaV1()), withListStatus(facedV1()), migrateTaskFaces))
	if err == nil {
		t.Fatal("expected a parse error: a list-typed property cannot key the move")
	}
	if !strings.Contains(err.Error(), "list-typed") {
		t.Errorf("error should say the property is list-typed, got: %v", err)
	}
}

// A drop_property before the migrate_face that reads it leaves the step with no
// values to key on, so it would move nothing and silently leave every row at
// the zero coordinate. Refused at parse time.
func TestMigrateFace_RefusesDropPropertyBeforeIt(t *testing.T) {
	to := facedV1()
	def := to.Entities["task"]
	props := maps.Clone(def.Properties)
	delete(props, "status")
	def.Properties = props
	to.Entities["task"] = def

	drop := "  - drop_property: {entity: task, property: status}\n"

	_, err := ParseFile("0001-faces.yaml", mustFileYAML(t, metaV1(), to, drop+migrateTaskFaces))
	if err == nil {
		t.Fatal("expected a parse error: drop_property precedes the migrate_face that reads it")
	}
	if !strings.Contains(err.Error(), "must come first") {
		t.Errorf("error should explain the ordering, got: %v", err)
	}

	// The correct order parses.
	if _, err := ParseFile("0001-faces.yaml", mustFileYAML(t, metaV1(), to, migrateTaskFaces+drop)); err != nil {
		t.Errorf("migrate_face before drop_property should parse, got: %v", err)
	}
}

// Rows whose value is unset, or outside the declared enum, cannot be placed by
// a mapping proven total over the DECLARED set. They stay put and are reported,
// never given a guessed face.
func TestMigrateFace_ReportsRowsOutsideTheDeclaredValueSet(t *testing.T) {
	st := seedStore(t)
	ctx := t.Context()
	if err := st.CreateEntity(ctx, &entity.Entity{
		ID: "TSK-4", Type: "task", Properties: map[string]any{"title": "four"},
	}); err != nil {
		t.Fatalf("seed: %v", err)
	}

	r := newTestRunner(t, Deps{Store: st, State: newFakeKV(), Audit: audit.NewMemory()})
	f := mustParse(t, "0001-faces.yaml", mustFileYAML(t, metaV1(), facedV1(), migrateTaskFaces))
	res, err := r.Run(ctx, []*File{f}, true)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	notes := strings.Join(res.Files[0].Steps[0].Notes, "\n")
	if !strings.Contains(notes, "not covered by the mapping") {
		t.Errorf("expected a note about the uncovered row, got: %q", notes)
	}
	if _, err := st.GetEntityState(ctx, "TSK-4", entity.Face("")); err != nil {
		t.Errorf("TSK-4 should have been left in place: %v", err)
	}
}

// Dry-run must count without writing — the operator's preview of a step that
// physically moves rows.
func TestMigrateFace_DryRunMovesNothing(t *testing.T) {
	st := seedStore(t)
	ctx := t.Context()
	r := newTestRunner(t, Deps{Store: st, State: newFakeKV(), Audit: audit.NewMemory()})
	f := mustParse(t, "0001-faces.yaml", mustFileYAML(t, metaV1(), facedV1(), migrateTaskFaces))
	res, err := r.Run(ctx, []*File{f}, false)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if got := res.Files[0].Steps[0].Affected; got != 3 {
		t.Errorf("dry-run affected = %d, want 3", got)
	}
	if _, err := st.GetEntityState(ctx, "TSK-1", entity.Face("draft")); err == nil {
		t.Error("dry-run wrote a row")
	}
}

// The enforcement that actually closes BUG-TMGWIN. A file spanning the delta
// with no migrate_face step is the do-nothing migration that caused the bug: it
// parsed, applied, advanced the marker and reported the schema in sync.
func TestParseFile_RefusesUnmigratedFaceAdoption(t *testing.T) {
	_, err := ParseFile("0001-faces.yaml", mustFileYAML(t, metaV1(), facedV1(), "  []\n"))
	if err == nil {
		t.Fatal("expected a parse error: the file spans a faces_introduced edge and migrates nothing")
	}
	if !strings.Contains(err.Error(), "migrate_face") {
		t.Errorf("error should name the missing step, got: %v", err)
	}

	// A step for a DIFFERENT entity does not satisfy the delta either.
	toMeta := facedV1()
	pdef := toMeta.Entities["person"]
	pdef.Faces = map[string]metamodel.FaceDef{"draft": {}, "published": {}}
	toMeta.Entities["person"] = pdef
	_, err = ParseFile("0002-faces.yaml", mustFileYAML(t, metaV1(), toMeta, migrateTaskFaces))
	if err == nil {
		t.Fatal("expected a parse error: person's adoption is unmigrated")
	}
	if !strings.Contains(err.Error(), `"person"`) {
		t.Errorf("error should name the unmigrated entity, got: %v", err)
	}
}

// The mapping between what CompareShapes can DEMAND and what the step
// vocabulary can DELIVER must stay complete. A new TierMigration delta kind
// that nobody lists here would otherwise ship detection with no remediation and
// no exemption — the shape of BUG-TMGWIN's root cause
// (AM-migration-delta-kinds-have-resolving-steps).
func TestResolvingSteps_CoversEveryMigrationDeltaKind(t *testing.T) {
	for _, kind := range metamodel.MigrationDeltaKinds() {
		if _, listed := resolvingSteps[kind]; !listed {
			t.Errorf("delta kind %q is classified TierMigration but is not in resolvingSteps — "+
				"either name a step that resolves it, or list it with an empty value and say why", kind)
		}
	}
}

// The generator must draft a migrate_face step for a faces_introduced delta
// rather than the do-nothing file that caused BUG-TMGWIN.
func TestGenerate_DraftsMigrateFaceForFacesIntroduced(t *testing.T) {
	draft, err := Generate(metaV1().ShapeProjection(), facedV1().ShapeProjection(), nil, "adopt faces")
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if draft == nil {
		t.Fatal("expected a draft for a faces_introduced delta")
	}
	got := string(draft.Content)
	if !strings.Contains(got, "migrate_face:") {
		t.Fatalf("draft has no migrate_face step:\n%s", got)
	}
	// Every enum value is pre-listed so the operator cannot silently omit one.
	for _, v := range []string{"open", "wip", "done"} {
		if !strings.Contains(got, v+": CHANGEME") {
			t.Errorf("draft does not pre-list %q for the operator to fill in:\n%s", v, got)
		}
	}
	if !strings.Contains(got, "TODO") {
		t.Errorf("the face step must be marked TODO, not applied blind:\n%s", got)
	}
	// Emitted LIVE: a commented step could not satisfy validateDeltasResolved.
	if strings.Contains(got, "# - migrate_face:") {
		t.Errorf("the step must be live, not commented:\n%s", got)
	}
}

// An unedited draft must NOT apply: CHANGEME is not a declared face, so the
// operator has to make a real choice before anything runs. This is the
// generator's half of the BUG-TMGWIN fix.
func TestGenerate_FaceDraftDoesNotApplyUnedited(t *testing.T) {
	draft, err := Generate(metaV1().ShapeProjection(), facedV1().ShapeProjection(), nil, "adopt faces")
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	_, err = ParseFile(draft.FileName, draft.Content)
	if err == nil {
		t.Fatal("an unedited draft must not parse — CHANGEME is not a face")
	}
	if !strings.Contains(err.Error(), "CHANGEME") {
		t.Errorf("the error should point at the placeholder, got: %v", err)
	}
}

// When no enum property can key the move, the generator declines rather than
// guessing, and says what the operator must supply.
func TestGenerate_FaceDraftWithoutEnumExplainsItself(t *testing.T) {
	toMeta := metaV1()
	pdef := toMeta.Entities["person"] // person has only `name`, no enum
	pdef.Faces = map[string]metamodel.FaceDef{"draft": {}, "published": {}}
	toMeta.Entities["person"] = pdef

	draft, err := Generate(metaV1().ShapeProjection(), toMeta.ShapeProjection(), nil, "adopt faces")
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if draft == nil {
		t.Fatal("expected a draft")
	}
	if got := string(draft.Content); !strings.Contains(got, "No enum property was found") {
		t.Errorf("draft should explain why it could not draft a migrate_face:\n%s", got)
	}
}

// Sanity-check the premise the step now rests on: since BUG-HC6I2T a row CAN
// leave the zero coordinate. Before that change two row-family invariants made
// this impossible, which is why the step could only confirm rather than move.
// If this ever fails, migrate_face's whole design is back in play.
func TestFaces_RowCanLeaveTheZeroCoordinate(t *testing.T) {
	st := memstore.New()
	ctx := t.Context()
	if err := st.CreateEntity(ctx, &entity.Entity{
		ID: "TSK-9", Type: "task", Properties: map[string]any{"title": "one"},
	}); err != nil {
		t.Fatalf("seed zero-coordinate row: %v", err)
	}
	if err := st.CreateEntity(ctx, &entity.Entity{
		ID: "TSK-9", Type: "task", Face: entity.Face("draft"), Properties: map[string]any{"title": "one"},
	}); err != nil {
		t.Fatalf("create at a named face: %v", err)
	}
	if _, err := st.DeleteEntityState(ctx, "TSK-9", entity.Face("")); err != nil {
		t.Fatalf("the zero-coordinate row must be deletable once the row lives at a face: %v", err)
	}
	if _, err := st.GetEntityState(ctx, "TSK-9", entity.Face("draft")); err != nil {
		t.Errorf("the moved row should survive: %v", err)
	}
}
