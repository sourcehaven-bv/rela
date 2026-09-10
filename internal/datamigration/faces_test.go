package datamigration

import (
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/audit"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
)

// facedMeta is metaV1 with `task` declaring content states, so a rename_face
// step has something to move.
func facedMeta(faces ...string) *metamodel.Metamodel {
	m := metaV1()
	def := m.Entities["task"]
	def.Faces = map[string]metamodel.FaceDef{}
	for _, f := range faces {
		def.Faces[f] = metamodel.FaceDef{}
	}
	m.Entities["task"] = def
	return m
}

// A migration must reach EVERY content state, not just the bare row
// (TKT-O0A8FO). Before this, the query left AllStates at its zero value —
// "default-state rows only" — so a project using faces got half-migrated:
// the bare row rewritten, every `@nl` row silently left on the old schema.
//
// This is the flag's documented purpose. A migration rewrites storage truth,
// and every row of a state family conforms to the same schema; choosing ONE
// face is a read-path concern (worlds) that has no business here.
func TestRunner_MigratesEveryContentState(t *testing.T) {
	st := seedStore(t)
	ctx := t.Context()

	// A second face of an existing task, carrying the SAME v1 shape: the old
	// `status` key with a value the migration remaps.
	if err := st.CreateEntity(ctx, &entity.Entity{
		ID: "TSK-1", Type: "task", Face: entity.Face("nl"),
		Properties: map[string]any{"title": "een", "status": "open", "due": "01/02/2026"},
	}); err != nil {
		t.Fatalf("seed face row: %v", err)
	}

	r := newTestRunner(t, Deps{Store: st, State: newFakeKV(), Audit: audit.NewMemory()})
	f := mustParse(t, "0001-test.yaml", mustFileYAML(t, metaV1(), metaV2(), v1ToV2Steps))
	if _, err := r.Run(ctx, []*File{f}, true); err != nil {
		t.Fatalf("Run: %v", err)
	}

	// The bare row migrated, as it always did.
	bare := getEntity(t, st, "TSK-1")
	if got := bare.Properties["state"]; got != "todo" {
		t.Errorf("bare row state = %v, want todo", got)
	}

	// And so did the face — the actual regression this pins.
	face, err := st.GetEntityState(ctx, "TSK-1", entity.Face("nl"))
	if err != nil {
		t.Fatalf("GetEntityState: %v", err)
	}
	if got := face.Properties["state"]; got != "todo" {
		t.Errorf("nl face state = %v, want todo — the face row was skipped", got)
	}
	if _, has := face.Properties["status"]; has {
		t.Error("nl face still carries the old `status` key")
	}
	if got := face.Properties["due"]; got != "2026-01-02" {
		t.Errorf("nl face due = %v, want the converted date", got)
	}

	// The family is intact: updating a face must not collapse it onto the
	// bare row, nor clobber the bare row's own content.
	if got := bare.Properties["title"]; got != "one" {
		t.Errorf("bare title = %v, want one — the face write leaked onto it", got)
	}
	if got := face.Properties["title"]; got != "een" {
		t.Errorf("face title = %v, want een", got)
	}
}

// rename_face rewrites a stored coordinate in place. A face is NOT part of the
// id, so this is the same shape of operation as rename_entity_type — no id
// rewrite, relations untouched (TKT-O0A8FO).
func TestRenameFace_MovesRowsToTheNewCoordinate(t *testing.T) {
	st := seedStore(t)
	ctx := t.Context()
	if err := st.CreateEntity(ctx, &entity.Entity{
		ID: "TSK-1", Type: "task", Face: entity.Face("nl"),
		Properties: map[string]any{"title": "een"},
	}); err != nil {
		t.Fatalf("seed face: %v", err)
	}

	r := newTestRunner(t, Deps{Store: st, State: newFakeKV(), Audit: audit.NewMemory()})
	f := mustParse(t, "0001-face.yaml", mustFileYAML(t,
		facedMeta("en", "nl"),
		facedMeta("en", "nl-BE"),
		"  - rename_face: {entity: task, from: nl, to: nl-BE}\n"))
	if _, err := r.Run(ctx, []*File{f}, true); err != nil {
		t.Fatalf("Run: %v", err)
	}

	moved, err := st.GetEntityState(ctx, "TSK-1", entity.Face("nl-BE"))
	if err != nil || moved == nil {
		t.Fatalf("row did not move to nl-BE: %v", err)
	}
	if got := moved.Properties["title"]; got != "een" {
		t.Errorf("content changed during the move: %v", got)
	}
	if old, err := st.GetEntityState(ctx, "TSK-1", entity.Face("nl")); err == nil && old != nil {
		t.Error("the old coordinate still holds a row — the rename duplicated instead of moving")
	}
	// The bare row is a different member of the family and must be untouched.
	if bare := getEntity(t, st, "TSK-1"); bare.Properties["title"] != "one" {
		t.Errorf("bare row was disturbed: %v", bare.Properties["title"])
	}
}

// A rename onto an OCCUPIED coordinate is a collision and must be refused:
// the entity holds different content at both faces, so completing the move
// would destroy one of them. (The convergence case below is the one exception,
// where the destination holds the source's OWN content from a crashed run.)
func TestRenameFace_OntoAnOccupiedCoordinateIsRefused(t *testing.T) {
	st := seedStore(t)
	ctx := t.Context()
	for face, title := range map[string]string{"nl": "twee", "nl-BE": "anders"} {
		if err := st.CreateEntity(ctx, &entity.Entity{
			ID: "TSK-2", Type: "task", Face: entity.Face(face),
			Properties: map[string]any{"title": title},
		}); err != nil {
			t.Fatalf("seed %s: %v", face, err)
		}
	}

	r := newTestRunner(t, Deps{Store: st, State: newFakeKV(), Audit: audit.NewMemory()})
	f := mustParse(t, "0001-face.yaml", mustFileYAML(t,
		facedMeta("en", "nl", "nl-BE"),
		facedMeta("en", "nl-BE"),
		"  - rename_face: {entity: task, from: nl, to: nl-BE}\n"))

	_, err := r.Run(ctx, []*File{f}, true)
	if err == nil {
		t.Fatal("a rename onto an occupied coordinate must fail rather than " +
			"silently destroy one of the two rows")
	}
	if !strings.Contains(err.Error(), "already exists at the destination") {
		t.Errorf("the error must name the collision so the operator can resolve it; got: %v", err)
	}

	// Both rows survive the refusal — nothing is half-applied.
	src, serr := st.GetEntityState(ctx, "TSK-2", entity.Face("nl"))
	if serr != nil || src.Properties["title"] != "twee" {
		t.Errorf("nl row lost or altered: %v (%v)", src, serr)
	}
	dst, derr := st.GetEntityState(ctx, "TSK-2", entity.Face("nl-BE"))
	if derr != nil || dst.Properties["title"] != "anders" {
		t.Errorf("nl-BE row lost or altered: %v (%v)", dst, derr)
	}
}

// A crash between the create and the delete leaves the row at BOTH
// coordinates. The re-run must converge — that is the engine's whole crash
// recovery — so a destination row holding the source's content counts as
// already moved, and only the source is deleted.
func TestRenameFace_ReRunConvergesAfterACrash(t *testing.T) {
	st := seedStore(t)
	ctx := t.Context()
	for _, face := range []string{"nl", "nl-BE"} {
		if err := st.CreateEntity(ctx, &entity.Entity{
			ID: "TSK-1", Type: "task", Face: entity.Face(face),
			Properties: map[string]any{"title": "een"},
		}); err != nil {
			t.Fatalf("seed %s: %v", face, err)
		}
	}

	r := newTestRunner(t, Deps{Store: st, State: newFakeKV(), Audit: audit.NewMemory()})
	f := mustParse(t, "0001-face.yaml", mustFileYAML(t,
		facedMeta("en", "nl"),
		facedMeta("en", "nl-BE"),
		"  - rename_face: {entity: task, from: nl, to: nl-BE}\n"))
	if _, err := r.Run(ctx, []*File{f}, true); err != nil {
		t.Fatalf("a re-run over the previous run's own copy must converge, not refuse: %v", err)
	}
	if old, err := st.GetEntityState(ctx, "TSK-1", entity.Face("nl")); err == nil && old != nil {
		t.Error("the source row survived the re-run")
	}
	moved, err := st.GetEntityState(ctx, "TSK-1", entity.Face("nl-BE"))
	if err != nil || moved.Properties["title"] != "een" {
		t.Errorf("the destination row must hold the content: %v %v", moved, err)
	}
}
