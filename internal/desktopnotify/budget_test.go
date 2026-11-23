package desktopnotify

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"slices"
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/store/storetest"
)

func newTaak(id, status, due string) *entity.Entity {
	return &entity.Entity{ID: id, Type: "taak", Properties: map[string]any{"title": id, "status": status, "vervaldatum": due}}
}

// padTaken adds n taken that no rule in these tests matches: same type and
// face as the matches, which is what a whole-type scan would return.
func padTaken(t *testing.T, st store.Store, n int) {
	t.Helper()
	ctx := context.Background()
	for i := range n {
		if err := st.CreateEntity(ctx, newTaak(fmt.Sprintf("TAAK-PAD-%04d", i), "done", "2026-01-01")); err != nil {
			t.Fatalf("CreateEntity: %v", err)
		}
	}
}

// measureEvaluate builds a store with the fixture plus n padding rows, runs
// Evaluate once through a Counting store, and checks its output.
func measureEvaluate(
	t *testing.T, cfg *Config, fixture []taak, wantMatches, wantBadge int,
) func(n int) (storetest.Cost, string) {
	t.Helper()
	return func(n int) (storetest.Cost, string) {
		st := newStore(t, fixture...)
		padTaken(t, st, n)
		counting := storetest.NewCounting(st)
		res, err := cfg.Evaluate(context.Background(), counting, store.TrivialScope())
		if err != nil {
			t.Fatalf("Evaluate: %v", err)
		}
		if len(res.Matches) != wantMatches || res.Badge != wantBadge {
			t.Fatalf("at %d: %d matches, badge %d; want %d, %d", n, len(res.Matches), res.Badge, wantMatches, wantBadge)
		}
		return counting.Cost(), counting.String()
	}
}

var openTaken = []taak{
	{"TAAK-1", "One", "open", "2026-10-01"},
	{"TAAK-2", "Two", "open", "2026-10-20"},
	{"TAAK-3", "Three", "open", ""},
}

// A condition that lowers exactly is answered by the store: the rule reads
// its matches only, and the badge is a count.
func TestEvaluate_LoweredConditionIsScaleFree(t *testing.T) {
	t.Parallel()
	cfg := load(t, `
notifications:
  - id: open
    type: taak
    condition: "entity.status == 'open'"
badge:
  type: taak
  condition: "entity.status == 'open'"
`)
	cost := storetest.AssertScaleFree(t, measureEvaluate(t, cfg, openTaken, 3, 3))
	if want := (storetest.Cost{Calls: 2, Rows: 3}); cost != want {
		t.Errorf("cost = %s, want %s (one capped query, one count)", cost, want)
	}
}

// A condition that does not lower still pushes its equalities, so rows they
// exclude are never read. The rule and the badge share the read.
func TestEvaluate_PrefilteredConditionSkipsExcludedRows(t *testing.T) {
	t.Parallel()
	cfg := load(t, `
notifications:
  - id: due
    type: taak
    condition: "entity.status == 'open' and entity.vervaldatum <= today()"
badge:
  type: taak
  condition: "entity.vervaldatum <= today() and entity.status == 'open'"
`)
	cost := storetest.AssertScaleFree(t, measureEvaluate(t, cfg, openTaken, 1, 1))
	if want := (storetest.Cost{Calls: 1, Rows: 3}); cost != want {
		t.Errorf("cost = %s, want %s (one read of the open taken)", cost, want)
	}
}

// or, not, ~= and ordered comparisons are answered by the store too
// (TKT-I3MGCRT): one capped query and one count, however many rows the
// type has.
func TestEvaluate_LoweredTreeIsScaleFree(t *testing.T) {
	t.Parallel()
	for _, condition := range []string{
		"entity.status == 'open' or entity.status == 'new'",
		"entity.status ~= 'done'",
		"not (entity.status == 'done' or entity.status < 'b')",
		"entity.status > 'n' and entity.status < 'p'",
	} {
		t.Run(condition, func(t *testing.T) {
			t.Parallel()
			cfg := load(t, fmt.Sprintf(`
notifications:
  - id: open
    type: taak
    condition: %q
badge:
  type: taak
  condition: %q
`, condition, condition))
			cost := storetest.AssertScaleFree(t, measureEvaluate(t, cfg, openTaken, 3, 3))
			if want := (storetest.Cost{Calls: 2, Rows: 3}); cost != want {
				t.Errorf("cost = %s, want %s (one capped query, one count)", cost, want)
			}
		})
	}
}

// A date condition is still judged in Go, but its `~=` conjunct narrows the
// read: rows it excludes are never read. Rows that pass it are, so the
// scan is real and its cost grows with them.
func TestEvaluate_DateConditionScanIsNarrowedByLoweredConjuncts(t *testing.T) {
	t.Parallel()
	const condition = "entity.status ~= 'done' and entity.vervaldatum <= today()"
	cfg := load(t, fmt.Sprintf(`
notifications:
  - id: due
    type: taak
    condition: %q
badge:
  type: taak
  condition: %q
`, condition, condition))
	if cfg.rules[0].exact {
		t.Fatal("the date condition lowered exactly; this test needs the Go path")
	}
	cost := storetest.AssertScaleFree(t, measureEvaluate(t, cfg, openTaken, 1, 1))
	if want := (storetest.Cost{Calls: 1, Rows: 3}); cost != want {
		t.Errorf("cost = %s, want %s (one read of the taken not done)", cost, want)
	}

	measureOpen := func(n int) storetest.Cost {
		st := newStore(t, openTaken...)
		ctx := context.Background()
		for i := range n {
			if err := st.CreateEntity(ctx, newTaak(fmt.Sprintf("TAAK-OPEN-%04d", i), "open", "2027-01-01")); err != nil {
				t.Fatalf("CreateEntity: %v", err)
			}
		}
		counting := storetest.NewCounting(st)
		if _, err := cfg.Evaluate(ctx, counting, store.TrivialScope()); err != nil {
			t.Fatalf("Evaluate: %v", err)
		}
		return counting.Cost()
	}
	if small, large := measureOpen(10), measureOpen(50); large.Rows-small.Rows != 40 {
		t.Errorf("rows %d at 10, %d at 50; want the scan to read every row not done", small.Rows, large.Rows)
	}
}

// A lowered rule reads a capped page however many entities match, so its
// rows do not grow with the matches either.
func TestEvaluate_LoweredRuleReadsAtMostTheCap(t *testing.T) {
	t.Parallel()
	cfg := load(t, `
notifications:
  - id: open
    type: taak
    condition: "entity.status == 'open'"
badge:
  type: taak
  condition: "entity.status == 'open'"
`)
	measure := func(n int) (storetest.Cost, string) {
		st := newStore(t)
		ctx := context.Background()
		for i := range n {
			if err := st.CreateEntity(ctx, newTaak(fmt.Sprintf("TAAK-%05d", i), "open", "")); err != nil {
				t.Fatalf("CreateEntity: %v", err)
			}
		}
		counting := storetest.NewCounting(st)
		res, err := cfg.Evaluate(ctx, counting, store.TrivialScope())
		if err != nil {
			t.Fatalf("Evaluate: %v", err)
		}
		if res.Badge != n || !slices.Equal(res.Truncated, []string{"open"}) {
			t.Fatalf("at %d: badge %d, truncated %v", n, res.Badge, res.Truncated)
		}
		return counting.Cost(), counting.String()
	}
	const small, large = MaxMatchesPerRule + 100, 2 * MaxMatchesPerRule
	s, sDetail := measure(small)
	l, lDetail := measure(large)
	storetest.AssertSameCost(t, small, s, sDetail, large, l, lDetail)
	if want := (storetest.Cost{Calls: 2, Rows: MaxMatchesPerRule + 1}); l != want {
		t.Errorf("cost = %s, want %s", l, want)
	}
}

func TestEvaluate_CapsMatchesPerRule(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name      string
		condition string
	}{
		{"lowered", "entity.status == 'open'"},
		{"judged in Go", "entity.status == 'open' and entity.vervaldatum <= today()"},
	}
	const total = MaxMatchesPerRule + 5
	taken := make([]taak, 0, total+1)
	for i := range total {
		taken = append(taken, taak{fmt.Sprintf("TAAK-%04d", i), "t", "open", "2026-10-01"})
	}
	// The "few" rule stays under the cap and still reports.
	taken = append(taken, taak{"TAAK-X", "t", "few", ""})
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			cfg := load(t, fmt.Sprintf(`
notifications:
  - id: a
    type: taak
    condition: %q
  - id: few
    type: taak
    condition: "entity.status == 'few'"
badge:
  type: taak
  condition: %q
`, tc.condition, tc.condition))
			res, err := cfg.Evaluate(context.Background(), newStore(t, taken...), store.TrivialScope())
			if err != nil {
				t.Fatalf("Evaluate: %v", err)
			}
			if len(res.Matches) != 1 || res.Matches[0].RuleID != "few" {
				t.Fatalf("matches = %+v, want only rule few's", res.Matches)
			}
			if !slices.Equal(res.Truncated, []string{"a"}) {
				t.Errorf("Truncated = %v, want [a]", res.Truncated)
			}
			if res.Badge != total {
				t.Errorf("badge = %d, want %d (the badge is not capped)", res.Badge, total)
			}
		})
	}
}

// A rule exactly at the cap is not truncated.
func TestEvaluate_AtCapIsNotTruncated(t *testing.T) {
	t.Parallel()
	var taken []taak
	for i := range MaxMatchesPerRule {
		taken = append(taken, taak{fmt.Sprintf("TAAK-%04d", i), "t", "open", ""})
	}
	cfg := load(t, "notifications:\n  - id: a\n    type: taak\n    condition: \"entity.status == 'open'\"\n")
	res, err := cfg.Evaluate(context.Background(), newStore(t, taken...), store.TrivialScope())
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	if len(res.Matches) != MaxMatchesPerRule || len(res.Truncated) != 0 {
		t.Fatalf("matches = %d, truncated = %v; want %d, none", len(res.Matches), res.Truncated, MaxMatchesPerRule)
	}
}

func TestLower(t *testing.T) {
	t.Parallel()
	tests := []struct {
		condition string
		exact     bool
		filter    string
	}{
		{"entity.status == 'open'", true, "status=open"},
		{"entity.title == 'x' and entity.status == 'open'", true, "status=open title=x"},
		{"entity.status == 'open' or entity.status == 'new'", true, "or(status=open, status=new)"},
		{"entity.status ~= 'done'", true, "status~=done"},
		{"not (entity.status == 'done')", true, "not(status=done)"},
		{"entity.status < 'b'", true, "status<b"},
		{"entity.punten >= 3", true, "punten>=3"},
		{"true", true, ""},
		// A date against today() is judged in Go; the conjuncts that lower
		// still narrow the read.
		{"entity.status == 'open' and entity.vervaldatum <= today()", false, "status=open"},
		{"entity.status ~= 'done' and entity.vervaldatum <= today()", false, "status~=done"},
		{"entity.vervaldatum <= today() and (entity.status == 'a' or entity.punten > 1)", false,
			"or(status=a, punten>1)"},
		{"entity.status == 'open' or entity.vervaldatum <= today()", false, ""},
	}
	for _, tc := range tests {
		t.Run(tc.condition, func(t *testing.T) {
			t.Parallel()
			cfg := load(t, fmt.Sprintf("badge:\n  type: taak\n  condition: %q\n", tc.condition))
			got := renderFilter(cfg.badge.pushed, cfg.badge.where)
			if cfg.badge.exact != tc.exact || got != tc.filter {
				t.Fatalf("exact = %v, filter = %q; want %v, %q", cfg.badge.exact, got, tc.exact, tc.filter)
			}
		})
	}
}

// renderFilter writes a condition's store filter for comparison: the pushed
// predicates, then the where tree.
func renderFilter(pushed []store.PropPredicate, where *store.Cond) string {
	var parts []string
	for _, p := range pushed {
		parts = append(parts, renderPred(p))
	}
	if where != nil {
		parts = append(parts, renderCond(*where))
	}
	return strings.Join(parts, " ")
}

func renderPred(p store.PropPredicate) string {
	op := map[store.PropOp]string{
		store.PropEqual: "=", store.PropNotEqualOrEmpty: "~=", store.PropLess: "<", store.PropLessEqual: "<=",
		store.PropGreater: ">", store.PropGreaterEqual: ">=",
	}[p.Op]
	return p.Property + op + p.Value
}

func renderCond(c store.Cond) string {
	var args []string
	for _, a := range c.Args {
		args = append(args, renderCond(a))
	}
	switch c.Op {
	case store.CondProp:
		return renderPred(c.Prop)
	case store.CondAnd:
		return "and(" + strings.Join(args, ", ") + ")"
	case store.CondOr:
		return "or(" + strings.Join(args, ", ") + ")"
	default:
		return "not(" + strings.Join(args, ", ") + ")"
	}
}

func TestTracker_TruncatedRuleStartsOver(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	tr, err := NewTracker(newKV(t))
	if err != nil {
		t.Fatalf("NewTracker: %v", err)
	}
	// Rule a is never truncated; rule b goes over the cap and back.
	steps := []struct {
		name       string
		a, b       []string
		bTruncated bool
		want       []string
	}{
		{name: "first run is silent", a: []string{"A-1"}, b: []string{"B-1"}},
		{
			name: "new matches notify", a: []string{"A-1", "A-2"}, b: []string{"B-1", "B-2"},
			want: []string{"a:A-2", "b:B-2"},
		},
		{
			name: "truncated rule notifies nothing", a: []string{"A-1", "A-2", "A-3"}, bTruncated: true,
			want: []string{"a:A-3"},
		},
		{name: "still truncated", a: []string{"A-1", "A-2", "A-3"}, bTruncated: true},
		{name: "back under the cap is silent", a: []string{"A-1", "A-2", "A-3"}, b: []string{"B-1", "B-2", "B-3"}},
		{
			name: "then a new match notifies", a: []string{"A-1"}, b: []string{"B-1", "B-2", "B-3", "B-4"},
			want: []string{"b:B-4"},
		},
	}
	for _, step := range steps {
		res := Result{Rules: []string{"a", "b"}}
		for _, id := range step.a {
			res.Matches = append(res.Matches, Match{RuleID: "a", EntityID: id})
		}
		for _, id := range step.b {
			res.Matches = append(res.Matches, Match{RuleID: "b", EntityID: id})
		}
		if step.bTruncated {
			res.Truncated = []string{"b"}
		}
		fresh, err := tr.Update(ctx, res)
		if err != nil {
			t.Fatalf("%s: Update: %v", step.name, err)
		}
		var got []string
		for _, m := range fresh {
			got = append(got, m.RuleID+":"+m.EntityID)
		}
		if !slices.Equal(got, step.want) {
			t.Fatalf("%s: new = %v, want %v", step.name, got, step.want)
		}
	}
}

// A rule that is over the cap the first time it runs leaves no record.
func TestTracker_TruncatedFirstRunWritesNothing(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	kv := newKV(t)
	tr, err := NewTracker(kv)
	if err != nil {
		t.Fatalf("NewTracker: %v", err)
	}
	if _, err := tr.Update(ctx, Result{Rules: []string{"a"}, Truncated: []string{"a"}}); err != nil {
		t.Fatalf("Update: %v", err)
	}
	if _, err := kv.Get(ctx, stateKeyPrefix+"a"); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("Get = %v, want no record", err)
	}
}
