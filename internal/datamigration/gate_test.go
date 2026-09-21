package datamigration

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/metamodel"
)

func newTestGate(t *testing.T, kv *fakeKV) (*Gate, *fakeMigState) {
	t.Helper()
	return newTestGateWithMigrations(t, kv, false)
}

// newTestGateWithMigrations builds a gate that reports whether the project has
// migration files, which is what distinguishes a new store from an
// un-baselined one.
func newTestGateWithMigrations(t *testing.T, kv *fakeKV, hasMigrations bool) (*Gate, *fakeMigState) {
	t.Helper()
	ms := newMigState()
	g, err := NewGate(GateDeps{
		MigState:      ms,
		State:         kv,
		HasMigrations: func(context.Context) (bool, error) { return hasMigrations, nil },
	})
	if err != nil {
		t.Fatalf("NewGate: %v", err)
	}
	return g, ms
}

// evalPersist is the operator-driven path: classify, then record. Most gate
// tests want it, because they assert on what was written.
func evalPersist(t *testing.T, g *Gate, m *metamodel.Metamodel) *Verdict {
	t.Helper()
	v, err := g.EvaluateAndPersist(t.Context(), m)
	if err != nil {
		t.Fatalf("EvaluateAndPersist: %v", err)
	}
	return v
}

func storedHash(t *testing.T, ms *fakeMigState) string {
	t.Helper()
	st, err := ms.Load(t.Context())
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if st == nil {
		return ""
	}
	proj, err := st.ShapeProjection()
	if err != nil {
		t.Fatalf("ShapeProjection: %v", err)
	}
	return proj.Hash()
}

// A project with no migrations is an ordinary new store, so adopting the live
// shape as the baseline is safe and silent.
func TestGate_BootstrapAdoptsBaseline(t *testing.T) {
	kv := newFakeKV()
	g, ms := newTestGate(t, kv)
	v := evalPersist(t, g, metaV1())
	if v.Status != StatusBootstrapped {
		t.Fatalf("status = %s, want bootstrapped", v.Status)
	}
	if got := storedHash(t, ms); got != metaV1().ShapeProjection().Hash() {
		t.Fatalf("baseline not recorded on bootstrap: %q", got)
	}
	if g.Verdict() != v {
		t.Fatalf("verdict not published")
	}
}

// With migrations on disk and nothing recorded, the shape the content conforms
// to is genuinely unknown: those files may be exactly what this store still
// needs. Baselining would mark them applied forever, so the gate refuses.
func TestGate_UnbaselinedWhenMigrationsExistAndNothingRecorded(t *testing.T) {
	kv := newFakeKV()
	g, ms := newTestGateWithMigrations(t, kv, true)
	v := evalPersist(t, g, metaV1())
	if v.Status != StatusUnbaselined {
		t.Fatalf("status = %s, want unbaselined", v.Status)
	}
	if got := storedHash(t, ms); got != "" {
		t.Fatalf("the un-baselined case must record nothing, got %q", got)
	}
	if !strings.Contains(v.Describe(), "rela migrate") {
		t.Errorf("Describe should tell the operator what to run, got: %s", v.Describe())
	}
}

// Evaluate classifies; it must never write. This is what lets a server run the
// gate at boot without touching a git-tracked file (TKT-XCJ0Y2).
func TestGate_EvaluateDoesNotPersist(t *testing.T) {
	kv := newFakeKV()
	g, ms := newTestGate(t, kv)
	if _, err := g.Evaluate(t.Context(), metaV1()); err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	if got := storedHash(t, ms); got != "" {
		t.Fatalf("Evaluate wrote migration state (%q); only Persist may write", got)
	}

	// And the adoption it classified is still available to persist later.
	v := g.Verdict()
	if err := g.Persist(t.Context(), v); err != nil {
		t.Fatalf("Persist: %v", err)
	}
	if got := storedHash(t, ms); got != metaV1().ShapeProjection().Hash() {
		t.Fatalf("Persist did not record the verdict's adoption, got %q", got)
	}
}

func TestGate_InSync(t *testing.T) {
	kv := newFakeKV()
	g, _ := newTestGate(t, kv)
	evalPersist(t, g, metaV1())
	if v := evalPersist(t, g, metaV1()); v.Status != StatusInSync {
		t.Fatalf("status = %v, want in-sync", v.Status)
	}
}

func TestGate_AdditiveAdoptsSilently(t *testing.T) {
	kv := newFakeKV()
	g, ms := newTestGate(t, kv)
	evalPersist(t, g, metaV1())
	m2 := metaV1()
	m2.Entities["task"].Properties["estimate"] = metamodel.PropertyDef{Type: "integer"}
	if v := evalPersist(t, g, m2); v.Status != StatusAdopted {
		t.Fatalf("status = %v, want adopted", v.Status)
	}
	if storedHash(t, ms) != m2.ShapeProjection().Hash() {
		t.Fatalf("recorded shape not advanced on additive adoption")
	}
	// Additive-only: nothing entered the GC ledger.
	ledger, _ := LoadLedger(t.Context(), kv)
	if len(ledger.Entries) != 0 {
		t.Fatalf("additive adoption polluted the ledger: %+v", ledger.Entries)
	}
}

func TestGate_DriftAdoptsAndRecordsLedger(t *testing.T) {
	kv := newFakeKV()
	g, _ := newTestGate(t, kv)
	evalPersist(t, g, metaV1())
	m2 := metaV1()
	delete(m2.Entities["task"].Properties, "tags")
	delete(m2.Entities, "person")
	if v := evalPersist(t, g, m2); v.Status != StatusAdopted {
		t.Fatalf("status = %v, want adopted", v.Status)
	}
	ledger, _ := LoadLedger(t.Context(), kv)
	if _, ok := ledger.Entries["task.tags"]; !ok {
		t.Errorf("deleted property not in ledger: %+v", ledger.Entries)
	}
	if e, ok := ledger.Entries["person"]; !ok || e.Kind != "entity_type" {
		t.Errorf("deleted entity type not in ledger: %+v", ledger.Entries)
	}
}

func TestGate_NeedsMigrationDoesNotAdopt(t *testing.T) {
	kv := newFakeKV()
	g, ms := newTestGate(t, kv)
	evalPersist(t, g, metaV1())
	oldHash := metaV1().ShapeProjection().Hash()

	v := evalPersist(t, g, metaV2())
	if v.Status != StatusNeedsMigration {
		t.Fatalf("status = %v, want needs-migration", v.Status)
	}
	if storedHash(t, ms) != oldHash {
		t.Fatalf("recorded shape moved despite needs-migration")
	}
	// The published verdict carries the deltas for the banner/status output.
	if len(v.Report.ByTier(metamodel.TierMigration)) == 0 {
		t.Fatalf("verdict carries no migration deltas")
	}
}

func TestGate_ReEvaluateOnReloadUpdatesVerdict(t *testing.T) {
	// Simulates the metamodel hot-reload path: same gate, new metamodel.
	kv := newFakeKV()
	g, _ := newTestGate(t, kv)
	evalPersist(t, g, metaV1())
	evalPersist(t, g, metaV2())
	if got := g.Verdict().Status; got != StatusNeedsMigration {
		t.Fatalf("verdict after reload = %s, want needs-migration", got)
	}
	// Operator reverts schema.yaml: next reload goes back to in-sync.
	evalPersist(t, g, metaV1())
	if got := g.Verdict().Status; got != StatusInSync {
		t.Fatalf("verdict after revert = %s, want in-sync", got)
	}
}

// Evaluate reads outside the lock, so the record can move before Persist runs.
// Persist must then decline rather than blind-overwrite: writing the verdict's
// captured applied list would UN-record whatever landed in between, and that
// migration would silently re-run.
func TestGate_PersistDeclinesWhenTheRecordMoved(t *testing.T) {
	kv := newFakeKV()
	g, ms := newTestGate(t, kv)
	evalPersist(t, g, metaV1())

	// Classify an additive change, but do not persist it yet.
	m2 := metaV1()
	m2.Entities["task"].Properties["estimate"] = metamodel.PropertyDef{Type: "integer"}
	v, err := g.Evaluate(t.Context(), m2)
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	if v.Status != StatusAdopted {
		t.Fatalf("status = %s, want adopted", v.Status)
	}

	// A concurrent runner records a migration and moves the shape.
	concurrent, err := (&State{}).WithApplied("20260919143022-concurrent.yaml", metaV2().ShapeProjection(), now())
	if err != nil {
		t.Fatalf("WithApplied: %v", err)
	}
	if saveErr := ms.Save(t.Context(), concurrent); saveErr != nil {
		t.Fatalf("Save: %v", saveErr)
	}

	// Persisting the stale verdict must not clobber it.
	if persistErr := g.Persist(t.Context(), v); persistErr != nil {
		t.Fatalf("Persist: %v", persistErr)
	}
	got, err := ms.Load(t.Context())
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if names := got.AppliedNames(); len(names) != 1 || names[0] != "20260919143022-concurrent.yaml" {
		t.Fatalf("a stale Persist un-recorded the concurrent migration: applied = %v", names)
	}
}

func now() time.Time { return time.Date(2026, 9, 19, 14, 30, 22, 0, time.UTC) }
