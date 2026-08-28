package datamigration

// Regression tests for the TKT-0C57FS code-review findings.

import (
	"context"
	"errors"
	iofs "io/fs"
	"strings"
	"testing"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// RR: an enum-value drift adoption must not create a ledger entry — an
// unprocessable entry used to wedge every subsequent GC tick forever.
func TestGC_EnumValueDriftNeverWedgesTheSweep(t *testing.T) {
	st := seedStore(t)
	kv := newFakeKV()
	gate := newTestGate(t, kv)
	if _, err := gate.Evaluate(t.Context(), metaV1()); err != nil {
		t.Fatal(err)
	}
	// Remove an enum value: drift-adopts.
	m2 := metaV1()
	m2.Types["status"] = metamodel.CustomType{Values: []string{"open", "done"}}
	v, err := gate.Evaluate(t.Context(), m2)
	if err != nil || v.Status != StatusAdopted {
		t.Fatalf("status = %v err = %v, want adopted", v.Status, err)
	}
	ledger, _ := LoadLedger(t.Context(), kv)
	if len(ledger.Entries) != 0 {
		t.Fatalf("enum-value drift entered the ledger: %+v", ledger.Entries)
	}

	// A legacy/unknown ledger kind must be dropped, not wedge the tick.
	ledger.Entries["type:status"] = LedgerEntry{Kind: "enum_values", FirstSeen: time.Now().Add(-2 * DefaultGrace)}
	ledger.Entries["task.tags"] = LedgerEntry{Kind: "property", FirstSeen: time.Now().Add(-2 * DefaultGrace)}
	if saveErr := SaveLedger(t.Context(), kv, ledger); saveErr != nil {
		t.Fatal(saveErr)
	}
	// Make tags a real orphan with data.
	m3 := metaV1()
	m3.Types["status"] = metamodel.CustomType{Values: []string{"open", "done"}}
	delete(m3.Entities["task"].Properties, "tags")
	if _, evalErr := gate.Evaluate(t.Context(), m3); evalErr != nil {
		t.Fatal(evalErr)
	}
	g := newTestGC(t, GCDeps{
		Store: st, State: kv, Meta: func() *metamodel.Metamodel { return m3 }, Verdicts: gate,
	})
	res, err := g.Tick(t.Context(), true)
	if err != nil {
		t.Fatalf("Tick wedged on legacy ledger kind: %v", err)
	}
	if len(res.Deleted) == 0 {
		t.Fatalf("legitimate GC did not run alongside the legacy entry: %+v", res)
	}
	after, _ := LoadLedger(t.Context(), kv)
	if _, still := after.Entries["type:status"]; still {
		t.Fatalf("legacy unknown-kind entry not dropped")
	}
}

// RR: cardinality bounds use effective values — `min: unset -> 0` is a
// semantic no-op and must not demand a migration.
func TestCompareShapes_CardinalityEffectiveValues(t *testing.T) {
	zero, one, five := 0, 1, 5
	tests := []struct {
		name     string
		from, to *int
		isMax    bool
		wantKind string // "" = no delta
		wantTier metamodel.ShapeTier
	}{
		{name: "min unset->0 is a no-op", from: nil, to: &zero, isMax: false, wantKind: ""},
		{name: "min 0->unset is a no-op", from: &zero, to: nil, isMax: false, wantKind: ""},
		{name: "min unset->1 tightens", from: nil, to: &one, isMax: false,
			wantKind: "relation_cardinality_tightened", wantTier: metamodel.TierMigration},
		{name: "min 5->1 loosens", from: &five, to: &one, isMax: false,
			wantKind: "relation_cardinality_loosened", wantTier: metamodel.TierAdditive},
		{name: "max unset->5 tightens", from: nil, to: &five, isMax: true,
			wantKind: "relation_cardinality_tightened", wantTier: metamodel.TierMigration},
		{name: "max 1->unset loosens", from: &one, to: nil, isMax: true,
			wantKind: "relation_cardinality_loosened", wantTier: metamodel.TierAdditive},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			fromMeta, toMeta := metaV1(), metaV1()
			rf := fromMeta.Relations["assigned-to"]
			rt := toMeta.Relations["assigned-to"]
			if tc.isMax {
				rf.MaxOutgoing, rt.MaxOutgoing = tc.from, tc.to
			} else {
				rf.MaxOutgoing, rt.MaxOutgoing = nil, nil // fixture default is 1; neutralize
				rf.MinOutgoing, rt.MinOutgoing = tc.from, tc.to
			}
			fromMeta.Relations["assigned-to"] = rf
			toMeta.Relations["assigned-to"] = rt

			r := metamodel.CompareShapes(fromMeta.ShapeProjection(), toMeta.ShapeProjection())
			var got []metamodel.ShapeDelta
			for _, d := range r.Deltas {
				if strings.HasPrefix(d.Kind, "relation_cardinality") {
					got = append(got, d)
				}
			}
			if tc.wantKind == "" {
				if len(got) != 0 {
					t.Fatalf("no-op bound change produced deltas: %+v", got)
				}
				return
			}
			if len(got) != 1 || got[0].Kind != tc.wantKind || got[0].Tier != tc.wantTier {
				t.Fatalf("got %+v, want kind=%s tier=%s", got, tc.wantKind, tc.wantTier)
			}
		})
	}
}

// RR: a runaway Lua script must be interruptible via ctx.
func TestLuaStep_RunawayScriptInterruptedByContext(t *testing.T) {
	f, fsys := luaFixture(t, "task", `function migrate(entity) while true do end end`)
	st := seedStore(t)
	r := newTestRunner(t, Deps{Store: st, ScriptFS: fsys})

	ctx, cancel := context.WithTimeout(t.Context(), 200*time.Millisecond)
	defer cancel()
	done := make(chan error, 1)
	go func() {
		_, err := r.Run(ctx, []*File{f}, true)
		done <- err
	}()
	select {
	case err := <-done:
		if err == nil {
			t.Fatalf("runaway script completed without error")
		}
	case <-time.After(5 * time.Second):
		t.Fatalf("runaway lua script was not interrupted by context cancellation")
	}
}

// RR: Scan must not ledger rela-managed (underscore) properties on entities.
func TestGC_ScanSkipsManagedProperties(t *testing.T) {
	st := seedStore(t)
	ctx := t.Context()
	e := getEntity(t, st, "TSK-1")
	e.Properties["_order_out"] = 1.5
	e.Properties["_internal"] = "x"
	if err := st.UpdateEntity(ctx, e); err != nil {
		t.Fatal(err)
	}
	kv := newFakeKV()
	gate := newTestGate(t, kv)
	if _, err := gate.Evaluate(ctx, metaV1()); err != nil {
		t.Fatal(err)
	}
	g := newTestGC(t, GCDeps{Store: st, State: kv, Meta: metaV1, Verdicts: gate})
	added, err := g.Scan(ctx)
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	if len(added) != 0 {
		t.Fatalf("scan ledgered managed properties: %v", added)
	}
}

// errFS is an fs.FS whose every Open fails with a non-ErrNotExist error,
// simulating an unreadable migrations/ directory.
type errFS struct{ err error }

func (f errFS) Open(string) (iofs.File, error) { return nil, f.err }

// RR: an unreadable migrations/ directory must surface, not read as an
// empty chain.
func TestLoadDir_UnreadableDirectorySurfaces(t *testing.T) {
	_, err := LoadDir(errFS{err: errors.New("permission denied")})
	if err == nil || !strings.Contains(err.Error(), "permission denied") {
		t.Fatalf("unreadable migrations/ read as empty chain: %v", err)
	}
}

// RR: a failed pre-delete version capture must abort the destructive step.
type failingCapture struct{}

func (failingCapture) WriteVersion(context.Context, store.VersionInput) error {
	return errors.New("capture backend down")
}

func (failingCapture) WriteRelationVersion(context.Context, store.RelationVersionInput) error {
	return errors.New("capture backend down")
}

func TestDropEntities_AbortsWhenCaptureFails(t *testing.T) {
	st := seedStore(t)
	// v2 drops the person type so drop_entities validates.
	from := metaV1()
	to := metaV1()
	delete(to.Entities, "person")
	f := mustParse(t, "0001-drop.yaml", mustFileYAML(t, from, to, "  - drop_entities: {type: person}\n"))

	r := newTestRunner(t, Deps{Store: st, Meta: to, Versions: failingCapture{}})

	// Dry-run never captures, so it must succeed.
	if _, err := r.Run(t.Context(), []*File{f}, false); err != nil {
		t.Fatalf("dry-run: %v", err)
	}
	// Apply: the capture failure must abort BEFORE the delete.
	_, err := r.Run(t.Context(), []*File{f}, true)
	if err == nil || !strings.Contains(err.Error(), "capture") {
		t.Fatalf("apply err = %v, want capture failure", err)
	}
	if _, gerr := st.GetEntity(t.Context(), "PER-1"); gerr != nil {
		t.Fatalf("entity was deleted despite failed capture: %v", gerr)
	}
}

// RR: rename_property leaves BOTH values untouched on a conflict.
func TestRenameProperty_ConflictLeavesBothValues(t *testing.T) {
	st := seedStore(t)
	ctx := t.Context()
	e := getEntity(t, st, "TSK-1")
	e.Properties["state"] = "todo" // target already set
	if err := st.UpdateEntity(ctx, e); err != nil {
		t.Fatal(err)
	}
	f := mustParse(t, "0001-r.yaml",
		mustFileYAML(t, metaV1(), metaV2(), "  - rename_property: {entity: task, from: status, to: state}\n"))
	r := newTestRunner(t, Deps{Store: st})
	res, err := r.Run(ctx, []*File{f}, true)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	e = getEntity(t, st, "TSK-1")
	if e.Properties["status"] != "open" || e.Properties["state"] != "todo" {
		t.Fatalf("conflict mutated data: status=%v state=%v", e.Properties["status"], e.Properties["state"])
	}
	var noted bool
	for _, n := range res.Files[0].Steps[0].Notes {
		if strings.Contains(n, "left untouched") {
			noted = true
		}
	}
	if !noted {
		t.Fatalf("conflict not surfaced in notes: %+v", res.Files[0].Steps[0].Notes)
	}
}
