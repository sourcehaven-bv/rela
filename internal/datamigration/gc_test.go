package datamigration

import (
	"testing"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/audit"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// fixedVerdict is a VerdictSource pinned to one verdict.
type fixedVerdict struct{ v *Verdict }

func (f fixedVerdict) Verdict() *Verdict { return f.v }

func newTestGC(t *testing.T, deps GCDeps) *GC {
	t.Helper()
	if deps.Audit == nil {
		deps.Audit = audit.NewMemory()
	}
	if deps.Lock == nil {
		deps.Lock = NewProcessLock()
	}
	g, err := NewGC(deps)
	if err != nil {
		t.Fatalf("NewGC: %v", err)
	}
	return g
}

// driftSetup drives a real gate through a drift adoption (tags + person
// deleted) and returns the pieces a GC test needs.
func driftSetup(t *testing.T) (store.Store, *fakeKV, *Gate, *metamodel.Metamodel) {
	t.Helper()
	st := seedStore(t)
	kv := newFakeKV()
	gate := newTestGate(t, kv)
	if _, err := gate.Evaluate(t.Context(), metaV1()); err != nil {
		t.Fatal(err)
	}
	m2 := metaV1()
	delete(m2.Entities["task"].Properties, "tags")
	delete(m2.Entities, "person")
	if _, err := gate.Evaluate(t.Context(), m2); err != nil {
		t.Fatal(err)
	}
	// Give the tasks tag values so the property GC has something to remove.
	ctx := t.Context()
	e := getEntity(t, st, "TSK-1")
	e.Properties["tags"] = []any{"a", "b"}
	if err := st.UpdateEntity(ctx, e); err != nil {
		t.Fatal(err)
	}
	return st, kv, gate, m2
}

func TestGC_SkipsWhileNeedsMigration(t *testing.T) {
	st, kv, _, m2 := driftSetup(t)
	g := newTestGC(t, GCDeps{
		Store: st, State: kv, Meta: func() *metamodel.Metamodel { return m2 },
		Verdicts: fixedVerdict{&Verdict{Status: StatusNeedsMigration}},
	})
	res, err := g.Tick(t.Context(), true)
	if err != nil {
		t.Fatalf("Tick: %v", err)
	}
	if res.Skipped == "" || len(res.Deleted) != 0 {
		t.Fatalf("tick did not skip under needs-migration: %+v", res)
	}
}

func TestGC_GracePeriodHoldsThenDeletes(t *testing.T) {
	st, kv, gate, m2 := driftSetup(t)
	sink := audit.NewMemory()
	g := newTestGC(t, GCDeps{
		Store: st, State: kv, Meta: func() *metamodel.Metamodel { return m2 },
		Verdicts: gate, Audit: sink,
	})

	// Within the grace period: everything pending, nothing deleted.
	res, err := g.Tick(t.Context(), true)
	if err != nil {
		t.Fatalf("Tick: %v", err)
	}
	if len(res.Deleted) != 0 || len(res.Pending) == 0 {
		t.Fatalf("grace period not honored: %+v", res)
	}
	if _, has := getEntity(t, st, "TSK-1").Properties["tags"]; !has {
		t.Fatalf("data deleted inside grace period")
	}

	// Jump the clock past the grace period.
	g.now = func() time.Time { return time.Now().Add(DefaultGrace + time.Hour) }
	res, err = g.Tick(t.Context(), true)
	if err != nil {
		t.Fatalf("Tick: %v", err)
	}
	if len(res.Deleted) == 0 {
		t.Fatalf("nothing deleted after grace expiry: %+v", res)
	}
	if _, has := getEntity(t, st, "TSK-1").Properties["tags"]; has {
		t.Errorf("orphaned property survived GC")
	}
	if _, err := st.GetEntity(t.Context(), "PER-1"); err == nil {
		t.Errorf("orphaned entity survived GC")
	}
	// Ledger entries consumed.
	ledger, _ := LoadLedger(t.Context(), kv)
	if len(ledger.Entries) != 0 {
		t.Errorf("ledger not cleared after GC: %+v", ledger.Entries)
	}
	// Audited, without content.
	recs := sink.Records()
	if len(recs) != 1 || recs[0].Op != audit.OpDataGC {
		t.Fatalf("audit = %+v, want one data-gc record", recs)
	}
}

func TestGC_DryRunCountsOnly(t *testing.T) {
	st, kv, gate, m2 := driftSetup(t)
	g := newTestGC(t, GCDeps{
		Store: st, State: kv, Meta: func() *metamodel.Metamodel { return m2 }, Verdicts: gate,
	})
	g.now = func() time.Time { return time.Now().Add(DefaultGrace + time.Hour) }
	res, err := g.Tick(t.Context(), false)
	if err != nil {
		t.Fatalf("Tick: %v", err)
	}
	if len(res.Deleted) == 0 {
		t.Fatalf("dry-run reported nothing to delete")
	}
	if _, has := getEntity(t, st, "TSK-1").Properties["tags"]; !has {
		t.Fatalf("dry-run deleted data")
	}
	ledger, _ := LoadLedger(t.Context(), kv)
	if len(ledger.Entries) == 0 {
		t.Fatalf("dry-run consumed ledger entries")
	}
}

func TestGC_ReAddedSubjectIsPruned(t *testing.T) {
	st, kv, gate, _ := driftSetup(t)
	// Schema re-adds tags and person before the grace period expires.
	restored := metaV1()
	if _, err := gate.Evaluate(t.Context(), restored); err != nil {
		t.Fatal(err)
	}
	g := newTestGC(t, GCDeps{
		Store: st, State: kv, Meta: func() *metamodel.Metamodel { return restored }, Verdicts: gate,
	})
	g.now = func() time.Time { return time.Now().Add(DefaultGrace + time.Hour) }
	res, err := g.Tick(t.Context(), true)
	if err != nil {
		t.Fatalf("Tick: %v", err)
	}
	if len(res.Deleted) != 0 {
		t.Fatalf("GC deleted data whose schema subject was restored: %+v", res.Deleted)
	}
	if _, has := getEntity(t, st, "TSK-1").Properties["tags"]; !has {
		t.Fatalf("restored property's data was deleted")
	}
}

func TestGC_ScanFindsLegacyOrphans(t *testing.T) {
	// An orphan that never went through a gate transition: a property that
	// exists only in the data (hand-edited file, pre-feature history).
	st := seedStore(t)
	ctx := t.Context()
	e := getEntity(t, st, "TSK-1")
	e.Properties["legacy_field"] = "old"
	if err := st.UpdateEntity(ctx, e); err != nil {
		t.Fatal(err)
	}
	kv := newFakeKV()
	gate := newTestGate(t, kv)
	if _, err := gate.Evaluate(ctx, metaV1()); err != nil {
		t.Fatal(err)
	}
	g := newTestGC(t, GCDeps{
		Store: st, State: kv, Meta: metaV1, Verdicts: gate,
	})
	added, err := g.Scan(ctx)
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	if len(added) != 1 || added[0] != "task.legacy_field" {
		t.Fatalf("scan added %v, want [task.legacy_field]", added)
	}
	// Second scan is a no-op (first-seen not reset).
	added, err = g.Scan(ctx)
	if err != nil || len(added) != 0 {
		t.Fatalf("re-scan added %v err %v, want none", added, err)
	}
}
