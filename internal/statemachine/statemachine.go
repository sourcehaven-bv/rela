// Package statemachine compiles the declarative enum state machines in a
// metamodel (CustomType.Transitions, see TKT-E4LW2) into an executable form
// and enforces them on the entity write path.
//
// # Two phases, cleanly split
//
// Compile runs ONCE at startup (from appbuild), turning the metamodel's
// declarative transition data into a [Set] of ready-to-run [Machine] values,
// each a headless flow on the engine in internal/flow (FEAT-J0O10QI): the
// enum values are the states of a [flow.Definition], the transitions are its
// branches, `when:` predicates are compiled, and load-time
// well-formedness is checked (dangling from/to/initial, bad predicate syntax)
// so a malformed machine fails fast at boot, never at write time. Nothing here
// consults the metamodel again after Compile.
//
// [Set.EnforceUpdate] runs on every entity write (from entitymanager, a required
// collaborator in the fixed write pipeline — as unforgettable as validation and
// audit). Given (old, new, principal) it finds every changed state-machine
// property, and for each:
//
//   - legality: the (from,to) move must be a declared edge, else [ErrIllegalTransition] (422);
//   - guard:    on a served path (a principal is present), the edge's guard
//     permission must be held for the subject, checked via the injected [Guard]
//     — else [ErrGuardDenied] (403);
//   - when:     the edge's compiled precondition must hold, else [ErrPreconditionFailed] (422).
//
// The move is decided by [flow.Decide], the decider realtime sessions and
// flowengine use: the guard is the poster rule (Rules.Admits), so it runs
// before the precondition (Rules.Holds), and a precondition that fails to
// evaluate stops the decision.
//
// # Why this shape
//
// The machines are built from the metamodel at boot and injected; the write
// path never re-derives them. This keeps `metamodel` a pure declarative source
// (it gains data, not behavior) and keeps `acl` a pure permission oracle (it
// answers "does principal hold permission P for subject S" and knows nothing
// about state machines). The enforcer owns old-vs-new and the transition
// semantics; it merely *asks* the ACL a permission question whose permission
// name comes from its own compiled edge.
package statemachine

import (
	"context"
	"errors"
	"fmt"

	"github.com/Sourcehaven-BV/rela/internal/flow"
	"github.com/Sourcehaven-BV/rela/internal/predicate"
)

// Sentinel errors classify an enforcement failure so entitymanager can map
// each to the right status (422 legality/precondition, 403 guard). They wrap
// with %w so callers use errors.Is.
var (
	// ErrIllegalTransition means the (from,to) move is not a declared edge.
	ErrIllegalTransition = errors.New("illegal transition")
	// ErrGuardDenied means the acting principal lacks the edge's guard
	// permission for the subject (served path only).
	ErrGuardDenied = errors.New("transition guard denied")
	// ErrPreconditionFailed means the edge's `when:` predicate evaluated false.
	ErrPreconditionFailed = errors.New("transition precondition failed")
	// ErrIllegalEntry means a create set a state-machine property to a value
	// other than the machine's entry value.
	ErrIllegalEntry = errors.New("illegal entry state")
)

// GuardError is returned (wrapping [ErrGuardDenied]) when an edge's guard
// permission is not held. It carries the Permission so a caller mapping the
// denial to an authorization response can name the specific right in a
// queryable field rather than parsing the message (RR-F30CZ/N1).
type GuardError struct {
	Prop       string
	From, To   string
	Permission string
}

func (e *GuardError) Error() string {
	return fmt.Sprintf("%s: %s %q→%q requires permission %q",
		ErrGuardDenied.Error(), e.Prop, e.From, e.To, e.Permission)
}

// Unwrap ties GuardError to the ErrGuardDenied sentinel so errors.Is keeps
// working.
func (e *GuardError) Unwrap() error { return ErrGuardDenied }

// Guard is the narrow permission oracle the enforcer needs to gate a
// transition. Defined at the consumer (CLAUDE.md "interfaces at the call
// site"); the wiring site adapts the real ACL. It answers exactly one question
// and knows nothing about state machines: does the principal carried on ctx
// hold `permission` for the entity identified by subjectID?
//
// The served-vs-inert decision belongs to the implementation: when there is no
// principal or policy to evaluate (a direct CLI write with no acl.yaml) the
// adapter returns true, so the guard is inert exactly where authorization is
// meaningless. The subject-aware answer (relation-conferred local roles, not
// just globals) is the ACL's responsibility.
type Guard interface {
	HoldsPermission(ctx context.Context, subjectID, permission string) bool
}

// GraphLookup is the narrow graph-query surface a `when:` predicate needs
// (has_relation / count_relations). Defined at the consumer; the wiring site
// supplies a snapshot-backed implementation. OutgoingCounts returns, for
// fromID, relation type → count of outgoing edges, so one scan answers both
// host functions (mirrors affordances.RelationLookup).
type GraphLookup interface {
	OutgoingCounts(ctx context.Context, fromID string) map[string]int
}

// TraversalBinder answers the `related(...)` traversals of `when:` programs for
// a batch of entities of one type. Defined at the consumer; the wiring site
// supplies a raw-store binder, because a transition is enforced on the write
// path where the store is the authority, and [Set.Performable] must agree with
// that enforcement (the drift guard) rather than with the viewer's reads.
type TraversalBinder interface {
	Bind(ctx context.Context, entityType string, ids []string, progs ...*predicate.Program) (
		func(rowID string) predicate.TraversalFunc, error)
}

// Option configures [Compile].
type Option func(*Set)

// WithTraversals wires the binder that answers `related(...)` in `when:`.
// Without it, a `when:` using related() is a compile problem.
func WithTraversals(b TraversalBinder) Option {
	return func(s *Set) { s.traversals = b }
}

// branch is one compiled transition out of a state of a headless flow: the
// target value, the (possibly empty) guard permission, the (possibly nil)
// compiled `when:` precondition, and the (possibly empty) display label of
// the move. The label is display-only and never consulted for enforcement.
type branch struct {
	to    string
	guard string
	when  *predicate.Program
	label string
}

// Machine is one enum state machine compiled to a headless flow
// (TKT-12RC7JT): a flow whose state is a property of the entity itself, so it
// has no `_flow-run` header, no run store entry, no timers and no outputs.
//
// def is the engine's [flow.Definition]: one state per enum value, in
// declaration order, with the entry value as its initial state. A value with
// no transition to another value is final. exits holds each state's branches
// in declaration order; the event of a headless flow is "set the property to
// a value", so a move from→to selects the branches of from that target to
// (see Machine.branches).
//
// A headless flow does not read an unset property as its initial state, as
// [flow.Read] does for an unset instance: an empty current value selects no
// branch, so the write is refused (see Machine.decideMove).
//
// Immutable after Compile. The `when:` programs are self-contained after
// compilation, so the compile-time env is not retained.
type Machine struct {
	name  string
	def   *flow.Definition
	exits map[string][]branch
}

// branches returns the branches the move from→to selects. Compile refuses a
// duplicate edge, so there is at most one.
func (m *Machine) branches(from, to string) []branch {
	var out []branch
	for _, b := range m.exits[from] {
		if b.to == to {
			out = append(out, b)
		}
	}
	return out
}

// eachBranch calls fn for every branch, ordered by from-state name and then
// declaration order, so load problems and lints are deterministic.
func (m *Machine) eachBranch(fn func(from string, b branch)) {
	for _, from := range sortedKeys(m.exits) {
		for _, b := range m.exits[from] {
			fn(from, b)
		}
	}
}

// Set is the compiled collection of machines keyed by the CustomType name they
// were built from, plus the property→type-name index needed to find which
// properties of a given entity type are state machines. Injected into
// entitymanager as a required collaborator.
type Set struct {
	machines map[string]*Machine // custom-type name → machine
	// propType maps entity type → property name → custom-type name, but only
	// for properties whose type is a state machine. Absent entries are not
	// machines (unconstrained, historical behavior).
	propType map[string]map[string]string
	// traversals answers `related(...)` in `when:`; nil when none is wired,
	// in which case Compile refuses any `when:` that uses it.
	traversals TraversalBinder
}

// Empty reports whether the set has no machines. A metamodel with no
// transitions compiles to an empty set; Enforce is then a no-op.
func (s *Set) Empty() bool { return s == nil || len(s.machines) == 0 }

// EntryValue returns the compiled entry value (Initial, else Default) for the
// state-machine property prop on entityType — the only value a create may set
// (BUG-X1C7S). Returns "" when prop is not a state machine on entityType or the
// Set is empty; a create form uses it to lock the field to its initial state.
func (s *Set) EntryValue(entityType, prop string) string {
	if s.Empty() {
		return ""
	}
	typeName, ok := s.propType[entityType][prop]
	if !ok {
		return ""
	}
	return s.machines[typeName].def.Initial()
}

// EmptySet returns a machine-less [Set] whose Enforce methods are no-ops. It is
// the explicit "no state machines" value for wiring sites and tests that need a
// non-nil enforcer without a metamodel (the Manager requires a non-nil
// enforcer so the write pipeline can't silently skip it; an empty set is the
// legitimate opt-out).
func EmptySet() *Set {
	return &Set{machines: map[string]*Machine{}, propType: map[string]map[string]string{}}
}
