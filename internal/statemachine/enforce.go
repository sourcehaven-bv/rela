package statemachine

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/flow"
	"github.com/Sourcehaven-BV/rela/internal/predicate"
)

// EnforceUpdate checks every state-machine property that changed between old and
// new, in property-name order for deterministic errors. For each changed
// machine property it applies, in order: legality (declared edge), guard, and
// the `when:` precondition. The first failure returns a wrapped sentinel; a nil
// return means every changed machine property made a legal, authorized,
// precondition-satisfying move (or nothing machine-typed changed).
//
// The guard's served/inert behavior is the guard's own concern: [Guard]
// resolves the acting principal from ctx and returns true (allow) when there is
// no principal/policy to evaluate — so on a direct CLI write with no policy the
// guard is inert while legality and preconditions still apply. Passing a nil
// guard makes every guarded edge fail closed.
//
// old and updated are the same entity's prior and post-write state; updated
// must be non-nil. Use [Set.EnforceCreate] for creates (no prior state).
func (s *Set) EnforceUpdate(ctx context.Context, old, updated *entity.Entity, guard Guard, lookup GraphLookup) error {
	if s.Empty() || updated == nil {
		return nil
	}
	props := s.propType[updated.Type]
	if len(props) == 0 {
		return nil
	}

	type move struct {
		prop, from, to string
		m              *Machine
	}
	var moves []move
	var whens []*predicate.Program
	for _, prop := range sortedKeys(props) {
		m := s.machines[props[prop]]
		from := ""
		if old != nil {
			from = old.GetString(prop)
		}
		to := updated.GetString(prop)
		if from == to {
			continue // property did not change
		}
		moves = append(moves, move{prop: prop, from: from, to: to, m: m})
		for _, b := range m.branches(from, to) {
			whens = append(whens, b.when)
		}
	}
	// Every changed property's `related(...)` is answered in one bind. A bind
	// failure is not an error here: it fails the first edge that needs it, as
	// a precondition error, like any other `when:` evaluation error.
	traversal := s.bindTraversals(ctx, updated, whens)
	for _, mv := range moves {
		if err := applyMove(ctx, mv.m, mv.prop, mv.from, mv.to, updated, guard, lookup, traversal); err != nil {
			return err
		}
	}
	return nil
}

// edgeTraversal carries the answer to an entity's `related(...)` into
// Machine.decideMove, or the reason there is none.
type edgeTraversal struct {
	fn  predicate.TraversalFunc
	err error
}

// bindTraversals answers every `related(...)` in whens for the one entity e.
// The result has neither a func nor an error when none of whens traverses.
//
// A row on a named face is refused: the store answers a traversal from the
// default face's edges, so the answer would describe another row. The
// refusal fails the precondition, which is the closed direction.
func (s *Set) bindTraversals(
	ctx context.Context, e *entity.Entity, whens []*predicate.Program,
) edgeTraversal {
	traverses := false
	for _, w := range whens {
		if w != nil && len(w.Traversals()) > 0 {
			traverses = true
			break
		}
	}
	switch {
	case !traverses:
		return edgeTraversal{}
	case s.traversals == nil:
		// coverage-ignore: invariant: Compile refuses a traversing `when:` without a binder
		return edgeTraversal{err: fmt.Errorf("%s(...) is not available: no store is wired to answer it",
			predicate.FuncRelated)}
	case e.Face != "":
		return edgeTraversal{err: fmt.Errorf("%s(...) cannot be answered on face %q", predicate.FuncRelated, e.Face)}
	}
	bound, err := s.traversals.Bind(ctx, e.Type, []string{e.ID}, whens...)
	if err != nil {
		return edgeTraversal{err: err}
	}
	return edgeTraversal{fn: bound(e.ID)}
}

// EnforceCreate checks the entry value of every state-machine property on a
// newly created entity. A create has no prior state, so there is no edge to
// traverse; the rule is that a machine property must enter at its entry value
// (Initial, else Default). Compile guarantees every machine HAS an entry value
// (BUG-X1C7S), so a create can never deviate from the initial state — a
// non-entry value is rejected with [ErrIllegalEntry] (422). Guards do not apply
// on create: create is entry, not a transition, and the operator's `initial`
// declares the (trusted) entry point.
func (s *Set) EnforceCreate(_ context.Context, e *entity.Entity) error {
	if s.Empty() || e == nil {
		return nil
	}
	props := s.propType[e.Type]
	for _, prop := range sortedKeys(props) {
		m := s.machines[props[prop]]
		entry := m.def.Initial()
		if entry == "" {
			// Unreachable for a compiled Set (Compile requires an entry value on
			// any machine with transitions). Kept as a defensive guard against a
			// hand-built Machine; a create then imposes no constraint.
			continue
		}
		got := e.GetString(prop)
		if got == "" {
			continue // absent → the default applies elsewhere; not an illegal entry
		}
		if got != entry {
			return fmt.Errorf("%w: %s=%q on create; must enter at %q",
				ErrIllegalEntry, prop, got, entry)
		}
	}
	return nil
}

// applyMove decides one changed machine property and maps a refused move
// to the wire-facing error. It shares Machine.decideMove with the
// read-side [Set.Performable], so enforcement and the "what can I do"
// affordance can never disagree about whether a transition is allowed (the
// drift guard).
func applyMove(
	ctx context.Context, m *Machine, prop, from, to string, e *entity.Entity, guard Guard, lookup GraphLookup,
	traversal edgeTraversal,
) error {
	res := m.decideMove(ctx, prop, from, to, e, guard, lookup, traversal)
	switch res.gate {
	case gateNone:
		return nil
	case gateUndeclared:
		if res.stranded {
			return fmt.Errorf("%w: %s %q→%q is not a declared transition (%q is not a declared state)",
				ErrIllegalTransition, prop, from, to, from)
		}
		return fmt.Errorf("%w: %s %q→%q is not a declared transition", ErrIllegalTransition, prop, from, to)
	case gateGuard:
		return &GuardError{Prop: prop, From: from, To: to, Permission: res.permission}
	default: // gatePrecondition
		if res.err != nil {
			return fmt.Errorf("%w: %s %q→%q when: %s", ErrPreconditionFailed, prop, from, to, res.err.Error())
		}
		return fmt.Errorf("%w: %s %q→%q precondition not met", ErrPreconditionFailed, prop, from, to)
	}
}

// gate identifies which check refused a move (or none).
type gate int

const (
	gateNone gate = iota
	// gateUndeclared: the move is not a declared transition.
	gateUndeclared
	gateGuard
	gatePrecondition
)

// moveResult is the outcome of deciding one move.
type moveResult struct {
	gate gate
	// stranded: the current value is not a state of the machine
	// (gateUndeclared only).
	stranded bool
	// permission is the guard the principal lacks (gateGuard only).
	permission string
	// err is set when a `when:` failed to evaluate (gatePrecondition only).
	err error
}

// errUnexpectedOutcome refuses a [flow.Decide] outcome the rules cannot
// produce, so a later change to the rules fails closed.
var errUnexpectedOutcome = errors.New("unexpected decision outcome")

// decideMove decides the move from→to of prop on e, the entity as it is
// after the write, through [flow.Decide]. It is the one evaluation path of a
// transition, shared by [Set.EnforceUpdate] and [Set.Performable].
//
// The guard permission is the poster rule (Rules.Admits) and the `when:`
// precondition is the guard (Rules.Holds), so the permission is checked
// first: a caller without it is refused whatever the precondition says, and
// the precondition is not evaluated for it. A nil guard refuses every
// guarded branch. A `when:` that fails to evaluate, or needs a traversal
// whose bind failed, stops the decision (fail closed).
//
// The engine's words name the other check, so the mapping is:
//
//	flow (engine)             statemachine (this package)
//	Rules.Admits refuses      gateGuard (the ACL permission)
//	Rules.Holds false         gatePrecondition (`when:` not met)
//	GuardFailed               gatePrecondition with the `when:` error
//	Moved                     gateNone (allowed)
//
// Invariant: a move selects at most one branch, because Compile refuses a
// duplicate (from, to). That is what lets a refusal by the poster rule, which
// the engine treats as "skip this branch", mean "guard denied" here. A move
// that selects more than one branch is refused as undeclared.
//
// An empty from is refused as undeclared: a headless flow does not read an
// unset property as its initial state (see [Machine]).
func (m *Machine) decideMove(
	ctx context.Context, prop, from, to string, e *entity.Entity, guard Guard, lookup GraphLookup,
	traversal edgeTraversal,
) moveResult {
	if from == "" {
		return moveResult{gate: gateUndeclared}
	}
	cur := flow.Read(m.def, flow.Instance{State: from}, time.Time{})
	if cur.Stranded {
		return moveResult{gate: gateUndeclared, stranded: true}
	}
	branches := m.branches(cur.State, to)
	if len(branches) != 1 {
		return moveResult{gate: gateUndeclared}
	}
	var refused string
	d := flow.Decide(cur.State, branches, flow.Rules[branch]{
		Admits: func(b branch) bool {
			if b.guard == "" {
				return true
			}
			if guard == nil || !guard.HoldsPermission(ctx, e.ID, b.guard) {
				refused = b.guard
				return false
			}
			return true
		},
		Holds: func(b branch) (bool, error) {
			if b.when == nil {
				return true, nil
			}
			if traversal.err != nil && len(b.when.Traversals()) > 0 {
				return false, traversal.err
			}
			return evalWhen(ctx, b.when, e, prop, lookup, traversal.fn)
		},
		Target: func(b branch) string { return b.to },
	})
	return outcome(d, refused)
}

// outcome maps a decision to a move result. refused is the guard permission
// the poster rule refused, if any. Only a move is allowed; every other
// outcome, including one the rules cannot produce, refuses.
func outcome(d flow.Decision[branch], refused string) moveResult {
	switch d.Outcome {
	case flow.Moved:
		return moveResult{gate: gateNone}
	case flow.NoMove:
		if refused != "" {
			return moveResult{gate: gateGuard, permission: refused}
		}
		return moveResult{gate: gatePrecondition}
	case flow.GuardFailed:
		return moveResult{gate: gatePrecondition, err: d.Err}
	default: // flow.Gated: the rules set no Gate.
		return moveResult{gate: gatePrecondition, err: errUnexpectedOutcome}
	}
}
