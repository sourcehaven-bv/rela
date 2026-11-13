// Package flowcfg compiles the flows of a project onto the engine in
// internal/flow (FEAT-J0O10QI). There are two kinds.
//
//   - A run-based flow is declared in flows.yaml: a process an operator
//     declares, each a state machine whose runs are recorded as `_flow-run`
//     entities (TKT-XHANLJU).
//   - A headless flow is compiled from the `transitions:` of an enum type in
//     schema.yaml: its state is the property of the entity itself, so it has
//     no run (TKT-12RC7JT, TKT-ZRQSGCM).
//
// # Run-based flows
//
// [Parse] reads flows.yaml against the schema and acl.yaml and returns a
// [Compiled] set of [Flow] values, or every problem it found. A flow names
// the entity type it carries (its subject), how a run starts, its states,
// the events that move a run between them, the timers that move it when
// nothing happens, and the outputs a state writes when a run enters it.
//
// The load refuses a file that cannot run: a reference that does not
// resolve, a guard that does not compile, a state no run can reach, a
// waiting role that cannot answer, and an output the flow's run_as may
// not write. What the file declares is configuration, not a secret, so
// the problems name keys and values freely.
//
// Each flow has a version: the hash of its own declaration, so editing one
// flow leaves the version of the others alone. A run records the version
// it started on.
//
// # Headless flows
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
// The package does no I/O of its own. Parse reads only its arguments.
// The headless enforcer reads the graph only through the collaborators the
// caller injects: [PermissionGuard], [GraphLookup] and [TraversalBinder].
package flowcfg
