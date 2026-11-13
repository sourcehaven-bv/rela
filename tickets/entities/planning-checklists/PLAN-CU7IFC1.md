---
id: PLAN-CU7IFC1
type: planning-checklist
title: 'Planning: Flows M5: compile transitions and automations onto the flow engine'
started: "2026-11-12"
completed: "2026-11-12"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:**

M5 as written (transitions, automations, delete the old paths, dogfood) does not
fit one reviewable PR. It is split into three sequential tickets:

- M5a, this ticket: enum `transitions:` compile to headless flows.
- M5b, TKT-GS7THPA: `automations:` compile to entity-change triggers with
outputs. Depends on this ticket.
- M5c, TKT-ZRQSGCM: delete the remaining old paths and dogfood the tickets
checklist workflow as a flow. Depends on M5b.

In scope for M5a:

- Each state-machine custom type compiles to a headless flow: a
`flow.Definition` whose states are the enum values (declaration order) and whose
initial state is the entry value (`initial`, else `default`), plus the declared
transitions as branches per state.
- Write enforcement (`Set.EnforceUpdate`) and the read-side verdicts
(`Set.Performable`) decide a move through `flow.Decide`, the decider that
realtime sessions and flowengine use. The guard permission is the poster rule
(`Admits`), the `when:` precondition is the guard (`Holds`).
- The state is the entity's own property. No `_flow-run` header, no run
store entry, no timers, no outputs.
- The old edge-map evaluation path (`edges` map, `edgeFor`) is removed.

Out of scope: automations (M5b), deleting `internal/statemachine` as a package
and the dogfood flow (M5c), showing headless flows in `rela flow` tooling, and
any change to the transitions syntax. No migration is needed: the schema syntax
and the stored data do not change.

**Acceptance Criteria:**

1. Every existing statemachine, entitymanager, affordances and dataentry
transition test passes unchanged.
2. A compiled machine exposes its `flow.Definition`: states equal the enum
values, initial equals the entry value.
3. Enforcement and verdicts route through `flow.Decide`; no other legality
or gate evaluation exists in the package.
4. Error classes are unchanged: undeclared move 422 (`ErrIllegalTransition`),
unheld guard 403 (`GuardError`), false or failing `when:` 422
(`ErrPreconditionFailed`), illegal create 422 (`ErrIllegalEntry`).
5. The guard is checked before `when:` (a caller without the permission gets
403 whatever the precondition says).
6. The `tickets/` and `docs-project/` projects load and pass their analyze
gates with a binary built from this branch.

## Research

- [x] For larger features: run `/research` to create a structured research doc
- [x] Searched for existing libraries that solve this problem
- [x] Checked codebase for similar patterns or reusable code
- [x] Looked for reference implementations in other projects
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** N/A. The approach follows the settled FEAT-J0O10QI design
("one engine, existing features compile onto it").

**Existing Solutions:**

- `internal/realtimecfg` compiles realtime.yaml into a `flow.Definition`,
and `internal/realtime/session.go:593` decides with `flow.Decide`. M5a follows
that precedent: config compiles to the engine's definition, the runtime decides
with the engine's decider.
- `internal/flowengine/guards.go` and `person.go` also decide with
`flow.Decide`.
- No external library applies; the engine is rela's own.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:**

- `statemachine.Machine` holds `def *flow.Definition` (unexported) and
`out map[string][]branch` (from-state to its transitions in declaration order).
`branch` carries `to`, `guard`, `when`, `label`.
- `compileMachine` builds the definition with `flow.NewState(value, final,
nil)` per enum value and `flow.NewDefinition(entry, states, nil)`. A value with
no transition to another value (self-loops excluded) is final. The existing load
checks stay (dangling from/to/entry, duplicate edge, bad `when:`, list property,
default not the entry value, traversals).
- `Machine.branches(from, to)` selects the branches of a move, the way
`flowcfg.Flow.Branches(state, event)` and realtimecfg key branches by event: the
event of a headless flow is "set the property to `to`".
- One function, `decideMove`, decides a move. An empty `from` is refused
as undeclared (a headless flow does not read unset as initial, unlike
`flow.Read` for an unset instance; documented on `Machine`). The current state
is read with `flow.Read`; a stranded value is refused as undeclared with a
message naming it. More than one selected branch fails closed. It then runs
`flow.Decide`: `Admits` asks the guard and records a refusal (nil guard refuses
a guarded branch); `Holds` evaluates `when:` (no `when:` holds; a bind error
fails only a traversing `when:`). Outcomes: `Moved` allowed; `NoMove` after a
refusal guard; `NoMove` otherwise precondition not met; `GuardFailed`
precondition error; `Gated` and any other outcome fail closed as a precondition
error. The invariant (one branch per move, enforced by the duplicate-edge load
error) is written on `decideMove`. `EnforceUpdate` and `Performable` both call
it.
- `Performable` returns nil for an unset, stranded or final state through
the definition.
- `EnforceCreate` and `EntryValue` read `def.Initial()`.
- Helpers that walked the edge map are rewritten over the branches:
`traversalProblems`, `TraversingWhens` (its `from→to` label is unchanged, the
affordances lint reads it), compile's duplicate detection.
- The public API of the package does not change, so no caller changes.
- Arch-lint: `statemachine` may depend on `flow` (a leaf).

**Alternatives considered:**

- Use the guard permission as `Rules.Gate`. Rejected: `Gate` runs after
`Holds`, so the precondition would be evaluated for a caller without the
permission and the 403/422 difference would disclose its outcome.
- Compile transitions into `flowcfg.Flow` and run them in `flowengine`.
Rejected for M5a: flowengine is run-oriented (header, run store, outputs), and a
headless flow has none of those. The `when:` env also differs from the
flows.yaml guard env, and mixing two envs in one branch type is a trap.
- Move the compile into `flowcfg`. Deferred to M5c, where the statemachine
package is folded; moving it now churns every caller for no behaviour gain.

**Files to modify:**

- `internal/statemachine/statemachine.go`, `compile.go`, `enforce.go`,
`resolve.go`, tests.
- `.go-arch-lint.yml` (statemachine may depend on flow).

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:**

- `schema.yaml` transitions (operator config, not secret): validated at load
as today; a malformed machine fails the boot.
- The written entity (caller data): only the state property's old and new
values select branches; a value not declared selects no branch and the write is
refused (allowlist).

**Security-Sensitive Operations:**

- The guard permission check on a transition (authorization).
- The `when:` precondition reading entity data and relations.

### Threat model (POLICY-015 §3, PROCEDURE-j5mq step 1)

Assets: the integrity of an entity's lifecycle (a guarded state change is an
authorization decision), and the confidentiality of entity data that a `when:`
precondition reads.

Actors: a caller writing through the data-entry API, MCP, CLI or Lua (all
through entitymanager); an operator authoring schema.yaml (trusted).

| Threat (STRIDE) | Vector | Mitigation in this PR |
| --- | --- | --- |
| Elevation of privilege | `flow.Decide` skips a branch its poster rule refuses; if a skip read as "no move" the caller could get a 422 instead of a 403, or a later branch could apply | One branch per (from, to); a refusal by `Admits` is recorded and mapped to `GuardError` (403). Test: unheld guard returns `ErrGuardDenied` |
| Elevation of privilege | A create enters a guarded state directly | `EnforceCreate` still pins the entry value from `def.Initial()`; existing tests |
| Information disclosure | Evaluating `when:` for a caller without the guard permission discloses the precondition through 403 vs 422 | Guard is `Admits`, which runs before `Holds`. Test: unheld guard plus false `when:` returns 403 |
| Information disclosure | A failing `when:` falls through to another branch | `flow.Decide` stops on a guard error (fail closed); mapped to precondition error |
| Tampering | A value outside the enum selects a branch | Branches are keyed by declared values only; an undeclared from or to selects none and is illegal |
| Denial of service | More work per write | Same O(out-edges of one state) as before, one traversal bind per write |
| Repudiation | Audit of the write | Unchanged: entitymanager still audits every write and `denied-write` |
| Spoofing | n/a | Identity is resolved at the edge as before; the guard adapter is unchanged |

Residual: none new. The read-side verdict keeps the accepted one-bit channel of
`Performable` (documented in the affordances lint).

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:**

1. AC1: race tests on `internal/statemachine`, `internal/entitymanager`,
`internal/affordances`, `internal/dataentry`, `internal/appbuild`.
2. AC2: in-package test that the compiled machine's definition lists the
enum values as states, the entry value as initial, and the right finals.
3. AC3: a cross-product test over guard (none, held, unheld, nil), `when:`
(none, true, false, related true, related false) and bind error asserts that the
`Performable` verdict matches the `EnforceUpdate` error class; a test pins one
branch per compiled move; a test hands `decideMove` two branches and expects a
refusal; a test maps `flow.Gated` to a refusal.
4. AC4: table test per outcome (illegal, guard, precondition false,
precondition error, allowed, illegal entry).
5. AC5: unheld guard with a false `when:` returns `GuardError`.
6. AC6: build `rela` from the branch and run the three analyze gates on
`tickets/` and `analyze all` on `docs-project/`. A CLI smoke run on a scratch
project makes one legal move, one undeclared move (refused) and one move whose
`when:` is false (refused).

**Edge Cases:**

- Old value empty (property unset before) or not a declared value: illegal.
- Self-loop declared: skipped by `EnforceUpdate` (unchanged value) and by
`Performable`.
- Several machine properties changed in one write: checked in property-name
order, first failure wins (as today).
- Named face with a traversing `when:`: precondition fails closed (as today).

**Negative Tests:**

- Undeclared move, unheld guard, false and failing `when:`, illegal entry.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:**

- Behaviour drift on the hot write path. Mitigation: public API unchanged,
every existing test kept, explicit outcome table test.
- Order of guard and precondition. Mitigation: guard is `Admits`, test pins
the order.

Effort: m for M5a (the ticket's l covered all of M5).

## Documentation Planning

- [x] User-facing docs identified (skip if internal refactor)
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:**

- [x] N/A - Internal change, no user-facing docs needed. The schema syntax
and behaviour do not change. The package doc of `internal/statemachine` records
that machines compile to the flow engine.

## Design Review

- [x] Run `/design-review` before starting implementation
- [x] All critical/significant findings addressed in plan

**Design Review Findings:** RR-2ZD9R0W, RR-PVOEULJ and RR-CG1QZ65 (significant);
RR-PTN06M0, RR-DYNTUV5, RR-3Z2TDI2, RR-DDK3E5Q and RR-KVC52JS (minor);
RR-5ABLP8W and RR-S28B0B7 (nit). All are addressed in this plan.
