// Package flowrun is the run store of declared flows: the execution state
// of each run, keyed by run id, outside the entity graph (FEAT-J0O10QI,
// TKT-PV9R0GK).
//
// A run's header is its _flow-run entity, which records history and is
// read through the ACL. Its execution state lives here: the state it is
// in, when it entered it, its timer schedule and reminder records, as a
// [flow.Instance], so the engine in internal/flow reads it unchanged. The
// run store is the authority on that state; the header is a projection
// the runtime writes after it.
//
// Retention is built in: [Store.Delete] drops a run's state when the run
// completes or is cancelled, so the store holds live runs only. The first
// run of a flow version stores the version's definition in the same step
// (see [NewRun.Definition]), and [Store.PruneDefinitions] drops the
// definitions no stored run uses.
//
// # Backends
//
// kvflowrun keeps every run in one document in a state.KV, for the fs,
// memory and desktop tiers. sqlflowrun is the one SQL implementation,
// which pgflowrun and sqliteflowrun adapt to their driver. Every backend
// passes flowruntest.RunAll, which is the one definition of the contract.
//
// # Consumers
//
// [Store] is declared beside its types because every backend must satisfy
// exactly it. A consumer still declares the subset it calls at its own
// site (the timers loop needs ListDue, Get and Update), so the whole
// interface does not spread.
//
// The store authorizes nothing. A run's state discloses facts about its
// subject, so a read path that shows runs to a person must go through the
// _flow-run entity's row gate, never through this store directly.
package flowrun
