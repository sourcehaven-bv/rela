package statemachine

import (
	"context"
	"fmt"

	"github.com/Sourcehaven-BV/rela/internal/entity"
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
		if err := s.applyEdge(ctx, m, prop, from, to, updated, guard, lookup); err != nil {
			return err
		}
	}
	return nil
}

// EnforceCreate checks the entry value of every state-machine property on a
// newly created entity. A create has no prior state, so there is no edge to
// traverse; the only rule is that a machine property must enter at its entry
// value (Initial, else Default). A machine with no entry value (neither set)
// imposes no constraint. Guards do not apply on create-entry in this first cut
// (the entry itself is not a guarded edge — see the ticket's deferred
// alternative).
func (s *Set) EnforceCreate(_ context.Context, e *entity.Entity) error {
	if s.Empty() || e == nil {
		return nil
	}
	props := s.propType[e.Type]
	for _, prop := range sortedKeys(props) {
		m := s.machines[props[prop]]
		if m.entry == "" {
			continue // unconstrained entry
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

// applyEdge runs the three checks for one changed machine property.
func (s *Set) applyEdge(
	ctx context.Context, m *Machine, prop, from, to string, e *entity.Entity, guard Guard, lookup GraphLookup,
) error {
	ed, ok := m.edgeFor(from, to)
	if !ok {
		return fmt.Errorf("%w: %s %q→%q is not a declared transition", ErrIllegalTransition, prop, from, to)
	}

	// Guard: the guard decides served-vs-inert (it resolves the principal from
	// ctx and allows when there is nothing to evaluate). A nil guard on a
	// guarded edge fails closed.
	if ed.guard != "" {
		if guard == nil || !guard.HoldsPermission(ctx, e.ID, ed.guard) {
			return &GuardError{Prop: prop, From: from, To: to, Permission: ed.guard}
		}
	}

	// Precondition.
	if ed.when != nil {
		ok, err := evalWhen(ctx, ed.when, e, prop, lookup)
		if err != nil {
			return fmt.Errorf("%w: %s %q→%q when: %s", ErrPreconditionFailed, prop, from, to, err.Error())
		}
		if !ok {
			return fmt.Errorf("%w: %s %q→%q precondition not met", ErrPreconditionFailed, prop, from, to)
		}
	}
	return nil
}
