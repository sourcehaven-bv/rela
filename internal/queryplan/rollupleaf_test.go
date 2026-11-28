package queryplan_test

import (
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/predicatefns"
	"github.com/Sourcehaven-BV/rela/internal/queryplan"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// leafMeta is rollupMeta with boolean roll-ups on task, one per shape the
// leaf decision tells apart, and an enum maximum.
func leafMeta() *metamodel.Metamodel {
	meta := rollupMeta()
	task := meta.Entities["task"]
	for _, p := range []string{"held", "held_deferred", "held_nested", "waiting", "waiting_gated", "held_max"} {
		task.Properties[p] = metamodel.PropertyDef{
			Type:   metamodel.PropertyTypeBoolean,
			Rollup: &metamodel.RollupDef{Via: metamodel.StringOrSlice{"blocks"}, Exists: true},
		}
	}
	meta.Entities["task"] = task
	return meta
}

// leafFacts are the facts rollup.Set.LeafFacts gives for each leafMeta
// roll-up: an exact along, one whose node condition reads a defer date,
// a nested one, a siblings one with and without an order field grant, and
// a max.
func leafFacts(_, property string) (queryplan.RollupLeafFacts, bool) {
	f, ok := map[string]queryplan.RollupLeafFacts{
		"held":          {Exists: true, Exact: true},
		"held_deferred": {Exists: true},
		"held_nested":   {Exists: true, Exact: true, Nested: true},
		"waiting":       {Exists: true, Exact: true, Siblings: true},
		"waiting_gated": {Exists: true, Exact: true, Siblings: true, OrderFieldGated: true},
		"held_max":      {Exact: true},
	}[property]
	return f, ok
}

// TestLowerConditionRollups_Lowers: `== true` and `== false` on an exact
// exists roll-up lower to a roll-up leaf, beside stored comparisons.
func TestLowerConditionRollups_Lowers(t *testing.T) {
	t.Parallel()
	meta := leafMeta()
	ev := predicatefns.NewEvaluator(meta)
	for _, tc := range []struct {
		src   string
		want  []queryplan.LoweredRollup
		props int
	}{
		{"entity.held == false", []queryplan.LoweredRollup{{Property: "held"}}, 0},
		{"entity.held == true", []queryplan.LoweredRollup{{Property: "held", Holds: true}}, 0},
		{"true == entity.waiting", []queryplan.LoweredRollup{{Property: "waiting", Holds: true}}, 0},
		{
			"entity.held == false and entity.waiting == false and entity.status == 'open'",
			[]queryplan.LoweredRollup{{Property: "held"}, {Property: "waiting"}}, 1,
		},
	} {
		t.Run(tc.src, func(t *testing.T) {
			t.Parallel()
			prog, err := ev.CompileWithRollups("task", tc.src)
			if err != nil {
				t.Fatal(err)
			}
			got, err := queryplan.LowerConditionRollups(prog, meta, "task", "PERS-JV", leafFacts)
			if err != nil {
				t.Fatal(err)
			}
			var leaves []queryplan.LoweredRollup
			props := len(got.Props)
			if got.Where != nil {
				leaves, props = loweredRollups(*got.Where, leaves, props)
			}
			if len(leaves) != len(tc.want) {
				t.Fatalf("leaves = %+v; want %+v", leaves, tc.want)
			}
			for i := range leaves {
				if leaves[i] != tc.want[i] {
					t.Errorf("leaf %d = %+v; want %+v", i, leaves[i], tc.want[i])
				}
			}
			if props != tc.props {
				t.Errorf("stored comparisons = %d; want %d", props, tc.props)
			}
		})
	}
}

// TestLowerConditionRollups_Declines: min and max, a defer-date node
// condition, a defer comparison in the list condition, a nested roll-up,
// waiting under an order field grant, another operator, and a lowering
// without facts all keep the Go path, and the error says why.
func TestLowerConditionRollups_Declines(t *testing.T) {
	t.Parallel()
	meta := leafMeta()
	ev := predicatefns.NewEvaluator(meta)
	for _, tc := range []struct {
		src, reason string
		facts       queryplan.RollupFacts
	}{
		{"entity.held_max == true", "only an exists roll-up", leafFacts},
		{"entity.first_due == '2026-01-01'", "", leafFacts},
		{"entity.held_deferred == false", "no exact store form", leafFacts},
		{"entity.held == false and entity.due > '2026-01-01'", "", leafFacts},
		{"entity.held_nested == false", "roll-up over a roll-up", leafFacts},
		{"entity.waiting_gated == false", "order field", leafFacts},
		{"entity.held ~= true", "", leafFacts},
		{"entity.held == false", "not stored", nil},
	} {
		t.Run(tc.src, func(t *testing.T) {
			t.Parallel()
			prog, err := ev.CompileWithRollups("task", tc.src)
			if err != nil {
				t.Fatal(err)
			}
			_, err = queryplan.LowerConditionRollups(prog, meta, "task", "PERS-JV", tc.facts)
			if err == nil {
				t.Fatal("lowered; want the Go path")
			}
			if !strings.Contains(err.Error(), tc.reason) {
				t.Errorf("error %q does not say %q", err, tc.reason)
			}
		})
	}
}

// loweredRollups appends the roll-up leaves of c in order and counts its
// property leaves onto props.
func loweredRollups(
	c queryplan.LoweredCond, out []queryplan.LoweredRollup, props int,
) (rollups []queryplan.LoweredRollup, propLeaves int) {
	switch c.Op {
	case store.CondRollup:
		return append(out, c.Rollup), props
	case store.CondProp:
		return out, props + 1
	case store.CondRelated, store.CondAnd, store.CondOr, store.CondNot:
	}
	for _, a := range c.Args {
		out, props = loweredRollups(a, out, props)
	}
	return out, props
}
