package affordances_test

import (
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/affordances"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/statemachine"
)

// transitionMeta is a ticket type whose `status` is a NAMED state-machine type
// (transitions live on the CustomType). open→review is guarded by `review-it`;
// review→done is guarded by `close-it` and gated on a `signed-off` relation.
func transitionMeta() *metamodel.Metamodel {
	return &metamodel.Metamodel{
		Types: map[string]metamodel.CustomType{
			"ticket-status": {
				Values:  []string{"open", "review", "done"},
				Initial: "open",
				Transitions: []metamodel.TransitionDef{
					{From: "open", To: "review", Guard: "review-it"},
					{From: "review", To: "done", Guard: "close-it", When: `count_relations(entity, "signed-off") > 0`},
					{From: "review", To: "open", Guard: "review-it"},
				},
			},
		},
		Entities: map[string]metamodel.EntityDef{
			"ticket": {Properties: map[string]metamodel.PropertyDef{
				"title":  {Type: metamodel.PropertyTypeString},
				"status": {Type: "ticket-status"},
			}},
		},
	}
}

// newTransitionResolver builds a resolver over transitionMeta with a policy
// granting `alice` the given permissions, and the given graph edges.
func newTransitionResolver(t *testing.T, perms string, edges ...[3]string) *affordances.PolicyResolver {
	t.Helper()
	meta := transitionMeta()
	p := policyFromYAML(t, `
roles:
  editor:
    permissions: [`+perms+`]
assignments:
  alice: editor
`)
	r, err := affordances.New(meta, newStubLookup(edges...), declFor(t, p))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	machines, err := statemachine.Compile(meta)
	if err != nil {
		t.Fatalf("Compile: %v", err)
	}
	return r.WithMachines(machines)
}

func verdictByTo(vs []statemachine.TransitionVerdict, to string) (statemachine.TransitionVerdict, bool) {
	for _, v := range vs {
		if v.To == to {
			return v, true
		}
	}
	return statemachine.TransitionVerdict{}, false
}

func TestTransitionVerdicts_GuardHeld(t *testing.T) {
	r := newTransitionResolver(t, "review-it")
	e := ticket("T-1", map[string]any{"status": "open"})

	got := r.TransitionVerdicts(ctxAs("alice"), e)
	vs := got["status"]
	if len(vs) != 1 {
		t.Fatalf("open has one out-edge (→review), got %+v", vs)
	}
	v := vs[0]
	if v.To != "review" || !v.Allowed || v.Reason != statemachine.VerdictAllowed {
		t.Errorf("open→review with review-it held should be allowed, got %+v", v)
	}
}

func TestTransitionVerdicts_GuardDenied(t *testing.T) {
	r := newTransitionResolver(t, "" /* no perms */)
	e := ticket("T-1", map[string]any{"status": "open"})

	v := r.TransitionVerdicts(ctxAs("alice"), e)["status"][0]
	if v.Allowed || v.Reason != statemachine.VerdictGuard {
		t.Errorf("open→review without review-it should be guard-denied, got %+v", v)
	}
	// The guard name is surfaced for the UI to explain the disabled option.
	if v.Guard != "review-it" {
		t.Errorf("verdict should carry guard name, got %q", v.Guard)
	}
}

func TestTransitionVerdicts_PreconditionGate(t *testing.T) {
	// alice holds both guards; review→done additionally needs a signed-off edge.
	t.Run("precondition unmet", func(t *testing.T) {
		r := newTransitionResolver(t, "review-it, close-it") // no signed-off edge
		e := ticket("T-1", map[string]any{"status": "review"})
		done, ok := verdictByTo(r.TransitionVerdicts(ctxAs("alice"), e)["status"], "done")
		if !ok {
			t.Fatal("expected a →done verdict")
		}
		if done.Allowed || done.Reason != statemachine.VerdictPrecondition {
			t.Errorf("review→done without signed-off should be precondition-blocked, got %+v", done)
		}
	})
	t.Run("precondition met", func(t *testing.T) {
		r := newTransitionResolver(t, "review-it, close-it", [3]string{"T-1", "signed-off", "PERS-x"})
		e := ticket("T-1", map[string]any{"status": "review"})
		done, _ := verdictByTo(r.TransitionVerdicts(ctxAs("alice"), e)["status"], "done")
		if !done.Allowed {
			t.Errorf("review→done with signed-off + close-it should be allowed, got %+v", done)
		}
	})
}

func TestTransitionVerdicts_NoMachinesWired(t *testing.T) {
	// A resolver without WithMachines returns an empty map.
	meta := transitionMeta()
	p := policyFromYAML(t, "roles: {}\n")
	r, err := affordances.New(meta, newStubLookup(), declFor(t, p))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if got := r.TransitionVerdicts(ctxAs("alice"), ticket("T-1", map[string]any{"status": "open"})); len(got) != 0 {
		t.Errorf("no machines wired should give empty map, got %+v", got)
	}
}

func TestTransitionVerdicts_NonMachineType(t *testing.T) {
	r := newTransitionResolver(t, "review-it")
	// feature has no status machine (not even in transitionMeta) → empty.
	if got := r.TransitionVerdicts(ctxAs("alice"), ticket("T-1", map[string]any{"status": "done"})); len(got) != 0 {
		// "done" is terminal → no out-edges → empty map.
		t.Errorf("terminal state should give empty map, got %+v", got)
	}
}
