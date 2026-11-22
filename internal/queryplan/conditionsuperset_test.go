package queryplan_test

import (
	"context"
	"fmt"
	"math/rand/v2"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/predicatefns"
	"github.com/Sourcehaven-BV/rela/internal/queryplan"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/store/memstore"
)

// ConditionSuperset keeps the top-level conjuncts that lower exactly and
// leaves the rest to the caller's Go evaluation (TKT-I3MGCRT).
func TestConditionSuperset_Shapes(t *testing.T) {
	meta := conditionMeta()
	tests := []struct {
		src, props, where string
	}{
		{src: "entity.status ~= 'done' and entity.due < today()", props: "status~=done"},
		{src: "entity.due < today() and (entity.status == 'a' or entity.count > 3)",
			where: "or(status=a, count>3#int)"},
		{src: "entity.count == 3 and entity.price > 3 and not (entity.status < 'b')",
			props: "count>=3#int count<=3#int", where: "not(status<b#str)"},
		// A conjunct holding a traversal is left out: it would need gating.
		{src: "related(entity, 'implements') and entity.status == 'a' and entity.due < today()",
			props: "status=a"},
		{src: "(entity.status == 'a' or related(entity, 'implements')) and entity.due < today()"},
		// Not an AND: one conjunct, which does not lower.
		{src: "entity.status == 'a' or entity.due < today()"},
		{src: "not (entity.status == 'a' and entity.due < today())"},
		// A condition LowerCondition accepts gives the same filter.
		{src: "entity.status == 'a' and (entity.count > 1 or entity.status ~= 'b')",
			props: "status=a", where: "or(count>1#int, status~=b)"},
	}
	ev := predicatefns.NewEvaluator(meta)
	for _, tc := range tests {
		t.Run(tc.src, func(t *testing.T) {
			prog, err := ev.CompileWithCurrentUser("task", tc.src)
			if err != nil {
				t.Fatalf("compile: %v", err)
			}
			got := queryplan.ConditionSuperset(prog, meta, "task", "")
			if len(got.Traversals) > 0 {
				t.Fatalf("traversals = %v, want none", got.Traversals)
			}
			var props []string
			for _, p := range got.Props {
				props = append(props, renderProp(p))
			}
			if g := strings.Join(props, " "); g != tc.props {
				t.Errorf("props = %q, want %q", g, tc.props)
			}
			where := ""
			if got.Where != nil {
				where = renderLowered(*got.Where)
			}
			if where != tc.where {
				t.Errorf("where = %q, want %q", where, tc.where)
			}
		})
	}
}

func TestConditionSuperset_NilInputs(t *testing.T) {
	meta := conditionMeta()
	prog, err := predicatefns.NewEvaluator(meta).CompileWithCurrentUser("task", "entity.status ~= 'a'")
	if err != nil {
		t.Fatal(err)
	}
	for name, got := range map[string]queryplan.ConditionLowering{
		"nil program":   queryplan.ConditionSuperset(nil, meta, "task", ""),
		"nil metamodel": queryplan.ConditionSuperset(prog, nil, "task", ""),
		"empty type":    queryplan.ConditionSuperset(prog, meta, "", ""),
	} {
		if len(got.Props) > 0 || got.Where != nil || len(got.Traversals) > 0 {
			t.Errorf("%s: %+v, want the zero value", name, got)
		}
	}
}

// Conjuncts that each fit the store's limits but together do not are
// dropped until the AND fits, so the filter is never refused.
func TestConditionSuperset_StaysWithinCondLimits(t *testing.T) {
	meta := conditionMeta()
	// Each `~=` on an integer lowers to two leaves. Balanced, so the
	// compiler's own nesting limit does not refuse it first.
	var balanced func(lo, hi int) string
	balanced = func(lo, hi int) string {
		if hi-lo == 1 {
			return fmt.Sprintf("entity.count ~= %d", lo)
		}
		mid := (lo + hi) / 2
		return "(" + balanced(lo, mid) + " and " + balanced(mid, hi) + ")"
	}
	n := store.MaxCondLeaves/2 + 20
	src := balanced(0, n) + " and entity.due < today()"
	prog, err := predicatefns.NewEvaluator(meta).CompileWithCurrentUser("task", src)
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	got := queryplan.ConditionSuperset(prog, meta, "task", "")
	if got.Where == nil {
		t.Fatal("nothing pushed")
	}
	c, ok := got.Where.PropCond()
	if !ok {
		t.Fatal("where holds a traversal")
	}
	if err := store.ValidateCond(c); err != nil {
		t.Fatalf("filter is refused: %v", err)
	}
	if kept := len(c.Args); kept >= n || kept < store.MaxCondLeaves/2-1 {
		t.Errorf("kept %d of %d conjuncts, want as many as fit", kept, n)
	}
}

func TestLoweredCond_PropCond(t *testing.T) {
	meta := conditionMeta()
	ev := predicatefns.NewEvaluator(meta)
	for src, wantOK := range map[string]bool{
		"not (entity.status == 'a') or entity.count > 1":        true,
		"entity.status == 'a' or related(entity, 'implements')": false,
	} {
		prog, err := ev.CompileWithCurrentUser("task", src)
		if err != nil {
			t.Fatalf("compile %q: %v", src, err)
		}
		low, ok := queryplan.LowerCondition(prog, meta, "task", "")
		if !ok || low.Where == nil {
			t.Fatalf("%q: did not lower to a tree", src)
		}
		if _, ok := low.Where.PropCond(); ok != wantOK {
			t.Errorf("%q: PropCond ok = %v, want %v", src, ok, wantOK)
		}
	}
}

// supersetLeaves are leaves no lowering answers: a date against today(), a
// decimal ordered comparison and a comparison of two attributes.
var supersetLeaves = []string{
	"entity.due < today()", "entity.due >= today()", "entity.price > 2", "entity.status == entity.assignee",
}

// TestConditionSuperset_KeepsEveryMatch is the differential for the
// superset claim: on every generated condition mixing leaves that lower
// with leaves that do not, each row the Go evaluator matches is in the
// store's result, under LoweringSemantics and the default semantics, and
// the filter keeps no row ConditionPrefilters drops.
// Where the whole condition lowers, the filter also returns
// exactly the Go matches.
func TestConditionSuperset_KeepsEveryMatch(t *testing.T) {
	meta := conditionMeta()
	ctx := context.Background()
	st := memstore.New()
	now := time.Date(2026, 10, 4, 12, 0, 0, 0, time.UTC)
	dates := []any{"absent", "2026-10-01", "2026-10-04", "2026-12-01", "", "TBD"}
	var rows []*entity.Entity
	for i := range 60 {
		e := entity.New(fmt.Sprintf("T-%02d", i), "task")
		for j, p := range []string{"status", "count", "assignee", "price"} {
			v := diffValues[(i*(j+3)+j*7)%len(diffValues)]
			if v != "absent" {
				e.Properties[p] = v
			}
		}
		if d := dates[i%len(dates)]; d != "absent" {
			e.Properties["due"] = d
		}
		if err := st.CreateEntity(ctx, e); err != nil {
			t.Fatal(err)
		}
		rows = append(rows, e)
	}

	defaults := predicatefns.NewEvaluatorWithClock(meta, func() time.Time { return now })
	ev := defaults.LoweringSemantics()
	leaves := slices.Concat(diffLeaves, supersetLeaves)
	narrowed := 0
	for i := range 400 {
		rng := rand.New(rand.NewPCG(uint64(i), 0x5E7))
		src := genFrom(rng, leaves, 0)
		prog, err := ev.CompileWithCurrentUser("task", src)
		if err != nil {
			t.Fatalf("compile %q: %v", src, err)
		}
		got := runLowered(t, st, meta, queryplan.ConditionSuperset(prog, meta, "task", ""))
		var want []string
		for _, e := range rows {
			// An error counts as no match, as on the desktop.
			if ok, err := ev.Matches(ctx, prog, "task", e.ID, e.Properties); err == nil && ok {
				want = append(want, e.ID)
			}
			// The claim does not depend on LoweringSemantics: a row the
			// default semantics match is kept too.
			ok, err := defaults.Matches(ctx, prog, "task", e.ID, e.Properties)
			if err == nil && ok && !slices.Contains(got, e.ID) {
				t.Errorf("%q: default semantics match %s, the filter drops it", src, e.ID)
			}
		}
		for _, id := range want {
			if !slices.Contains(got, id) {
				t.Errorf("%q: Go matches %s, the filter drops it", src, id)
			}
		}
		// The filter replaces ConditionPrefilters on the desktop, so it
		// must narrow at least as far.
		pre := runLowered(t, st, meta, queryplan.ConditionLowering{
			Props: queryplan.ConditionPrefilters(prog, meta, []string{"task"}, ""),
		})
		for _, id := range got {
			if !slices.Contains(pre, id) {
				t.Errorf("%q: the filter keeps %s, which ConditionPrefilters drops", src, id)
			}
		}
		_, exact := queryplan.LowerCondition(prog, meta, "task", "")
		if exact && !slices.Equal(want, got) {
			t.Errorf("%q lowers exactly:\n  go     %v\n  filter %v", src, want, got)
		}
		if !exact && len(want) > 0 && len(got) < len(rows) {
			narrowed++
		}
	}
	// Guard the generator: the claim must be exercised on conditions that
	// do not lower, match something, and are narrowed by the filter.
	if narrowed < 20 {
		t.Fatalf("only %d conditions exercised a narrowing filter", narrowed)
	}
}

func genFrom(rng *rand.Rand, leaves []string, depth int) string {
	n := rng.IntN(6)
	if depth >= 3 {
		n = 0
	}
	switch n {
	case 0, 1:
		return leaves[rng.IntN(len(leaves))]
	case 2, 3:
		return "(" + genFrom(rng, leaves, depth+1) + " and " + genFrom(rng, leaves, depth+1) + ")"
	case 4:
		return "(" + genFrom(rng, leaves, depth+1) + " or " + genFrom(rng, leaves, depth+1) + ")"
	default:
		return "not (" + genFrom(rng, leaves, depth+1) + ")"
	}
}
