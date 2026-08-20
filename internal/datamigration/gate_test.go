package datamigration

import (
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/metamodel"
)

func newTestGate(t *testing.T, kv *fakeKV) *Gate {
	t.Helper()
	g, err := NewGate(kv)
	if err != nil {
		t.Fatalf("NewGate: %v", err)
	}
	return g
}

func TestGate_BootstrapAdoptsBaseline(t *testing.T) {
	kv := newFakeKV()
	g := newTestGate(t, kv)
	v, err := g.Evaluate(t.Context(), metaV1())
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	if v.Status != StatusBootstrapped {
		t.Fatalf("status = %s, want bootstrapped", v.Status)
	}
	marker, _ := LoadMarker(t.Context(), kv)
	if marker == nil || marker.ShapeHash != metaV1().ShapeProjection().Hash() {
		t.Fatalf("marker not written on bootstrap: %+v", marker)
	}
	if g.Verdict() != v {
		t.Fatalf("verdict not published")
	}
}

func TestGate_InSync(t *testing.T) {
	kv := newFakeKV()
	g := newTestGate(t, kv)
	if _, err := g.Evaluate(t.Context(), metaV1()); err != nil {
		t.Fatal(err)
	}
	v, err := g.Evaluate(t.Context(), metaV1())
	if err != nil || v.Status != StatusInSync {
		t.Fatalf("status = %v err = %v, want in-sync", v.Status, err)
	}
}

func TestGate_AdditiveAdoptsSilently(t *testing.T) {
	kv := newFakeKV()
	g := newTestGate(t, kv)
	if _, err := g.Evaluate(t.Context(), metaV1()); err != nil {
		t.Fatal(err)
	}
	m2 := metaV1()
	m2.Entities["task"].Properties["estimate"] = metamodel.PropertyDef{Type: "integer"}
	v, err := g.Evaluate(t.Context(), m2)
	if err != nil || v.Status != StatusAdopted {
		t.Fatalf("status = %v err = %v, want adopted", v.Status, err)
	}
	marker, _ := LoadMarker(t.Context(), kv)
	if marker.ShapeHash != m2.ShapeProjection().Hash() {
		t.Fatalf("marker not advanced on additive adoption")
	}
	// Additive-only: nothing entered the GC ledger.
	ledger, _ := LoadLedger(t.Context(), kv)
	if len(ledger.Entries) != 0 {
		t.Fatalf("additive adoption polluted the ledger: %+v", ledger.Entries)
	}
}

func TestGate_DriftAdoptsAndRecordsLedger(t *testing.T) {
	kv := newFakeKV()
	g := newTestGate(t, kv)
	if _, err := g.Evaluate(t.Context(), metaV1()); err != nil {
		t.Fatal(err)
	}
	m2 := metaV1()
	delete(m2.Entities["task"].Properties, "tags")
	delete(m2.Entities, "person")
	v, err := g.Evaluate(t.Context(), m2)
	if err != nil || v.Status != StatusAdopted {
		t.Fatalf("status = %v err = %v, want adopted", v.Status, err)
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
	g := newTestGate(t, kv)
	if _, err := g.Evaluate(t.Context(), metaV1()); err != nil {
		t.Fatal(err)
	}
	oldHash := metaV1().ShapeProjection().Hash()

	v, err := g.Evaluate(t.Context(), metaV2())
	if err != nil || v.Status != StatusNeedsMigration {
		t.Fatalf("status = %v err = %v, want needs-migration", v.Status, err)
	}
	marker, _ := LoadMarker(t.Context(), kv)
	if marker.ShapeHash != oldHash {
		t.Fatalf("marker moved despite needs-migration")
	}
	// The published verdict carries the deltas for the banner/status output.
	if len(v.Report.ByTier(metamodel.TierMigration)) == 0 {
		t.Fatalf("verdict carries no migration deltas")
	}
}

func TestGate_ReEvaluateOnReloadUpdatesVerdict(t *testing.T) {
	// Simulates the metamodel hot-reload path: same gate, new metamodel.
	kv := newFakeKV()
	g := newTestGate(t, kv)
	if _, err := g.Evaluate(t.Context(), metaV1()); err != nil {
		t.Fatal(err)
	}
	if _, err := g.Evaluate(t.Context(), metaV2()); err != nil {
		t.Fatal(err)
	}
	if got := g.Verdict().Status; got != StatusNeedsMigration {
		t.Fatalf("verdict after reload = %s, want needs-migration", got)
	}
	// Operator reverts schema.yaml: next reload goes back to in-sync.
	if _, err := g.Evaluate(t.Context(), metaV1()); err != nil {
		t.Fatal(err)
	}
	if got := g.Verdict().Status; got != StatusInSync {
		t.Fatalf("verdict after revert = %s, want in-sync", got)
	}
}
