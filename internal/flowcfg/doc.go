// Package flowcfg loads flows.yaml: the processes an operator declares,
// each a state machine that runs on the engine in internal/flow without a
// channel (FEAT-J0O10QI, TKT-XHANLJU).
//
// [Parse] reads the file against the schema and acl.yaml and returns a
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
// The package is pure: it reads only its arguments.
package flowcfg
