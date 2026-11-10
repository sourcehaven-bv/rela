---
id: PLAN-MFSBIYB
type: planning-checklist
title: 'Planning: Flows M2: step forms and run UI'
started: "2026-11-10"
completed: "2026-11-10"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:**

M1d runs flows a person starts with `rela flow start`; nobody can do a step yet,
and runs are visible only through `rela flow runs`. This ticket lets people do
their steps and see runs, in the SPA and over MCP.

- In: `writes:` on a state in flows.yaml: the subject properties the step
form edits. Load checks: declared on the subject type, writable kind (not
computed, roll-up or reserved), only on a non-final state with `waiting_for`.
- In: completing a step. The caller must hold one of the state's
`waiting_for` roles on the subject. The form values (only `writes` properties)
are written as the caller through `AuthorizeUpdate` and the grant's
`PatchEntity` (row ACL, field write gate, validation, audit with the flow step).
Then the engine fires the state's `change` branches for that caller (`roles:`
checked against the caller's roles on the subject, `when:` on the subject as
written). A stale generation is a conflict.
- In: engine `Change`, `Retry` and `Explain`; a bounded timeline of moves
(state, time, cause, by) kept in the run's stored state envelope.
- In: "why a run is waiting": the roles it waits for and whether the caller
holds one, each timer with its due time, each `change` branch with its roles and
whether its guard holds now, a pending step with its failure count and next
attempt, or a stranded run.
- In: data-entry API under `/api/v1/_flows`: flow list (config), the runs
of one subject (entity panel), run detail with `_actions` (complete, cancel,
retry), complete, cancel, retry, manual start, the run list (paged through the
generic list pipeline on `_flow-run`) and "my tasks".
- In: SPA: an entity-page panel per run (stepper reused from the channel
session panel, why-waiting, step form, diagram with the current state, timeline
with gaps), a run list with retry and cancel, a "my tasks" view, and a manual
start button.
- In: MCP tools `list_flow_tasks`, `show_flow_run`, `complete_flow_step`
on both transports.
- In: docs (GUIDE-flows, data-entry API reference, MCP tools).
- Out: `change` events from ordinary edits, triggers, the flow worker,
retention and the channel `_inbox` stay in TKT-FWQZGAM; "my tasks" is the read
model it can reuse. Policy for edits outside a flow is M3. Jobs in flight are M4
(the pending step is the analogue shown now).

**Acceptance Criteria:**

1. A step is completed only by a holder of a waiting role, writes only
`writes` properties as the caller through the field gate, and moves the run
through its `change` branch - `TestFlowControl_CompleteStep` (appbuild, table:
holder, non-holder 403, hidden run 404, property outside writes, stale
generation, no branch applies, field gate refusal).
2. `writes` load checks - `TestParse_Writes` (flowcfg, table).
3. Engine `Change`, `Retry`, `Explain` and the timeline -
`TestEngine_Change*`, `TestEngine_Retry`, `TestEngine_Explain` (flowengine).
4. The run list and the task list are row-gated and size-independent -
`TestQueryBudget_FlowRuns` (dataentry) and `TestFlowControl_TasksBudget`
(appbuild): the same store read count at 10 and 50 runs; a run on a hidden
subject is neither listed nor counted.
5. HTTP: `_actions` agree with the POST gates, hidden runs are the uniform
404, no identity fails closed - `TestFlowsAPI_*` (dataentry).
6. MCP tools list tasks, show a run with its waiting reason, and complete a
step - `TestFlowTools_*` (mcp).
7. E2E: start the sample flow from the entity page, fill in the step form,
complete the step and see the run finish - `e2e/tests/flows.spec.ts`.

## Research

- [x] ~~For larger features: run `/research` to create a structured research doc~~ (N/A: the design is settled in FEAT-J0O10QI and the M1 tickets; this ticket adds surfaces over it)
- [x] ~~Searched for existing libraries that solve this problem~~ (N/A: builds on the in-repo engine, list pipeline and SPA components)
- [x] Checked codebase for similar patterns or reusable code
- [x] Looked for reference implementations in other projects
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** N/A

**Existing Solutions:**

- Channel sessions: `RlChannelSession` stepper, the `_inbox` paging shape
(cap, sort, page in memory, count after the gate).
- `appbuild.FlowControl` (gated reads, manual permission), `flowengine`
(`Decide`, `move`, `settle`), `visibility.HolderLookup` (gated property match
pushed into the store), `acl.Request.LocalRolePredicate` and `Globals`.
- `entitymanager.AuthorizeUpdate` + `UpdateGrant.PatchEntity`;
`dataentry.listPage` for a gated, paged list; `storetest.Counting`.
- `mcpwire` for MCP tool groups over appbuild services; `dataentrywire`
setters for data-entry.
- Waymark's run view: the reason a run is not moving, shown next to the run.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:**

- New leaf package `internal/flowview`: the read-model types (run, task,
waiting reason, timeline) that appbuild fills and dataentry and mcp map to their
wire shapes. dataentry and mcp cannot import appbuild.
- `flowengine`: `Change(ctx, id, generation, holds)` decides the `change`
branches with `flow.Decide` (Admits: branch roles held; Holds: guard on the raw
subject), moves and settles; refuses a run that is settling or at another
generation. `Retry` settles a run with a pending mark now. `Explain` reports the
current state, timers and due times, change branches with guard verdicts,
pending and stranded. Moves append to a timeline in `flowrun.State.Log` (at most
32 entries, carried by every update).
- `appbuild.FlowControl` gains `Flows`, `SubjectRuns`, `Run`, `Complete`,
`Retry` and `Tasks`. Reads go through the gated reader; the caller's roles on
the subject come from the bound `acl.Request` (`ForEntity` for one subject;
`Globals` + `LocalRolePredicate` per role for the task list). Fails closed
without a bound request.
- Completing a step: identity, header gate (uniform 404), run store, then
generation, `writes` and the waiting role. The engine claims the step with a
compare-and-set (generation, caller, time) before the write, so a second
completion at the same generation is a conflict before it writes; `Change`
clears the claim, and a claim older than two minutes is void. The write goes to
`run.Subject` (id and face). The answer is `{written, moved, state, waiting}`.
- Every engine update goes through one helper that carries the run's
timeline and claim; a test fails on a direct `Runs.Update` elsewhere.
- Without acl.yaml every role counts as held (operator shell).
- Tasks: a new `visibility.HeaderLookup` composes the row gate with caller
`Props` (status `input_required`) and `Narrowing` (the flows where one of the
caller's roles, held globally or conferrable through a role relation, is waited
for) and reads headers in the store, capped. Then `_carries` edges in one query,
one role-holding query per (subject type, local role), and the run's own version
decides its waiting roles (compiled once per version per operation). Past the
cap the list is served with `truncated` set after the gate. Sorted and paged in
memory, as the channel inbox does.
- `complete` in `_actions` and the POST share one check: waiting role and
the row update decision (asked without a denied-write audit row). Field verdicts
come from the subject's own `_fields`.
- `Explain` never echoes error text: failure count, next attempt, and a
fixed text for a guard that fails to evaluate.
- The header may lag the run store after a failed header patch; complete
revalidates against the run store.
- Data-entry: `SetFlows(app, svc)` with a consumer-side interface; the run
list calls `listPage` on `_flow-run` with `filter[flow]`/`filter[status]` and
resolves the page's subjects with one edge query and one header batch.
`_actions` on run detail come from the same checks the POSTs call.
- MCP: `mcp.WithFlows` option with a consumer-side interface,
`mcpwire.Flows(svc)` adapter, wired in the stdio and remote servers.
- SPA: `api/flows.ts`, `FlowRunsPanel`/`FlowRunCard` on the entity page,
`FlowRunsView`, `FlowRunView`, `FlowTasksView`; mermaid through
`renderMermaidDiagrams`; timeline through `RlTimeline`.
- Diagram: `flowcfg.Flow.MermaidAt(state)` marks the current state with a
class; `internal/mermaid` gains the option.

Alternatives rejected:

- Timeline from the audit log: the audit sink is write-only JSONL with no
reader, and scanning daily files per request is not a read path. The run's own
envelope carries the same provenance (step, by, cause) for active runs; finished
runs keep start and end on the header. Noted as a decision.
- A `flow:complete` key on the subject's entity `_actions`: it would add a
run lookup to every entity GET. The key lives on the run detail instead.
- Gating the task list after paging the run store: an empty page with a
next cursor would reveal hidden runs.
- A new event type for step completion: `change` already means "the
subject changed, by someone with these roles"; TKT-FWQZGAM wires ordinary edits
to the same event.

**Files to modify:**

`internal/flowcfg/{raw,load,compiled,check,mermaid}.go`,
`internal/mermaid/mermaid.go`, `internal/flowrun/{flowrun.go,kvflowrun,
sqlflowrun,flowruntest}`, `internal/fsimport` (log copy),
`internal/flowengine/*`, `internal/flowview/*` (new),
`internal/appbuild/flowcontrol*.go`, `internal/dataentry/flows_*.go`,
`internal/apiwire/v1/flows.go`, `internal/dataentrywire/{wire,mcp}.go`,
`internal/mcp/tools_flows.go`, `internal/mcpwire/flows.go`,
`internal/cli/mcp_wiring.go`, `.go-arch-lint.yml`, `frontend/src/...`,
`e2e/tests/flows*.ts`, GUIDE-flows and the API/MCP guides.

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:**

- HTTP and MCP: run id (checked by `flowrun.CheckID`, resolved through the
header gate), subject address (gated reader), generation (integer), properties
(only the state's `writes`, values validated by the entity manager), flow name
(must exist), paging (bounded).
- flows.yaml `writes:` (load checks above).
- Stored run state: `flowrun.CheckState` bounds and checks the timeline.

**Security-Sensitive Operations:**

- The step write runs as the caller through the field-gated manager; the
waiting-role check is an extra allowlist, never a substitute for the ACL.
- Cancel, retry and start need the flow's manual permission on the subject.
- Every read of a run goes through the header's row gate; a hidden run, or
a run on a hidden subject, answers like a missing one.

### Threat model (POLICY-015 §3, PROCEDURE-j5mq step 1)

| STRIDE | Threat | Mitigation |
| ------ | ------ | ---------- |
| Spoofing | A caller completes a step as another person, or a step write runs as system:flow or run_as | Identity is bound once at the edge (HTTP middleware, stdio MCP `BindIdentity`); FlowControl reads the bound Request and fails closed without one. The step write runs as the caller; only headers use system:flow. |
| Tampering | A step form writes properties outside the step, or past the caller's grants; a stale form moves a run that moved on | Only `writes` properties are accepted; the write goes through `AuthorizeUpdate` and the field gate. The form carries the generation it saw; another generation is a conflict. A settling run refuses a change. |
| Repudiation | A step completion cannot be traced to a person | The write is audited with the caller as principal and the flow step (`flow`, `run`, `step`, `by`); the move is recorded in the run's timeline with `by`. |
| Information disclosure | The run list, task list, run panel or MCP tools reveal a run on a hidden subject, its existence through counts or cursors, or a hidden value through a guard verdict | Headers are read through the row gate pushed into the store; counts are taken after the gate; a hidden run is the uniform 404. Guards read only properties no `visible:` grant restricts (flowcfg load check), so a verdict reveals nothing a reader of the subject cannot read. The timeline names who moved a run, as version history names who wrote an entity. Config (flow, state, role names) is not secret. |
| Denial of service | A task list or run list scans every run, or many runs flood a page | Lists are paged in the store or capped (task candidates) with an error past the cap; the budget tests pin a size-independent read count; the timeline is capped at 32 entries. |
| Elevation of privilege | A caller without a waiting role completes a step, or without the permission cancels, retries or starts a run | Complete needs a waiting role on the subject (global or through a role relation); cancel, retry and start need the manual permission on the subject. `_actions` and the POST gates call the same checks (contract test). |

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:** see Acceptance Criteria.

**Edge Cases:**

- A run that moved since the form was loaded (stale generation).
- A run settling after a failed output (complete refused, retry offered).
- A run on an older flow version (its own definition decides the form and
branches).
- A completion whose `when:` does not hold (write kept, run stays, reason
returned).
- A role held only through a role relation on the subject.
- Timeline capped at 32 entries.

**Negative Tests:**

- No identity, hidden run, unknown run, non-holder, property outside
`writes`, field write refused, cancel/retry without permission, retry with
nothing pending, finished run.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:**

- Without the flow worker (TKT-FWQZGAM) timers and automatic retries do not
run in a server; the run panel shows the due time and the operator retry covers
a failed step. Documented.
- The timeline covers active runs only; finished runs show start and end.
Documented.

Effort: l.

## Documentation Planning

- [x] User-facing docs identified (skip if internal refactor)
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:** GUIDE-flows (`writes:`, step forms, run panel, run
list, tasks, MCP tools, status), the data-entry API reference and the MCP tools
guide.

## Design Review

- [x] Run `/design-review` before starting implementation
- [x] All critical/significant findings addressed in plan

**Design Review Findings:** 15 findings (3 critical, 6 significant, 5 minor, 1
nit), recorded as review responses on the ticket and addressed in the approach
above: step claim before the write, timeline carried by one update helper, task
candidates narrowed by role and truncated rather than failing, a `Headers` query
method on `visibility.HolderLookup`, per-version compile cache, change-event
generation contract (written into TKT-FWQZGAM), roles in flows.yaml need
acl.yaml (so no roles-without-ACL path exists), check order and per-face header
gate, one complete check for `_actions` and the POST, face-exact write, complete
response shape, no error text in `Explain`, timeline decision, header lag,
`flowview` leaf.

**Decisions made during implementation:**

- The timeline lives in the run's state envelope (`Log`, at most 32
entries: the start and the newest moves), not in the audit log.
- A completed step writes through the field-gated manager with the flow
step on ctx, then calls `Engine.Change` with the generation it saw; the existing
change event still fires (TKT-FWQZGAM relies on this).
- Tasks match the current flows.yaml version only and stop at 500
candidates with `truncated: true`.
- The `flowview` JSON values are the wire shape; there are no separate
apiwire types.
- Retry needs the same manual-start permissions as cancel.
- `SubjectRuns` explains its runs in one batch `Explain`, so the subject
is read once.
