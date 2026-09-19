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
	if deps.MigState == nil {
		deps.MigState = newMigState()
	}
	if deps.Audit == nil {
		deps.Audit = audit.NewMemory()
	}
	if deps.ScriptFS == nil {
		deps.ScriptFS = emptyFS
	}
	if deps.Lock == nil {
		deps.Lock = NewProcessLock()
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
	f := mustParse(t, testName("test"), mustFileYAML(t, metaV1(), metaV2(), v1ToV2Steps))

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
	// Migration state untouched.
	got, err := r.deps.MigState.Load(t.Context())
	if err != nil || got != nil {
		t.Fatalf("dry-run recorded migration state: %v %v", got, err)
	}
}

func TestRunner_ApplyTransformsAndAdvancesMarker(t *testing.T) {
	st := seedStore(t)
	sink := audit.NewMemory()
	kv := newFakeKV()
	ms := newMigState()
	r := newTestRunner(t, Deps{Store: st, State: kv, MigState: ms, Audit: sink})
	f := mustParse(t, testName("test"), mustFileYAML(t, metaV1(), metaV2(), v1ToV2Steps))

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

	recorded, err := ms.Load(t.Context())
	if err != nil || recorded == nil {
		t.Fatalf("migration state missing after apply: %v", err)
	}
	proj, err := recorded.ShapeProjection()
	if err != nil {
		t.Fatalf("ShapeProjection: %v", err)
	}
	if proj.Hash() != f.ToProjection.Hash() {
		t.Errorf("recorded shape = %s, want the file's to-shape %s", proj.Hash(), f.ToProjection.Hash())
	}
	if names := recorded.AppliedNames(); len(names) != 1 || names[0] != testName("test") {
		t.Errorf("applied list = %v", names)
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
	f := mustParse(t, testName("test"), mustFileYAML(t, metaV1(), metaV2(), v1ToV2Steps))

	// Simulate a crash after step 1 by running a file with only the first
	// step applied, then running the FULL file (recovery = re-run).
	partial := mustParse(t, testName("test"),
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

func TestRunner_RecomputeComputedGraph(t *testing.T) {
	from := metaV1()
	from.Entities["task"].Properties["effort"] = metamodel.PropertyDef{Type: "integer"}
	to := metaV1()
	to.Entities["task"].Properties["effort"] = metamodel.PropertyDef{Type: "integer"}
	to.Entities["task"].Properties["doubled"] = metamodel.PropertyDef{
		Type: "integer", Computed: "entity.effort * 2",
	}
	to.Entities["task"].Properties["quadrupled"] = metamodel.PropertyDef{
		Type: "integer", Computed: "entity.doubled * 2",
	}

	st := seedStore(t)
	for i, id := range []string{"TSK-1", "TSK-2", "TSK-3"} {
		e := getEntity(t, st, id)
		e.Properties["effort"] = int64(i + 1)
		e.Properties["doubled"] = int64(99)
		e.Properties["quadrupled"] = int64(99)
		if err := st.UpdateEntity(t.Context(), e); err != nil {
			t.Fatal(err)
		}
	}
	f := mustParse(t, testName("computed"), mustFileYAML(t, from, to,
		"  - recompute_computed: {entity: task}\n"))
	r := newTestRunner(t, Deps{Store: st, Meta: to})

	dry, err := r.Run(t.Context(), []*File{f}, false)
	if err != nil {
		t.Fatalf("dry-run: %v", err)
	}
	if got := dry.Files[0].Steps[0].Affected; got != 3 {
		t.Fatalf("dry-run affected = %d, want 3", got)
	}
	if got := getEntity(t, st, "TSK-1").Properties["quadrupled"]; got != int64(99) {
		t.Fatalf("dry-run wrote quadrupled = %v", got)
	}

	applied, err := r.Run(t.Context(), []*File{f}, true)
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	if got := applied.Files[0].Steps[0].Affected; got != 3 {
		t.Fatalf("apply affected = %d, want 3", got)
	}
	if got := getEntity(t, st, "TSK-3").Properties["doubled"]; got != int64(6) {
		t.Errorf("doubled = %v, want 6", got)
	}
	if got := getEntity(t, st, "TSK-3").Properties["quadrupled"]; got != int64(12) {
		t.Errorf("quadrupled = %v, want 12", got)
	}

	steady, err := r.Run(t.Context(), []*File{f}, true)
	if err != nil {
		t.Fatalf("steady-state apply: %v", err)
	}
	if got := steady.Files[0].Steps[0].Affected; got != 0 {
		t.Errorf("steady-state affected = %d, want 0", got)
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
	f := mustParse(t, testName("test"), mustFileYAML(t, metaV1(), metaV2(), v1ToV2Steps))

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

// Resolve plans by NAME: a file runs when the applied list does not contain
// it. That is the whole rule, and it is what makes a data-only migration
// representable (TKT-XCJ0Y2).
func TestResolve_PlansByAppliedName(t *testing.T) {
	v1, v2 := metaV1().ShapeProjection(), metaV2().ShapeProjection()
	a := mustParse(t, testName("a"), mustFileYAML(t, metaV1(), metaV2(), v1ToV2Steps))

	t.Run("unapplied file runs", func(t *testing.T) {
		plan, err := Resolve(v1, nil, v2, []*File{a})
		if err != nil {
			t.Fatalf("Resolve: %v", err)
		}
		if len(plan) != 1 || plan[0].Name != a.Name {
			t.Fatalf("plan = %v, want just %s", names(plan), a.Name)
		}
	})

	t.Run("applied file is skipped", func(t *testing.T) {
		plan, err := Resolve(v2, []string{a.Name}, v2, []*File{a})
		if err != nil {
			t.Fatalf("Resolve: %v", err)
		}
		if len(plan) != 0 {
			t.Fatalf("plan = %v, want empty", names(plan))
		}
	})

	t.Run("a data-only file runs like any other", func(t *testing.T) {
		// Same shape on both sides: under the old hash-edge model this file
		// could not even be parsed, let alone planned.
		d := mustParse(t, testName("backfill"), mustFileYAML(t, metaV2(), metaV2(),
			"  - set_default: {entity: task, property: state, value: todo}\n"))
		plan, err := Resolve(v2, nil, v2, []*File{d})
		if err != nil {
			t.Fatalf("Resolve: %v", err)
		}
		if len(plan) != 1 || plan[0].Name != d.Name {
			t.Fatalf("plan = %v, want the data-only migration", names(plan))
		}
	})
}

// Files run in name order, which for timestamp-prefixed names is creation
// order. LoadDir sorts them; Resolve must preserve that.
func TestResolve_PreservesFileOrder(t *testing.T) {
	v1, v2 := metaV1().ShapeProjection(), metaV2().ShapeProjection()
	first := mustParse(t, "20260101000000-first.yaml", mustFileYAML(t, metaV1(), metaV2(), v1ToV2Steps))
	second := mustParse(t, "20260202000000-second.yaml", mustFileYAML(t, metaV2(), metaV2(),
		"  - set_default: {entity: task, property: state, value: todo}\n"))

	plan, err := Resolve(v1, nil, v2, []*File{first, second})
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if len(plan) != 2 || plan[0].Name != first.Name || plan[1].Name != second.Name {
		t.Fatalf("plan = %v, want [first second]", names(plan))
	}
}

// The residual check is a diagnostic, not the planner: it is how an operator
// learns they edited schema.yaml without writing the migration it needs.
func TestResolve_RefusesAnUncoveredSchemaChange(t *testing.T) {
	v1, v2 := metaV1().ShapeProjection(), metaV2().ShapeProjection()
	if _, err := Resolve(v1, nil, v2, nil); err == nil {
		t.Fatal("a needs-migration gap with no migration files must be refused")
	}

	// Covered by a file, the same gap resolves.
	a := mustParse(t, testName("a"), mustFileYAML(t, metaV1(), metaV2(), v1ToV2Steps))
	if _, err := Resolve(v1, nil, v2, []*File{a}); err != nil {
		t.Fatalf("a covered gap must resolve: %v", err)
	}
}

// An additive change needs no migration, so an empty plan is success.
func TestResolve_AdditiveGapNeedsNoMigration(t *testing.T) {
	m2 := metaV1()
	m2.Entities["task"].Properties["estimate"] = metamodel.PropertyDef{Type: "integer"}
	plan, err := Resolve(metaV1().ShapeProjection(), nil, m2.ShapeProjection(), nil)
	if err != nil {
		t.Fatalf("an additive gap must resolve with no migration: %v", err)
	}
	if len(plan) != 0 {
		t.Fatalf("plan = %v, want empty", names(plan))
	}
}

func names(files []*File) []string {
	out := make([]string, 0, len(files))
	for _, f := range files {
		out = append(out, f.Name)
	}
	return out
}
