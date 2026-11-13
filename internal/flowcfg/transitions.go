package flowcfg

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
type PermissionGuard interface {
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
// path where the store is the authority, and [Transitions.Performable] must agree with
// that enforcement (the drift guard) rather than with the viewer's reads.
type TraversalBinder interface {
	Bind(ctx context.Context, entityType string, ids []string, progs ...*predicate.Program) (
		func(rowID string) predicate.TraversalFunc, error)
}

// TransitionOption configures [CompileTransitions].
type TransitionOption func(*Transitions)

// WithTransitionTraversals wires the binder that answers `related(...)` in `when:`.
// Without it, a `when:` using related() is a compile problem.
func WithTransitionTraversals(b TraversalBinder) TransitionOption {
	return func(s *Transitions) { s.traversals = b }
}

// headlessBranch is one compiled transition out of a state of a headless flow: the
// target value, the (possibly empty) guard permission, the (possibly nil)
// compiled `when:` precondition, and the (possibly empty) display label of
// the move. The label is display-only and never consulted for enforcement.
type headlessBranch struct {
	to    string
	guard string
	when  *predicate.Program
	// whenSource is the `when:` as written, for display.
	whenSource string
	label      string
}

// HeadlessFlow is one enum state machine compiled to a headless flow
// (TKT-12RC7JT): a flow whose state is a property of the entity itself, so it
// has no `_flow-run` header, no run store entry, no timers and no outputs.
//
// def is the engine's [flow.Definition]: one state per enum value, in
// declaration order, with the entry value as its initial state. A value with
// no transition to another value is final. exits holds each state's branches
// in declaration order; the event of a headless flow is "set the property to
// a value", so a move from→to selects the branches of from that target to
// (see HeadlessFlow.branches).
//
// A headless flow does not read an unset property as its initial state, as
// [flow.Read] does for an unset instance: an empty current value selects no
// branch, so the write is refused (see HeadlessFlow.decideMove).
//
// Immutable after CompileTransitions. The `when:` programs are self-contained after
// compilation, so the compile-time env is not retained.
type HeadlessFlow struct {
	name string
	// states are the enum values in declaration order.
	states []string
	def    *flow.Definition
	exits  map[string][]headlessBranch
}

// branches returns the branches the move from→to selects. CompileTransitions refuses a
// duplicate edge, so there is at most one.
func (m *HeadlessFlow) branches(from, to string) []headlessBranch {
	var out []headlessBranch
	for _, b := range m.exits[from] {
		if b.to == to {
			out = append(out, b)
		}
	}
	return out
}

// eachBranch calls fn for every branch, ordered by from-state name and then
// declaration order, so load problems and lints are deterministic.
func (m *HeadlessFlow) eachBranch(fn func(from string, b headlessBranch)) {
	for _, from := range sortedKeys(m.exits) {
		for _, b := range m.exits[from] {
			fn(from, b)
		}
	}
}

// Transitions is the compiled collection of machines keyed by the CustomType name they
// were built from, plus the property→type-name index needed to find which
// properties of a given entity type are state machines. Injected into
// entitymanager as a required collaborator.
type Transitions struct {
	machines map[string]*HeadlessFlow // custom-type name → machine
	// propType maps entity type → property name → custom-type name, but only
	// for properties whose type is a state machine. Absent entries are not
	// machines (unconstrained, historical behavior).
	propType map[string]map[string]string
	// traversals answers `related(...)` in `when:`; nil when none is wired,
	// in which case CompileTransitions refuses any `when:` that uses it.
	traversals TraversalBinder
}

// Empty reports whether the set has no machines. A metamodel with no
// transitions compiles to an empty set; Enforce is then a no-op.
func (s *Transitions) Empty() bool { return s == nil || len(s.machines) == 0 }

// EntryValue returns the compiled entry value (Initial, else Default) for the
// state-machine property prop on entityType — the only value a create may set
// (BUG-X1C7S). Returns "" when prop is not a state machine on entityType or the
// Transitions is empty; a create form uses it to lock the field to its initial state.
func (s *Transitions) EntryValue(entityType, prop string) string {
	if s.Empty() {
		return ""
	}
	typeName, ok := s.propType[entityType][prop]
	if !ok {
		return ""
	}
	return s.machines[typeName].def.Initial()
}

// NoTransitions returns a machine-less [Transitions] whose Enforce methods are no-ops. It is
// the explicit "no state machines" value for wiring sites and tests that need a
// non-nil enforcer without a metamodel (the Manager requires a non-nil
// enforcer so the write pipeline can't silently skip it; an empty set is the
// legitimate opt-out).
func NoTransitions() *Transitions {
	return &Transitions{machines: map[string]*HeadlessFlow{}, propType: map[string]map[string]string{}}
}
