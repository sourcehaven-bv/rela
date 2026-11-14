// Package transitions compiles the `transitions:` of the enum types in
// schema.yaml into headless flows on the engine in internal/flow
// (FEAT-J0O10QI, TKT-12RC7JT, TKT-ZRQSGCM), and enforces them on the entity
// write path. A headless flow keeps its state in a property of the entity
// itself, so unlike a flows.yaml flow (package flowcfg) it has no run.
//
// [CompileTransitions] runs once at startup (from appbuild). It turns the
// metamodel's declarative transition data into [Transitions], a set of
// [HeadlessFlow] values: the enum values are the states of a
// [flow.Definition], the transitions are its branches, and `when:`
// predicates are compiled. Load-time well-formedness is checked (dangling
// from/to/initial, bad predicate syntax), so a malformed machine fails at
// boot, never at write time. Nothing here consults the metamodel again
// after CompileTransitions.
//
// [Transitions.EnforceUpdate] runs on every entity write, from
// entitymanager, where it is a required collaborator in the fixed write
// pipeline, like validation and audit. Given the old and new entity it
// finds every changed state-machine property, and for each checks:
//
//   - legality: the (from,to) move must be a declared edge, else
//     [ErrIllegalTransition] (422);
//   - guard: on a served path (a principal is present), the edge's guard
//     permission must be held for the subject, checked through the
//     injected [PermissionGuard], else [ErrGuardDenied] (403);
//   - when: the edge's compiled precondition must hold, else
//     [ErrPreconditionFailed] (422).
//
// The move is decided by [flow.Decide], the decider realtime sessions and
// flowengine use: the guard is the poster rule (Rules.Admits), so it runs
// before the precondition (Rules.Holds), and a precondition that fails to
// evaluate stops the decision. [Transitions.Performable] answers the
// read-side question "which moves can this principal make" through the same
// decision.
//
// A headless flow does not read an unset property as its initial state
// (DEC-EJ00DNZ): every move from an unset value is refused.
//
// The machines are built from the metamodel at boot and injected; the write
// path never re-derives them. This keeps `metamodel` a declarative source
// and keeps `acl` a permission oracle that knows nothing about state
// machines. The enforcer owns old-versus-new and the transition semantics;
// it asks the ACL a permission question whose permission name comes from
// its own compiled edge.
//
// # Purity
//
// The package does no I/O of its own. It reads the graph only through the
// collaborators the caller injects: [PermissionGuard], [GraphLookup] and
// [TraversalBinder].
package transitions
