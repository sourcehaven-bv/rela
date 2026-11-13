package transitions

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"slices"
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/flow"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
)

// A machine compiles to a headless flow: the enum values are the states in
// declaration order, the entry value is the initial state, and a value with
// no move to another value is final.
func TestCompile_HeadlessDefinition(t *testing.T) {
	set := mustCompile(t, snapshotMeta())
	m := set.machines["snapshot-status"]
	if got := m.def.Initial(); got != "in-review" {
		t.Fatalf("initial = %q, want in-review", got)
	}
	wantFinal := map[string]bool{"in-review": false, "approved": false, "established": false, "obsolete": true}
	for _, v := range []string{"in-review", "approved", "established", "obsolete"} {
		st, ok := m.def.State(v)
		if !ok {
			t.Fatalf("state %q missing from the definition", v)
		}
		if st.Final() != wantFinal[v] {
			t.Errorf("state %q final = %v, want %v", v, st.Final(), wantFinal[v])
		}
	}
	if _, ok := m.def.State("nope"); ok {
		t.Fatal("an undeclared value is a state")
	}
}

// A value whose only transition is a self-loop is final.
func TestCompile_SelfLoopOnlyIsFinal(t *testing.T) {
	meta := snapshotMeta()
	ct := meta.Types["snapshot-status"]
	ct.Transitions = append(ct.Transitions, metamodel.TransitionDef{From: "obsolete", To: "obsolete"})
	meta.Types["snapshot-status"] = ct
	m := mustCompile(t, meta).machines["snapshot-status"]
	if st, _ := m.def.State("obsolete"); !st.Final() {
		t.Fatal("a value with only a self-loop is not final")
	}
}

// Every compiled move selects at most one branch: the invariant that lets a
// refusal by the poster rule mean "guard denied".
func TestCompile_OneBranchPerMove(t *testing.T) {
	for _, meta := range []*metamodel.Metamodel{snapshotMeta(), relatedMeta(true)} {
		set, err := CompileTransitions(meta, WithTransitionTraversals(&stubBinder{}))
		if err != nil {
			t.Fatal(err)
		}
		for name, m := range set.machines {
			for from, bs := range m.exits {
				for _, b := range bs {
					if n := len(m.branches(from, b.to)); n != 1 {
						t.Errorf("%s: %s→%s selects %d branches", name, from, b.to, n)
					}
				}
			}
		}
	}
}

// A move handed more than one branch fails closed as undeclared rather than
// reading a guard refusal of one branch as the answer for the move.
func TestDecideMove_SeveralBranchesRefused(t *testing.T) {
	compiled := mustCompile(t, snapshotMeta()).machines["snapshot-status"]
	m := &HeadlessFlow{name: compiled.name, def: compiled.def, exits: maps.Clone(compiled.exits)}
	m.exits["in-review"] = append(slices.Clone(m.exits["in-review"]), headlessBranch{to: "approved"})
	res := m.decideMove(context.Background(), "status", "in-review", "approved",
		ent("S1", "snapshot", "approved"), inertGuard{}, nil, edgeTraversal{})
	if res.gate != gateUndeclared {
		t.Fatalf("gate = %v, want gateUndeclared", res.gate)
	}
}

// An unset current value does not read as the initial state: no move leaves
// it, not to the entry value and not to its successor (DEC-EJ00DNZ).
func TestEnforceUpdate_UnsetIsNotInitial(t *testing.T) {
	set := mustCompile(t, snapshotMeta())
	for _, to := range []string{"in-review", "approved", "established"} {
		err := set.EnforceUpdate(context.Background(), ent("S1", "snapshot", ""), ent("S1", "snapshot", to),
			inertGuard{}, nil)
		if !errors.Is(err, ErrIllegalTransition) {
			t.Fatalf("unset → %s: err = %v, want ErrIllegalTransition", to, err)
		}
	}
}

// A current value the machine does not declare is stranded: every move from
// it is illegal, and the error says why.
func TestEnforceUpdate_StrandedValue(t *testing.T) {
	set := mustCompile(t, snapshotMeta())
	err := set.EnforceUpdate(context.Background(), ent("S1", "snapshot", "legacy"),
		ent("S1", "snapshot", "approved"), inertGuard{}, nil)
	if !errors.Is(err, ErrIllegalTransition) || !strings.Contains(err.Error(), "not a declared state") {
		t.Fatalf("err = %v, want ErrIllegalTransition naming the undeclared state", err)
	}
	if got := set.Performable(context.Background(), ent("S1", "snapshot", "legacy"), "status",
		inertGuard{}, nil); got != nil {
		t.Fatalf("Performable from a stranded value = %v, want nil", got)
	}
}

// The guard runs before the precondition: a caller without the permission
// is refused 403 even when the precondition does not hold, and a nil guard
// refuses a guarded move.
func TestEnforceUpdate_GuardBeforePrecondition(t *testing.T) {
	set := mustCompile(t, snapshotMeta())
	noSignature := fakeLookup{counts: map[string]int{}}
	for name, g := range map[string]PermissionGuard{"unheld": fakeGuard{}, "nil": nil} {
		err := set.EnforceUpdate(context.Background(), ent("S1", "snapshot", "approved"),
			ent("S1", "snapshot", "established"), g, noSignature)
		var ge *GuardError
		if !errors.As(err, &ge) || ge.Permission != "establish" {
			t.Fatalf("%s guard: err = %v, want GuardError for establish", name, err)
		}
	}
}

// countingLookup counts the graph lookups a `when:` makes.
type countingLookup struct{ calls int }

func (l *countingLookup) OutgoingCounts(_ context.Context, _ string) map[string]int {
	l.calls++
	return map[string]int{}
}

// The precondition is not evaluated for a caller without the guard
// permission, so its outcome cannot reach that caller.
func TestEnforceUpdate_PreconditionSkippedWhenGuardUnheld(t *testing.T) {
	set := mustCompile(t, snapshotMeta())
	lookup := &countingLookup{}
	err := set.EnforceUpdate(context.Background(), ent("S1", "snapshot", "approved"),
		ent("S1", "snapshot", "established"), fakeGuard{}, lookup)
	if !errors.Is(err, ErrGuardDenied) {
		t.Fatalf("err = %v, want ErrGuardDenied", err)
	}
	if lookup.calls != 0 {
		t.Fatalf("the precondition ran %d graph lookups for an unauthorized caller", lookup.calls)
	}
	err = set.EnforceUpdate(context.Background(), ent("S1", "snapshot", "approved"),
		ent("S1", "snapshot", "established"), fakeGuard{perms: map[string]bool{"establish": true}}, lookup)
	if !errors.Is(err, ErrPreconditionFailed) || lookup.calls != 1 {
		t.Fatalf("authorized: err = %v, lookups = %d; want a precondition failure after one lookup", err,
			lookup.calls)
	}
}

// The rules set no Gate, so flow.Gated cannot occur; if it ever does, the
// move is refused, never allowed.
func TestOutcome_UnexpectedFailsClosed(t *testing.T) {
	res := outcome(flow.Decision[headlessBranch]{Outcome: flow.Gated}, "")
	if res.gate == gateNone || !errors.Is(res.err, errUnexpectedOutcome) {
		t.Fatalf("Gated outcome = %+v, want a refusal", res)
	}
	if reasonFor(gateUndeclared) == VerdictAllowed {
		t.Fatal("an undeclared move reads as allowed")
	}
}

// A traversal bind failure fails only the moves whose `when:` traverses: an
// untraversing out-edge of the same state stays performable.
func TestPerformable_BindErrorFailsOnlyTraversingEdges(t *testing.T) {
	meta := relatedMeta(true)
	ct := meta.Types["work-status"]
	ct.Values = append(ct.Values, "parked")
	ct.Transitions = append(ct.Transitions, metamodel.TransitionDef{From: "open", To: "parked", When: `entity.value == "parked"`})
	meta.Types["work-status"] = ct
	set, err := CompileTransitions(meta, WithTransitionTraversals(&stubBinder{err: errors.New("store down")}))
	if err != nil {
		t.Fatal(err)
	}
	verdicts := set.Performable(context.Background(), ent("T1", "ticket", "open"), "status", inertGuard{}, nil)
	got := map[string]TransitionVerdict{}
	for _, v := range verdicts {
		got[v.To] = v
	}
	if !got["parked"].Allowed {
		t.Errorf("parked = %+v, want allowed", got["parked"])
	}
	for _, to := range []string{"done", "dropped"} {
		if got[to].Allowed || got[to].Reason != VerdictPrecondition {
			t.Errorf("%s = %+v, want a precondition refusal", to, got[to])
		}
	}
}

// The drift guard: for every combination of guard, `when:` and bind
// failure, the read verdict of a move agrees with the error class the write
// returns for it.
func TestDecideMove_VerdictMatchesEnforcement(t *testing.T) {
	guards := map[string]struct {
		permission string
		holder     PermissionGuard
	}{
		"unguarded": {"", fakeGuard{}},
		"held":      {"p", fakeGuard{perms: map[string]bool{"p": true}}},
		"unheld":    {"p", fakeGuard{}},
		"nil":       {"p", nil},
	}
	whens := map[string]string{
		"none":          "",
		"true":          `entity.value == "done"`,
		"false":         `entity.value == "open"`,
		"related-true":  "related(entity, 'owned-by')",
		"related-false": "not related(entity, 'owned-by')",
	}
	for gName, g := range guards {
		for wName, when := range whens {
			for _, bindErr := range []bool{false, true} {
				name := fmt.Sprintf("%s/%s/binderr=%v", gName, wName, bindErr)
				t.Run(name, func(t *testing.T) {
					meta := relatedMeta(true)
					ct := meta.Types["work-status"]
					ct.Transitions = []metamodel.TransitionDef{
						{From: "open", To: "done", Guard: g.permission, When: when},
					}
					meta.Types["work-status"] = ct
					binder := &stubBinder{answer: map[string]bool{"T1": true}}
					if bindErr {
						binder.err = errors.New("store down")
					}
					set, err := CompileTransitions(meta, WithTransitionTraversals(binder))
					if err != nil {
						t.Fatal(err)
					}
					ctx := context.Background()
					verdicts := set.Performable(ctx, ent("T1", "ticket", "open"), "status", g.holder, nil)
					if len(verdicts) != 1 {
						t.Fatalf("verdicts = %v, want one", verdicts)
					}
					v := verdicts[0]
					werr := set.EnforceUpdate(ctx, ent("T1", "ticket", "open"), ent("T1", "ticket", "done"),
						g.holder, nil)
					switch {
					case v.Allowed:
						if werr != nil {
							t.Fatalf("verdict allows, write refuses: %v", werr)
						}
					case v.Reason == VerdictGuard:
						if !errors.Is(werr, ErrGuardDenied) {
							t.Fatalf("verdict guard, write err = %v", werr)
						}
					case v.Reason == VerdictPrecondition:
						if !errors.Is(werr, ErrPreconditionFailed) {
							t.Fatalf("verdict precondition, write err = %v", werr)
						}
					default:
						t.Fatalf("unexpected verdict %+v", v)
					}
					if (gName == "unheld" || gName == "nil") && v.Reason != VerdictGuard {
						t.Fatalf("reason = %q, want guard: the guard is checked first", v.Reason)
					}
					wantAllowed := (gName == "unguarded" || gName == "held") &&
						(slices.Contains([]string{"none", "true"}, wName) || (wName == "related-true" && !bindErr))
					if v.Allowed != wantAllowed {
						t.Fatalf("allowed = %v, want %v", v.Allowed, wantAllowed)
					}
				})
			}
		}
	}
}
