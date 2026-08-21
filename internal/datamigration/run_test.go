package datamigration

import (
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/audit"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
)

const v1ToV2Steps = `  - rename_property: {entity: task, from: status, to: state}
  - map_values:
      entity: task
      property: state
      mapping: {open: todo, wip: doing}
  - convert: {entity: task, property: due, to_type: date, from_format: "01/02/2006"}
`

func newTestRunner(t *testing.T, deps Deps) *Runner {
	t.Helper()
	if deps.Meta == nil {
		deps.Meta = metaV2()
	}
	if deps.State == nil {
		deps.State = newFakeKV()
	}
	if deps.Audit == nil {
		deps.Audit = audit.NewMemory()
	}
	if deps.ScriptFS == nil {
		deps.ScriptFS = emptyFS
	}
	r, err := NewRunner(deps)
	if err != nil {
		t.Fatalf("NewRunner: %v", err)
	}
	return r
}

func TestRunner_DryRunCountsWithoutWriting(t *testing.T) {
	st := seedStore(t)
	r := newTestRunner(t, Deps{Store: st})
	f := mustParse(t, "0001-test.yaml", mustFileYAML(t, metaV1(), metaV2(), v1ToV2Steps))

	res, err := r.Run(t.Context(), []*File{f}, false)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if res.Applied {
		t.Fatalf("dry-run reported Applied")
	}
	steps := res.Files[0].Steps
	if steps[0].Affected != 3 { // all three tasks carry status
		t.Errorf("rename affected %d, want 3", steps[0].Affected)
	}
	if steps[1].Affected != 0 { // dry-run: rename didn't happen, no `state` yet
		t.Errorf("map_values affected %d in dry-run, want 0 (previous step not applied)", steps[1].Affected)
	}
	// Nothing was written.
	if _, has := getEntity(t, st, "TSK-1").Properties["state"]; has {
		t.Fatalf("dry-run wrote to the store")
	}
	// Marker untouched.
	marker, err := LoadMarker(t.Context(), r.deps.State)
	if err != nil || marker != nil {
		t.Fatalf("dry-run touched the marker: %v %v", marker, err)
	}
}

func TestRunner_ApplyTransformsAndAdvancesMarker(t *testing.T) {
	st := seedStore(t)
	sink := audit.NewMemory()
	kv := newFakeKV()
	r := newTestRunner(t, Deps{Store: st, State: kv, Audit: sink})
	f := mustParse(t, "0001-test.yaml", mustFileYAML(t, metaV1(), metaV2(), v1ToV2Steps))

	res, err := r.Run(t.Context(), []*File{f}, true)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	e1 := getEntity(t, st, "TSK-1")
	if got := e1.Properties["state"]; got != "todo" {
		t.Errorf("TSK-1 state = %v, want todo (renamed + mapped)", got)
	}
	if _, has := e1.Properties["status"]; has {
		t.Errorf("TSK-1 still has old status key")
	}
	if got := e1.Properties["due"]; got != "2026-01-02" {
		t.Errorf("TSK-1 due = %v, want 2026-01-02 (converted)", got)
	}
	if got := getEntity(t, st, "TSK-2").Properties["state"]; got != "doing" {
		t.Errorf("TSK-2 state = %v, want doing", got)
	}
	// Already-ISO date stays valid (parsed by common layouts, reformatted to itself).
	if got := getEntity(t, st, "TSK-3").Properties["due"]; got != "2026-03-04" {
		t.Errorf("TSK-3 due = %v, want unchanged ISO date", got)
	}

	marker, err := LoadMarker(t.Context(), kv)
	if err != nil || marker == nil {
		t.Fatalf("marker missing after apply: %v", err)
	}
	if marker.ShapeHash != f.To {
		t.Errorf("marker hash = %s, want to-hash %s", marker.ShapeHash, f.To)
	}
	if len(marker.Applied) != 1 || marker.Applied[0] != "0001-test.yaml" {
		t.Errorf("marker applied = %v", marker.Applied)
	}

	recs := sink.Records()
	if len(recs) != 1 || recs[0].Op != audit.OpDataMigration {
		t.Fatalf("audit records = %+v, want one data-migration record", recs)
	}
	if strings.Contains(recs[0].Summary, "todo") {
		t.Errorf("audit summary leaks content: %s", recs[0].Summary)
	}

	// Validation delta: v2-invalid data healed.
	if res.ValidationAfter > res.ValidationBefore {
		t.Errorf("validation got worse: before=%d after=%d", res.ValidationBefore, res.ValidationAfter)
	}
}

func TestRunner_ReRunAfterPartialApplyIsIdempotent(t *testing.T) {
	st := seedStore(t)
	kv := newFakeKV()
	r := newTestRunner(t, Deps{Store: st, State: kv})
	f := mustParse(t, "0001-test.yaml", mustFileYAML(t, metaV1(), metaV2(), v1ToV2Steps))

	// Simulate a crash after step 1 by running a file with only the first
	// step applied, then running the FULL file (recovery = re-run).
	partial := mustParse(t, "0001-test.yaml",
		mustFileYAML(t, metaV1(), metaV2(), "  - rename_property: {entity: task, from: status, to: state}\n"))
	if _, err := r.Run(t.Context(), []*File{partial}, true); err != nil {
		t.Fatalf("partial run: %v", err)
	}
	// Full re-run must complete without damage: rename is a no-op now.
	if _, err := r.Run(t.Context(), []*File{f}, true); err != nil {
		t.Fatalf("recovery run: %v", err)
	}
	if got := getEntity(t, st, "TSK-1").Properties["state"]; got != "todo" {
		t.Errorf("state = %v after recovery, want todo", got)
	}
	// And a third full run changes nothing (steady state).
	res, err := r.Run(t.Context(), []*File{f}, true)
	if err != nil {
		t.Fatalf("steady-state run: %v", err)
	}
	for _, s := range res.Files[0].Steps {
		if s.Affected != 0 {
			t.Errorf("steady-state step %s affected %d, want 0", s.Kind, s.Affected)
		}
	}
}

func TestRunner_UnconvertibleValueLeftInPlace(t *testing.T) {
	st := seedStore(t)
	ctx := t.Context()
	e := getEntity(t, st, "TSK-2")
	e.Properties["due"] = "not a date"
	if err := st.UpdateEntity(ctx, e); err != nil {
		t.Fatal(err)
	}
	r := newTestRunner(t, Deps{Store: st})
	f := mustParse(t, "0001-test.yaml", mustFileYAML(t, metaV1(), metaV2(), v1ToV2Steps))

	res, err := r.Run(ctx, []*File{f}, true)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if got := getEntity(t, st, "TSK-2").Properties["due"]; got != "not a date" {
		t.Errorf("unconvertible value was altered: %v", got)
	}
	var found bool
	for _, n := range res.Files[0].Steps[2].Notes {
		if strings.Contains(n, "cannot convert") {
			found = true
		}
	}
	if !found {
		t.Errorf("no note about the unconvertible value: %+v", res.Files[0].Steps[2].Notes)
	}
}

func TestRunner_NilDepsRejected(t *testing.T) {
	if _, err := NewRunner(Deps{}); err == nil {
		t.Fatalf("NewRunner accepted empty deps")
	}
}

func TestResolve_ChainAndFreeEdges(t *testing.T) {
	v1 := metaV1()
	// v1a: v1 plus an additive property — a shape the gate would have
	// adopted without any migration file existing for it.
	v1a := metaV1()
	v1a.Entities["task"].Properties["estimate"] = metamodel.PropertyDef{Type: "integer"}
	// The migration is authored from v1a (its gen-time marker) to v2a.
	v2a := metaV2()
	v2a.Entities["task"].Properties["estimate"] = metamodel.PropertyDef{Type: "integer"}
	f := mustParse(t, "0001-test.yaml", mustFileYAML(t, v1a, v2a, v1ToV2Steps))

	t.Run("exact from-hash match", func(t *testing.T) {
		plan, err := Resolve(v1a.ShapeProjection(), nil, v2a.ShapeProjection(), []*File{f})
		if err != nil || len(plan) != 1 {
			t.Fatalf("plan = %v, err = %v", plan, err)
		}
	})
	t.Run("compatible gap bridges to the migration", func(t *testing.T) {
		// Store is at plain v1 (never adopted v1a): the additive gap is a
		// free edge onto the migration's from-shape.
		plan, err := Resolve(v1.ShapeProjection(), nil, v2a.ShapeProjection(), []*File{f})
		if err != nil || len(plan) != 1 {
			t.Fatalf("plan = %v, err = %v", plan, err)
		}
	})
	t.Run("compatible tail gap after last migration", func(t *testing.T) {
		// Live schema is v2a plus one more additive property.
		liveMeta := metaV2()
		liveMeta.Entities["task"].Properties["estimate"] = metamodel.PropertyDef{Type: "integer"}
		liveMeta.Entities["task"].Properties["notes"] = metamodel.PropertyDef{Type: "string"}
		plan, err := Resolve(v1a.ShapeProjection(), nil, liveMeta.ShapeProjection(), []*File{f})
		if err != nil || len(plan) != 1 {
			t.Fatalf("plan = %v, err = %v", plan, err)
		}
	})
	t.Run("incompatible tail gap fails naming deltas", func(t *testing.T) {
		liveMeta := metaV2()
		liveMeta.Entities["task"].Properties["estimate"] = metamodel.PropertyDef{Type: "integer"}
		liveMeta.Entities["task"].Properties["due"] = metamodel.PropertyDef{Type: "integer"} // another incompatible change, unmigrated
		_, err := Resolve(v1a.ShapeProjection(), nil, liveMeta.ShapeProjection(), []*File{f})
		if err == nil || !strings.Contains(err.Error(), "rela migrate gen") {
			t.Fatalf("err = %v, want incompatible-tail error", err)
		}
	})
	t.Run("applied file is skipped", func(t *testing.T) {
		plan, err := Resolve(v2a.ShapeProjection(), []string{"0001-test.yaml"}, v2a.ShapeProjection(), []*File{f})
		if err != nil || len(plan) != 0 {
			t.Fatalf("plan = %v, err = %v", plan, err)
		}
	})
	t.Run("already-at-to is skipped", func(t *testing.T) {
		plan, err := Resolve(v2a.ShapeProjection(), nil, v2a.ShapeProjection(), []*File{f})
		if err != nil || len(plan) != 0 {
			t.Fatalf("plan = %v, err = %v", plan, err)
		}
	})
	t.Run("incompatible gap to migration fails", func(t *testing.T) {
		// Store at v2 (due already a date, status renamed) but the migration
		// expects v1a — the gap backwards is incompatible.
		_, err := Resolve(metaV2().ShapeProjection(), nil, v2a.ShapeProjection(), []*File{f})
		if err == nil {
			t.Fatalf("expected incompatible-gap error")
		}
	})
}
