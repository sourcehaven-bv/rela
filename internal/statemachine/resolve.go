package statemachine

import (
	"context"
	"sort"

	"github.com/Sourcehaven-BV/rela/internal/entity"
)

// TransitionVerdict is the resolved, read-side view of one outgoing transition
// for a specific (principal, entity): the target value, the guard permission it
// requires (empty when unguarded), whether the principal can perform it right
// now, and — when they can't — which gate blocked it. It carries no compiled
// predicate; it is the answer to "what can this principal do to this field on
// this entity", suitable for a UI status control or a CLI.
type TransitionVerdict struct {
	To      string
	Guard   string      // ACL permission name; "" when the edge is unguarded
	Allowed bool        // true iff the guard is held AND the precondition holds
	Reason  VerdictGate // why Allowed is false; VerdictAllowed when it's true
}

// VerdictGate names the gate that made a transition non-performable.
type VerdictGate string

const (
	// VerdictAllowed means the transition is performable now.
	VerdictAllowed VerdictGate = ""
	// VerdictGuard means the principal lacks the edge's guard permission.
	VerdictGuard VerdictGate = "guard"
	// VerdictPrecondition means the edge's `when:` precondition is not met
	// (or errored) against the entity's current graph state.
	VerdictPrecondition VerdictGate = "precondition"
)

// Performable resolves the outgoing transitions from the current value of prop
// on e, for the principal carried on ctx, returning a [TransitionVerdict] per
// declared out-edge. Each verdict reflects BOTH the guard (evaluated
// subject-aware via guard) and the `when:` precondition (evaluated against the
// graph via lookup) — the same two gates, in the same order, that
// [Set.EnforceUpdate] enforces on a write (they share [evalEdge]). Verdicts are
// sorted by To for a stable UI order.
//
// Returns nil when prop is not a state machine on e's type, when e sits in a
// terminal state (no declared out-edges from its current value), or on an
// empty/nil Set.
//
// This is a BOUNDED read: it evaluates only the out-edges of ONE field on ONE
// entity (O(out-edges)), not a per-row scan across a result set. That is why
// evaluating the `when:` predicate here is consistent with the
// no-predicate-on-reads rule, whose concern is unbounded/hot list paths — see
// internal/entitymanager/CLAUDE.md.
func (s *Set) Performable(
	ctx context.Context, e *entity.Entity, prop string, guard Guard, lookup GraphLookup,
) []TransitionVerdict {
	if s.Empty() || e == nil {
		return nil
	}
	typeName, ok := s.propType[e.Type][prop]
	if !ok {
		return nil // prop is not a state machine on this entity type
	}
	m := s.machines[typeName]
	from := e.GetString(prop)

	var out []TransitionVerdict
	for key, ed := range m.edges {
		if key.from != from {
			continue
		}
		res := evalEdge(ctx, ed, prop, e, guard, lookup)
		out = append(out, TransitionVerdict{
			To:      key.to,
			Guard:   ed.guard,
			Allowed: res.gate == gateNone,
			Reason:  reasonFor(res.gate),
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].To < out[j].To })
	return out
}

// MachineProps returns the property names of entityType whose type is a state
// machine, sorted. Empty when the type has no machine-typed property (or on a
// nil/empty Set). Lets a consumer enumerate which fields of an entity have a
// lifecycle without knowing the metamodel.
func (s *Set) MachineProps(entityType string) []string {
	if s.Empty() {
		return nil
	}
	props := s.propType[entityType]
	if len(props) == 0 {
		return nil
	}
	out := make([]string, 0, len(props))
	for name := range props {
		out = append(out, name)
	}
	sort.Strings(out)
	return out
}

func reasonFor(g gate) VerdictGate {
	switch g {
	case gateGuard:
		return VerdictGuard
	case gatePrecondition:
		return VerdictPrecondition
	default:
		return VerdictAllowed
	}
}
