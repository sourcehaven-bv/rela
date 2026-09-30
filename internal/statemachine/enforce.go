package statemachine

import (
	"context"
	"fmt"

	"github.com/Sourcehaven-BV/rela/internal/entity"
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
		if ed, ok := m.edgeFor(from, to); ok {
			whens = append(whens, ed.when)
		}
	}
	// Every changed property's `related(...)` is answered in one bind. A bind
	// failure is not an error here: it fails the first edge that needs it, as
	// a precondition error, like any other `when:` evaluation error.
	traversal := s.bindTraversals(ctx, updated, whens)
	for _, mv := range moves {
		if err := s.applyEdge(ctx, mv.m, mv.prop, mv.from, mv.to, updated, guard, lookup, traversal); err != nil {
			return err
		}
	}
	return nil
}

// edgeTraversal carries the answer to an entity's `related(...)` into
// [evalEdge], or the reason there is none.
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

// EnforceRestore checks every state-machine property of a history restore:
// an entity brought back at a recorded value rather than created (BUG-KK1UXH).
// The entry rule does not apply, since the record existed. Instead each value
// must be one the acting principal could have reached by transitions: some
// path of declared edges from the entry value to it, every edge on the path
// unguarded or guarded by a permission the principal holds. Otherwise
// delete-then-restore would reach a state a guard keeps the principal out of.
//
// A value no path of declared edges reaches is refused with [ErrIllegalEntry]
// (422); that covers a value the schema no longer declares, which no
// transition could ever leave again. A value reachable only through a guard
// the principal lacks yields a [GuardError] (403) naming the first such edge
// in (from, to) order.
//
// What it does not check, by design:
//
//   - `when:` preconditions. They judge a move from a prior state against the
//     current graph, and a restore has neither; the cascade delete has also
//     removed the relations a precondition would typically count.
//   - Legality of the move from the state the entity had when it was
//     deleted. A restore may bring back an earlier version, which is a move
//     no declared edge makes. The trust boundary for that is the right to
//     delete the entity and read its deleted history.
//
// The guard's served-vs-inert behavior is the [Guard]'s own, as in
// [Set.EnforceUpdate]; a nil guard makes every guarded edge fail closed. The
// guard is asked about an entity that is currently deleted, so a grant that
// came from the entity's own relations is gone; that fails toward refusal.
func (s *Set) EnforceRestore(ctx context.Context, e *entity.Entity, guard Guard) error {
	if s.Empty() || e == nil {
		return nil
	}
	held := map[string]bool{}
	holds := func(ed edge) bool {
		if ed.guard == "" {
			return true
		}
		v, ok := held[ed.guard]
		if !ok {
			v = guard != nil && guard.HoldsPermission(ctx, e.ID, ed.guard)
			held[ed.guard] = v
		}
		return v
	}
	props := s.propType[e.Type]
	for _, prop := range sortedKeys(props) {
		m := s.machines[props[prop]]
		got := e.GetString(prop)
		if got == "" || got == m.entry || m.entry == "" {
			// m.entry == "" is unreachable for a compiled Set, as in EnforceCreate.
			continue
		}
		if _, ok := m.reach(func(edge) bool { return true })[got]; !ok {
			return fmt.Errorf("%w: %s=%q on restore; no declared transition path from %q reaches it",
				ErrIllegalEntry, prop, got, m.entry)
		}
		permitted := m.reach(holds)
		if _, ok := permitted[got]; ok {
			continue
		}
		// got is reachable, but not through held guards alone, so some edge
		// leaving the permitted region carries an unheld guard.
		for _, k := range m.sortedKeys() {
			if _, in := permitted[k.from]; in && !holds(m.edges[k]) {
				return &GuardError{Prop: prop, From: k.from, To: k.to, Permission: m.edges[k].guard}
			}
		}
	}
	return nil
}

// reach returns the states reachable from the entry value, itself included,
// over the edges follow admits.
func (m *Machine) reach(follow func(edge) bool) map[string]struct{} {
	seen := map[string]struct{}{m.entry: {}}
	keys := m.sortedKeys()
	for grew := true; grew; {
		grew = false
		for _, k := range keys {
			_, fromSeen := seen[k.from]
			_, toSeen := seen[k.to]
			if fromSeen && !toSeen && follow(m.edges[k]) {
				seen[k.to] = struct{}{}
				grew = true
			}
		}
	}
	return seen
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
		if m.entry == "" {
			// Unreachable for a compiled Set (Compile requires an entry value on
			// any machine with transitions). Kept as a defensive guard against a
			// hand-built Machine; a create then imposes no constraint.
			continue
		}
		got := e.GetString(prop)
		if got == "" {
			continue // absent → the default applies elsewhere; not an illegal entry
		}
		if got != m.entry {
			return fmt.Errorf("%w: %s=%q on create; must enter at %q",
				ErrIllegalEntry, prop, got, m.entry)
		}
	}
	return nil
}

// applyEdge runs the three checks for one changed machine property, mapping a
// failing gate to the wire-facing error. Legality (undeclared edge) and the
// gate evaluation share one code path — [evalEdge] — with the read-side
// [Set.Performable], so enforcement and the "what can I do" affordance can
// never disagree about whether a transition is allowed (the drift guard).
func (s *Set) applyEdge(
	ctx context.Context, m *Machine, prop, from, to string, e *entity.Entity, guard Guard, lookup GraphLookup,
	traversal edgeTraversal,
) error {
	ed, ok := m.edgeFor(from, to)
	if !ok {
		return fmt.Errorf("%w: %s %q→%q is not a declared transition", ErrIllegalTransition, prop, from, to)
	}
	switch res := evalEdge(ctx, ed, prop, e, guard, lookup, traversal); res.gate {
	case gateNone:
		return nil
	case gateGuard:
		return &GuardError{Prop: prop, From: from, To: to, Permission: ed.guard}
	default: // gatePrecondition
		if res.err != nil {
			return fmt.Errorf("%w: %s %q→%q when: %s", ErrPreconditionFailed, prop, from, to, res.err.Error())
		}
		return fmt.Errorf("%w: %s %q→%q precondition not met", ErrPreconditionFailed, prop, from, to)
	}
}

// gate identifies which check on an edge failed (or none).
type gate int

const (
	gateNone gate = iota
	gateGuard
	gatePrecondition
)

// edgeResult is the outcome of evaluating one edge's gates.
type edgeResult struct {
	gate gate
	err  error // non-nil only when a `when:` predicate errored (gatePrecondition)
}

// evalEdge evaluates an edge's guard then precondition for (principal-on-ctx,
// entity e), in the same order and with the same semantics the write path
// enforces. It is the single source of truth shared by Set.applyEdge (write,
// maps to errors) and [Set.Performable] (read, maps to verdicts) so the two can
// never drift. A guard is checked first (an unheld guard short-circuits without
// evaluating the precondition, matching enforcement). A nil guard on a guarded
// edge fails closed. The guard's served-vs-inert decision lives in the Guard
// implementation.
func evalEdge(
	ctx context.Context, ed edge, prop string, e *entity.Entity, guard Guard, lookup GraphLookup,
	traversal edgeTraversal,
) edgeResult {
	if ed.guard != "" {
		if guard == nil || !guard.HoldsPermission(ctx, e.ID, ed.guard) {
			return edgeResult{gate: gateGuard}
		}
	}
	if ed.when != nil {
		if traversal.err != nil && len(ed.when.Traversals()) > 0 {
			return edgeResult{gate: gatePrecondition, err: traversal.err}
		}
		ok, err := evalWhen(ctx, ed.when, e, prop, lookup, traversal.fn)
		if err != nil {
			return edgeResult{gate: gatePrecondition, err: err}
		}
		if !ok {
			return edgeResult{gate: gatePrecondition}
		}
	}
	return edgeResult{gate: gateNone}
}
