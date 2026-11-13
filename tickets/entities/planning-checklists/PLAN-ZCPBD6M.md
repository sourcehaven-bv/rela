---
id: PLAN-ZCPBD6M
type: planning-checklist
title: 'Planning: Flows M4a: parallel branches'
started: "2026-11-11"
completed: "2026-11-11"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:** In: fork and join in flows.yaml (`to:` lists, `join:`), the load
checks, the run state for parallel branches in every run store, the engine
(settle, timers, change, claim, retry, explain), the read model (views, tasks,
outside_edits), the web app panel, MCP and docs. Out: job steps (TKT-PY2G11W),
the flow worker (TKT-FWQZGAM), nested forks and forking timers.

**Acceptance Criteria:**
1. A flow without forks loads, runs and stores exactly as before: the
existing flowcfg, flowengine, flowrun and appbuild tests pass unchanged, and a
run without branches encodes to the same JSON.
2. A fork moves the run into every target; each branch waits, moves and
writes outputs on its own (engine tests).
3. `join: all` continues once every branch arrives; `join: any` on the
first; `join: {when:}` early when the predicate holds (engine tests).
4. A branch that reaches a final state ends the run (engine test).
5. The load refuses a fork without a join, overlapping regions, nested
forks and edges into a region (flowcfg tests).
6. The run-store conformance suite covers branches on kv, sqlite and
postgres.
7. Views, tasks and completing a branch step work over HTTP (appbuild
tests); the web app shows each branch (vitest).

## Research

- [x] ~~For larger features: run `/research` to create a structured research doc~~ (N/A: the approach follows the standard fork/join of statecharts and BPMN parallel gateways; the options were settled in the M4 split with the user)
- [x] Searched for existing libraries that solve this problem
- [x] Checked codebase for similar patterns or reusable code
- [x] Looked for reference implementations in other projects
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** N/A

**Existing Solutions:** BPMN parallel gateways (fork and join with "all"), BPMN
inclusive/complex gateways (a join condition) and SCXML parallel states. No Go
library fits: the engine's state, CAS store and ACL gates are rela's own. The
pending mark (RR-U30RNUZ), the step claim (RR-LGRBAP4) and the generation check
are reused per branch.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:**

#### flows.yaml

A branch (`on.change` or `on.done`) whose `to:` is a list forks the run:

```yaml
submitted:
  waiting_for: [manager]
  on:
    change: { to: [legal, finance], label: Send for review }
legal:
  waiting_for: [lawyer]
  on:
    change: { to: reviewed }
finance:
  waiting_for: [controller]
  on:
    change:
      - { to: reviewed, when: entity.budget_ok == true }
      - { to: rejected }
reviewed:
  join: all
  on:
    done: { to: approved }
```

- `join:` marks the state that closes a fork. `join: all` waits for every
branch. `join: any` continues with the first branch that arrives. `join: { when:
<predicate> }` continues when every branch has arrived, or earlier when the
predicate holds. The predicate sees `entity` (the subject, as in any guard),
`arrived` (int) and `branches` (int). It is compiled with the guard allowlist
and the visible: check. An evaluation error counts as false and is logged;
because a join always continues once every branch has arrived, a run never waits
on it for ever.
- The join is tested only when no live branch has a pending mark, so a
withdrawn branch never writes after the join's outputs. An arrival is recorded;
the settle that leaves no pending mark tests the join.
- When the join continues, the branches that have not arrived are
withdrawn. Outputs they wrote stay.
- A branch that reaches a final state ends the whole run with that
state's status; the other branches are withdrawn.
- Once entered, a join state is an ordinary state: outputs, `on:`,
`after:` and `waiting_for:` work as anywhere; its timers count from when the
join fires. A loop back to the fork state forks afresh.
- Timers keep one target. A fork has 2 to `MaxForkTargets` (8) distinct
targets. One state may hold several fork branches when their regions and joins
are distinct.
- A branch cannot leave its region for rework. The alternative is a
`join: {when:}` that fires on a rejection, followed by a guarded done branch on
the join.

Code: `flowcfg.Branch` keeps `To` for an ordinary branch and gains `Fork
[]string` (with `To` empty) and `Targets()`, which every graph walk uses.
`flowcfg.State` gains `Join()`. The wire `Branch` gains `fork`.

Load checks (one fork level in this release):

- The region of a fork target is every state reachable from it without
passing the join or a final state.
- The regions of one fork are disjoint and each reaches exactly the one
join the fork closes. A join closes exactly one fork. From every region state
the join or a final state is reachable.
- A region holds no fork, no join, not the initial state and not the fork
state. No state outside a region moves into it, and only the regions of its fork
move into a join.
- A fork target is neither final nor a join.
- `reachable` and `doneCycles` follow every fork target.

#### Run state

- `flowrun.Run` embeds `flowrun.State`, so backends and the engine carry
every field without copying them one by one. `State` gains `Branches`: per live
branch its name (its first state), instance, pending mark, claim and `arrived`.
`flowrun.MaxBranches` (8) bounds it; `CheckState` checks each branch like the
run-level instance. `Step` gains `branch`. A conformance test round-trips every
field on kv, sqlite and postgres. No table or column changes: the envelope is
JSON.
- While forked, the run-level instance has an empty state and a nonzero
generation, with `Previous` set to the fork state. `flow.Read` reports it
stranded, so code that does not know branches, including an older binary on the
same database, refuses it rather than acting on it.
- Generations are run-wide: the engine sets each new generation to the
highest in the run plus one after `Enter`. The (run id, generation) API names
exactly one step. The wire generation of a forked run is 0.
- The run-level `NextDueAt` stays the index hint: `save` computes it as the
earliest due time over all tokens.
- `flowrun.Query` gains `IDs` (run ids, at most `MaxPage`).

#### Engine

- One token lookup finds the step a generation names: the run-level
instance when not forked, else a live branch.
- Every write goes through one bounded read-modify-write helper. On a
conflict it re-reads and re-applies the change when its own token is still at
the same generation and mark; it reports the run lost only when its token moved
or is gone. Claim uses it too; exhaustion is a retry-later error, not
`ErrClaimed`.
- A fork, a join that fires and a branch move to a final state are each one
compare-and-set: the join and the final move drop the branches and move the
run-level instance into the target with a pending mark, so the existing settle
and finish paths write the outputs and end the run.
- settle, Due/advance (every due token), Change, Retry and Explain work
per token. Branches settle in fork target order. Backoff counts and delays per
token.
- The header is written at the fork (state = the join, status
input_required when any region state waits for a role, else working) and again
only when the join fires or the run ends.

#### Read model and clients

- `Explanation.Parallel` and `flowview.Run.Parallel` list the branches,
each with state, label, status, generation, waiting reason and whether the
caller may complete it. The waiting blocks come from the same function as today,
including the rule that clears guard verdicts when the caller cannot read the
subject.
- Tasks: a join whose fork's regions wait for a role the caller may hold
is a candidate group; the candidate runs are read in one call by run id (budget
test); each branch the caller can act on is a task.
- outside_edits: a forked run is governed by every live branch's state.
A step's own write stays exempt from its run's rules (`WithinRun`): it writes
only its state's `writes:`, which the flow author listed.
- When the flow worker (TKT-FWQZGAM) adds a change hook, a subject change
is offered to every live branch.
- The web app shows a waiting block and step form per branch; the diagram
marks every active state. MCP and CLI pass the new fields through.

**Alternatives rejected:** one run per branch (child runs) would need a
parent/child relation, break `per_subject: one` and split the timeline; a
general Petri-net marking with nested forks is more than the use cases need and
makes the load checks much harder. A join `when:` that may wait after every
branch arrived would need the flow worker to re-test it.

**Files to modify:** `internal/flowcfg` (raw, states, check, compiled, mermaid,
guard env), `internal/flowrun` (+ kv and sql backends, flowruntest),
`internal/flowengine`, `internal/flowedits`, `internal/appbuild/flowsteps.go`,
`internal/flowview`, `internal/mermaid`, `frontend/src/api/flows.ts`,
`frontend/src/components/flows/*`, i18n, GUIDE-flows.

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:** flows.yaml (operator config): strict decoding,
the load checks above, `MaxBranches` targets per fork, the join guard compiled
with the guard allowlist and the visible: field check. The stored run state:
`CheckState` bounds the branch list and checks each branch like the run-level
instance. HTTP: a step names (run, generation); the generation must name a live
branch.

**Security-Sensitive Operations:** Completing a step: the same checks run per
branch (waiting role on the subject, read gate, `AuthorizeUpdate`, claim).
Outputs still write as run_as through entitymanager.

**Threat model (POLICY-015 §3, PROCEDURE-j5mq step 1).** Assets: subject data,
the run state, who may complete a step. Entry points: flows.yaml, the step,
retry and cancel endpoints, MCP complete_flow_step. Threats and mitigations: (1)
a person completes a branch they do not wait for: the role check uses that
branch's state, and the generation must match a live branch, so a stale or other
branch's generation is refused. (2) A join guard discloses hidden values: it is
compiled under the same visible: check as every guard, and its verdict is not
shown. (3) A forked run grows without bound: forks are one level, targets are
capped, and the branch list is checked on every store write. (4) Concurrent
steps in two branches: each write is a compare-and-set on the run's rev; the
loser re-reads, so a join fires once. (5) Tasks leak another subject's run:
candidates still come from the gated header query and subjects from the gated
read. No new trust boundary.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:** As in the acceptance criteria: flowcfg load tests,
flowengine tests with the in-memory adapters, flowruntest conformance on all
backends, appbuild HTTP-level tests and vitest for the card.

**Edge Cases:** both branches completed concurrently (one CAS loses and
retries); an output fails in one branch (backoff counts per branch, the other
branch still moves); a timer in a branch; a branch looping inside its region; a
run on an older version with a fork; cancel while forked.

**Negative Tests:** stale generation of a withdrawn branch; completing a branch
for a role you do not hold; flows.yaml with a fork into a final state, a join
closing two forks, a state entered from outside a region, a nested fork.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:** Regressions for single-state runs: mitigated by keeping the run-level
instance as the only token of an unforked run and by running the existing suites
unchanged. Concurrency at the join: mitigated by the rev compare-and-set.
Effort: l.

## Documentation Planning

- [x] User-facing docs identified (skip if internal refactor)
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:** GUIDE-flows (parallel branches section), regenerated
into `docs/`.

## Design Review

- [x] Run `/design-review` before starting implementation
- [x] All critical/significant findings addressed in plan

**Design Review Findings:** 19 review-responses linked to TKT-ZO9IB6V (1
critical, 8 significant, 7 minor, 3 nit), all addressed in the Technical
Approach above.
