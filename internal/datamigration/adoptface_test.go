package datamigration

import (
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/audit"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// adoptDeps builds AdoptDeps over a memstore with the faced schema.
func adoptDeps(t *testing.T, st store.Store, faces ...string) AdoptDeps {
	t.Helper()
	return AdoptDeps{
		Store: st,
		Meta:  facedMeta(faces...),
		Audit: audit.NewMemory(),
		Lock:  NewProcessLock(),
	}
}

// The repair this command exists for: rows stranded at the zero coordinate on
// a type that ALREADY declares faces. No shape change is involved, so no
// migration file could carry a migrate_face step (its Validate proves the
// faces are new in the to-shape), which is why analyze's remedy text used to
// name a step that would refuse the job.
func TestAdopt_MovesStrandedRowsOntoFaces(t *testing.T) {
	st := seedStore(t)
	ctx := t.Context()

	res, err := Adopt(ctx, adoptDeps(t, st, "draft", "published"), AdoptRequest{
		Entity:   "task",
		Property: "status",
		Mapping:  map[string]string{"open": "draft", "wip": "draft", "done": "published"},
		Apply:    true,
	})
	if err != nil {
		t.Fatalf("Adopt: %v", err)
	}
	if res.Affected != 3 {
		t.Fatalf("affected = %d, want 3", res.Affected)
	}

	// The move is real: the row exists at the face and the zero coordinate is
	// empty. A half-move that left both would be the bug create-then-delete
	// exists to make recoverable, not acceptable.
	for id, face := range map[string]string{"TSK-1": "draft", "TSK-2": "draft", "TSK-3": "published"} {
		moved, err := st.GetEntityState(ctx, id, entity.Face(face))
		if err != nil || moved == nil {
			t.Fatalf("%s not at face %q: %v", id, face, err)
		}
		if bare, err := st.GetEntityState(ctx, id, ""); err == nil && bare != nil {
			t.Errorf("%s still has a zero-coordinate row", id)
		}
	}
	// A type without faces is not stranded and must not be touched.
	if _, err := st.GetEntityState(ctx, "PER-1", ""); err != nil {
		t.Errorf("person row was moved: %v", err)
	}
}

// Dry-run is the default, and it must be honest: the count an operator reviews
// is the count that will move, computed by the same planner the apply uses.
func TestAdopt_DryRunCountsWithoutMoving(t *testing.T) {
	st := seedStore(t)
	ctx := t.Context()

	res, err := Adopt(ctx, adoptDeps(t, st, "draft"), AdoptRequest{
		Entity:   "task",
		Property: "status",
		Mapping:  map[string]string{"open": "draft", "wip": "draft", "done": "draft"},
	})
	if err != nil {
		t.Fatalf("Adopt: %v", err)
	}
	if res.Affected != 3 {
		t.Fatalf("affected = %d, want 3", res.Affected)
	}
	if res.Applied {
		t.Error("Applied set on a dry run")
	}
	if bare, err := st.GetEntityState(ctx, "TSK-1", ""); err != nil || bare == nil {
		t.Error("dry run moved a row")
	}
}

// Unlike migrate_face, the mapping need NOT be exhaustive. That step runs in
// the same file as the drop_property that erases the keying values, so an
// omission is unrecoverable; here nothing is dropped, so an omitted value
// leaves its rows exactly where they already were — still stranded, still
// reported, still fixable by re-running with a wider mapping. Refusing would
// only stop the operator fixing the rows they CAN place.
func TestAdopt_PartialMappingMovesWhatItCanAndReportsTheRest(t *testing.T) {
	st := seedStore(t)
	ctx := t.Context()

	res, err := Adopt(ctx, adoptDeps(t, st, "draft"), AdoptRequest{
		Entity:   "task",
		Property: "status",
		Mapping:  map[string]string{"open": "draft"},
		Apply:    true,
	})
	if err != nil {
		t.Fatalf("Adopt: %v", err)
	}
	if res.Affected != 1 {
		t.Fatalf("affected = %d, want 1", res.Affected)
	}
	if len(res.Notes) == 0 {
		t.Fatal("no note for the unmapped values")
	}
	joined := strings.Join(res.Notes, "\n")
	for _, want := range []string{"wip", "done"} {
		if !strings.Contains(joined, want) {
			t.Errorf("notes do not mention %q:\n%s", want, joined)
		}
	}
	// Left where it was, not guessed at a face.
	if bare, err := st.GetEntityState(ctx, "TSK-2", ""); err != nil || bare == nil {
		t.Error("an unmapped row was moved")
	}
}

// Re-running is the crash-recovery mechanism, so it must converge: the second
// pass finds nothing left at the zero coordinate and does nothing.
func TestAdopt_ReRunConverges(t *testing.T) {
	st := seedStore(t)
	ctx := t.Context()
	req := AdoptRequest{
		Entity:   "task",
		Property: "status",
		Mapping:  map[string]string{"open": "draft", "wip": "draft", "done": "published"},
		Apply:    true,
	}
	deps := adoptDeps(t, st, "draft", "published")
	if _, err := Adopt(ctx, deps, req); err != nil {
		t.Fatalf("first run: %v", err)
	}
	res, err := Adopt(ctx, deps, req)
	if err != nil {
		t.Fatalf("second run: %v", err)
	}
	if res.Affected != 0 {
		t.Fatalf("second run affected %d rows, want 0", res.Affected)
	}
}

// A destination holding DIFFERENT content is two distinct states, and moving
// would destroy one of them. Same contract as rename_face and migrate_face:
// refused with the id named, not silently overwritten.
func TestAdopt_OccupiedDestinationWithDifferentContentIsRefused(t *testing.T) {
	st := seedStore(t)
	ctx := t.Context()
	if err := st.CreateEntity(ctx, &entity.Entity{
		ID: "TSK-1", Type: "task", Face: "draft",
		Properties: map[string]any{"title": "a different draft"},
	}); err != nil {
		t.Fatalf("seed destination: %v", err)
	}

	_, err := Adopt(ctx, adoptDeps(t, st, "draft"), AdoptRequest{
		Entity:   "task",
		Property: "status",
		Mapping:  map[string]string{"open": "draft"},
		Apply:    true,
	})
	if err == nil {
		t.Fatal("expected a refusal")
	}
	// Assert the COLLISION refusal specifically, not merely that something
	// failed: without this the store's own duplicate-create error also names
	// TSK-1, so the test would pass with the guard removed.
	if !strings.Contains(err.Error(), "a row already exists there with different content") {
		t.Errorf("not the collision refusal: %v", err)
	}
	if !strings.Contains(err.Error(), "TSK-1") {
		t.Errorf("error does not name the colliding id: %v", err)
	}
	// Both rows survive the refusal.
	if bare, gErr := st.GetEntityState(ctx, "TSK-1", ""); gErr != nil || bare == nil {
		t.Error("the source row was destroyed by a refused move")
	}
}

// A type declaring no faces has no stranded rows by definition: the zero
// coordinate is where its single state belongs. Refusing names that, rather
// than reporting zero and letting the operator think the repair ran.
func TestAdopt_RefusesATypeWithNoFaces(t *testing.T) {
	st := seedStore(t)
	deps := adoptDeps(t, st)
	deps.Meta = metaV1()

	_, err := Adopt(t.Context(), deps, AdoptRequest{
		Entity:   "task",
		Property: "status",
		Mapping:  map[string]string{"open": "draft"},
	})
	if err == nil || !strings.Contains(err.Error(), "declares no faces") {
		t.Fatalf("want a no-faces refusal, got %v", err)
	}
}

// The destination must be a DECLARED face. Typos are the common case, and a
// silent move to an undeclared coordinate would strand the row again — the
// exact fault this command repairs.
func TestAdopt_RefusesAnUndeclaredDestinationFace(t *testing.T) {
	st := seedStore(t)
	_, err := Adopt(t.Context(), adoptDeps(t, st, "draft"), AdoptRequest{
		Entity:   "task",
		Property: "status",
		Mapping:  map[string]string{"open": "pubished"}, // typo
	})
	if err == nil || !strings.Contains(err.Error(), "not a face declared") {
		t.Fatalf("want an undeclared-face refusal, got %v", err)
	}
}

// An apply is a raw-store write, so it is audited like every other sanctioned
// exception — under its OWN op, so a reader can tell a stranded-row repair
// from a schema-change migration.
func TestAdopt_ApplyIsAudited(t *testing.T) {
	st := seedStore(t)
	sink := audit.NewMemory()
	deps := adoptDeps(t, st, "draft")
	deps.Audit = sink

	if _, err := Adopt(t.Context(), deps, AdoptRequest{
		Entity:   "task",
		Property: "status",
		Mapping:  map[string]string{"open": "draft"},
		Apply:    true,
	}); err != nil {
		t.Fatalf("Adopt: %v", err)
	}
	recs := sink.Records()
	if len(recs) != 1 {
		t.Fatalf("got %d audit records, want 1", len(recs))
	}
	if recs[0].Op != audit.OpDataAdoptFace {
		t.Errorf("op = %q, want %q", recs[0].Op, audit.OpDataAdoptFace)
	}
}

// A dry run writes nothing, so it must not claim in the audit log that it did.
func TestAdopt_DryRunIsNotAudited(t *testing.T) {
	st := seedStore(t)
	sink := audit.NewMemory()
	deps := adoptDeps(t, st, "draft")
	deps.Audit = sink

	if _, err := Adopt(t.Context(), deps, AdoptRequest{
		Entity:   "task",
		Property: "status",
		Mapping:  map[string]string{"open": "draft"},
	}); err != nil {
		t.Fatalf("Adopt: %v", err)
	}
	if n := len(sink.Records()); n != 0 {
		t.Fatalf("dry run wrote %d audit record(s)", n)
	}
}

// Constructors reject nil required collaborators rather than deferring the
// failure to a downstream symptom (CLAUDE.md).
func TestAdopt_RejectsMissingCollaborators(t *testing.T) {
	_, err := Adopt(t.Context(), AdoptDeps{}, AdoptRequest{
		Entity: "task", Property: "status", Mapping: map[string]string{"open": "draft"},
	})
	if err == nil {
		t.Fatal("expected a refusal for nil deps")
	}
}
