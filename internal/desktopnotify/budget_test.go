package desktopnotify

import (
	"context"
	"fmt"
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
	var taken []taak
	// Created in reverse, so the result order cannot come from insertion.
	for i := total; i > 0; i-- {
		taken = append(taken, taak{fmt.Sprintf("TAAK-%04d", i), "t", "open", "2026-10-01"})
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			cfg := load(t, fmt.Sprintf(`
notifications:
  - id: a
    type: taak
    condition: %q
  - id: none
    type: taak
    condition: "entity.status == 'nope'"
badge:
  type: taak
  condition: %q
`, tc.condition, tc.condition))
			res, err := cfg.Evaluate(context.Background(), newStore(t, taken...), store.TrivialScope())
			if err != nil {
				t.Fatalf("Evaluate: %v", err)
			}
			if len(res.Matches) != MaxMatchesPerRule {
				t.Fatalf("matches = %d, want %d", len(res.Matches), MaxMatchesPerRule)
			}
			first, last := res.Matches[0].EntityID, res.Matches[MaxMatchesPerRule-1].EntityID
			if first != "TAAK-0001" || last != fmt.Sprintf("TAAK-%04d", MaxMatchesPerRule) {
				t.Errorf("matches run %s..%s, want the lowest ids", first, last)
			}
			if !slices.IsSortedFunc(res.Matches, func(a, b Match) int { return strings.Compare(a.EntityID, b.EntityID) }) {
				t.Error("matches are not ordered by id")
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
		pushed    string
	}{
		{"entity.status == 'open'", true, "status=open"},
		{"entity.title == 'x' and entity.status == 'open'", true, "status=open,title=x"},
		{"entity.status == 'open' and entity.vervaldatum <= today()", false, "status=open"},
		{"entity.status == 'open' or entity.status == 'new'", false, ""},
		{"entity.status ~= 'done'", false, ""},
		{"true", false, ""},
	}
	for _, tc := range tests {
		t.Run(tc.condition, func(t *testing.T) {
			t.Parallel()
			cfg := load(t, fmt.Sprintf("badge:\n  type: taak\n  condition: %q\n", tc.condition))
			got := make([]string, len(cfg.badge.pushed))
			for i, p := range cfg.badge.pushed {
				if p.Op != store.PropEqual {
					t.Fatalf("pushed %+v, want equalities only", p)
				}
				got[i] = p.Property + "=" + p.Value
			}
			if cfg.badge.exact != tc.exact || strings.Join(got, ",") != tc.pushed {
				t.Fatalf("exact = %v, pushed = %v; want %v, %q", cfg.badge.exact, got, tc.exact, tc.pushed)
			}
		})
	}
}

func TestTracker_TruncatedRuleKeepsIDsPastCap(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	tr, err := NewTracker(newKV(t))
	if err != nil {
		t.Fatalf("NewTracker: %v", err)
	}
	result := func(truncated bool, ids ...string) Result {
		res := Result{Rules: []string{"a"}}
		for _, id := range ids {
			res.Matches = append(res.Matches, Match{RuleID: "a", EntityID: id})
		}
		if truncated {
			res.Truncated = []string{"a"}
		}
		return res
	}
	steps := []struct {
		name string
		res  Result
		want []string
	}{
		{"first run is silent", result(false, "X-1", "X-2", "X-3", "X-4", "X-5"), nil},
		// X-2 stopped matching; X-4 and X-5 lie past the cap and are kept.
		{"truncated run", result(true, "X-1", "X-3"), nil},
		{"id kept past the cap does not notify", result(true, "X-1", "X-3", "X-4"), nil},
		{"id dropped under the cap notifies again", result(true, "X-1", "X-2", "X-3"), []string{"X-2"}},
		{"full run forgets", result(false, "X-1"), nil},
		{"forgotten id notifies", result(true, "X-1", "X-5"), []string{"X-5"}},
	}
	for _, step := range steps {
		fresh, err := tr.Update(ctx, step.res)
		if err != nil {
			t.Fatalf("%s: Update: %v", step.name, err)
		}
		var got []string
		for _, m := range fresh {
			got = append(got, m.EntityID)
		}
		if !slices.Equal(got, step.want) {
			t.Fatalf("%s: new = %v, want %v", step.name, got, step.want)
		}
	}
}
