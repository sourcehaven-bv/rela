package queryplan_test

import (
	"context"
	"fmt"
	"math/rand/v2"
	"slices"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/predicatefns"
	"github.com/Sourcehaven-BV/rela/internal/queryplan"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/store/memstore"
)

// diffValues is the raw value mix the differential seeds every property
// with: unset, JSON null, the empty string, integer-shaped strings, reals
// that are and are not integral, numbers past int64, booleans and lists.
var diffValues = []any{
	"absent", nil, "", "a", "b", "Zed", "7", "+3", "007", " 7", "x",
	1, 3, 5, -4, 1.0, 2.5, 1e19, 9223372036854775807.0, true, false,
	[]any{"a"}, []any{}, map[string]any{"id": "a"},
}

// diffLeaves are the comparison leaves the generator combines.
var diffLeaves = []string{
	"entity.status == 'a'", "entity.status ~= 'a'", "entity.status < 'b'", "entity.status >= ''",
	"'b' <= entity.status", "entity.status > 'Zed'",
	"entity.count == 3", "entity.count ~= 3", "entity.count > 3", "entity.count <= 5",
	"-1 < entity.count", "entity.count >= 7", "entity.count < 0",
	"true", "false",
}

// TestLowerCondition_AgreesWithEvaluator is the differential between the
// two answers to one condition (TKT-8HY05WB): the Go evaluator with
// LoweringSemantics on each row, and LowerCondition's predicates run as a
// store query. Store backends are held to graphquerynaive by
// storetest.RunGraphDifferential, so memstore stands for all of them here.
// The Go side must also never fail: a store query cannot report an error
// per row.
func TestLowerCondition_AgreesWithEvaluator(t *testing.T) {
	meta := conditionMeta()
	ctx := context.Background()
	st := memstore.New()
	var rows []*entity.Entity
	for i := range 60 {
		e := entity.New(fmt.Sprintf("T-%02d", i), "task")
		for j, p := range []string{"status", "count"} {
			v := diffValues[(i*(j+3)+j*7)%len(diffValues)]
			if v != "absent" {
				e.Properties[p] = v
			}
		}
		if err := st.CreateEntity(ctx, e); err != nil {
			t.Fatal(err)
		}
		rows = append(rows, e)
	}

	ev := predicatefns.NewEvaluator(meta).LoweringSemantics()
	for i := range 300 {
		rng := rand.New(rand.NewPCG(uint64(i), 0x8B5))
		src := genCondition(rng, 0)
		prog, err := ev.CompileWithCurrentUser("task", src)
		if err != nil {
			t.Fatalf("compile %q: %v", src, err)
		}
		var want []string
		for _, e := range rows {
			ok, err := ev.Matches(ctx, prog, "task", e.ID, e.Properties)
			if err != nil {
				t.Fatalf("%q on %s (%v): Go evaluation failed: %v", src, e.ID, e.Properties, err)
			}
			if ok {
				want = append(want, e.ID)
			}
		}
		lowered, ok := queryplan.LowerCondition(prog, meta, "task", "")
		if !ok {
			t.Fatalf("%q did not lower", src)
		}
		got := runLowered(t, st, meta, lowered)
		if !slices.Equal(want, got) {
			t.Errorf("%q:\n  go    %v\n  store %v", src, want, got)
		}
	}
}

func genCondition(rng *rand.Rand, depth int) string {
	n := rng.IntN(5)
	if depth >= 3 {
		n = 0
	}
	switch n {
	case 0, 1:
		return diffLeaves[rng.IntN(len(diffLeaves))]
	case 2:
		return "(" + genCondition(rng, depth+1) + " and " + genCondition(rng, depth+1) + ")"
	case 3:
		return "(" + genCondition(rng, depth+1) + " or " + genCondition(rng, depth+1) + ")"
	default:
		return "not (" + genCondition(rng, depth+1) + ")"
	}
}

// runLowered runs a lowering without traversals and returns the ids.
func runLowered(t *testing.T, st store.Store, _ *metamodel.Metamodel, l queryplan.ConditionLowering) []string {
	t.Helper()
	if len(l.Traversals) > 0 {
		t.Fatal("the differential generates no traversals")
	}
	q := store.GraphQuery{EntityType: "task", Props: l.Props, Faces: store.InWorld(store.TrivialScope())}
	if l.Where != nil {
		c, ok := l.Where.PropCond()
		if !ok {
			t.Fatal("the differential generates no traversals")
		}
		q.Where = &c
	}
	var ids []string
	for e, err := range st.GraphQuery(context.Background(), q) {
		if err != nil {
			t.Fatal(err)
		}
		ids = append(ids, e.ID)
	}
	slices.Sort(ids)
	return ids
}
