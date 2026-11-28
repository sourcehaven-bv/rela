package queryplan_test

import (
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/predicatefns"
	"github.com/Sourcehaven-BV/rela/internal/queryplan"
)

// rollupMeta is conditionMeta with two string-shaped roll-ups on task: a
// picked enum and a date minimum. Both look like a stored string property
// to a lowering that ignores Rollup.
func rollupMeta() *metamodel.Metamodel {
	meta := conditionMeta()
	task := meta.Entities["task"]
	task.Properties["latest_status"] = metamodel.PropertyDef{
		Type: metamodel.PropertyTypeEnum, Values: []string{"open", "done"},
		Rollup: &metamodel.RollupDef{Via: metamodel.StringOrSlice{"blocks"}, Pick: "status", LatestBy: "due"},
	}
	task.Properties["first_due"] = metamodel.PropertyDef{
		Type:   metamodel.PropertyTypeDate,
		Rollup: &metamodel.RollupDef{Via: metamodel.StringOrSlice{"blocks"}, Min: "due"},
	}
	meta.Entities["task"] = task
	return meta
}

// TestLowering_DeclinesRollupLeaf: the store holds no roll-up value, so a
// condition or scope that reads one never lowers, and RollupRead names it.
func TestLowering_DeclinesRollupLeaf(t *testing.T) {
	t.Parallel()
	meta := rollupMeta()
	ev := predicatefns.NewEvaluator(meta)
	for _, tc := range []struct{ src, property string }{
		{"entity.latest_status == 'open'", "latest_status"},
		{"entity.first_due == '2026-01-01'", "first_due"},
		{"entity.status == 'open' and entity.latest_status == 'open'", "latest_status"},
	} {
		t.Run(tc.src, func(t *testing.T) {
			t.Parallel()
			prog, err := ev.CompileWithRollups("task", tc.src)
			if err != nil {
				t.Fatal(err)
			}
			if _, ok := queryplan.LowerCondition(prog, meta, "task", "PERS-JV"); ok {
				t.Error("LowerCondition lowered a roll-up read")
			}
			if _, ok := queryplan.LowerScope(prog, meta, "task", "PERS-JV"); ok {
				t.Error("LowerScope lowered a roll-up read")
			}
			if got, ok := queryplan.RollupRead(prog, meta, "task"); !ok || got != tc.property {
				t.Errorf("RollupRead = %q, %v; want %q", got, ok, tc.property)
			}
		})
	}
}
