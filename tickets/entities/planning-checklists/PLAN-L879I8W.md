---
id: PLAN-L879I8W
type: planning-checklist
title: 'Planning: Flows M5e: dogfood the tickets checklist workflow in tickets/flows.yaml'
started: "2026-11-16"
completed: "2026-11-16"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

The ticket body predates M5d (TKT-GWOBH2U, #2350). M5d already moved every
checklist and date automation from `tickets/schema.yaml` to the `triggers:` of
`tickets/flows.yaml`; `schema.yaml` has no `automations:` left, and the three
gates are clean on develop (d0764fe8d). The written acceptance ("runs its
checklist workflow from flows.yaml", "gates stay clean") is therefore met. What
is missing is the proof ("tested end to end on a scratch copy of tickets/") and
a decision on the stateful flow the brief asks about.

Findings from a scratch copy of `tickets/` (`.ignored/scratch`, branch binary):

- R2 is the lead blocker. `on: create` starts and `change` events from
ordinary edits are not wired (TKT-GYXOTQC, M1f, backlog), and the CLI has no
command to do a step (`rela flow` has run, validate, show, start, runs, cancel).
On the scratch copy `rela create ticket` started no run, and `rela update -P
status=planning` left a manually started run in `queued`. `flow start` also
needs the flow's manual permission, which ties to R6.
- R1. A flow `create` output must relate the new entity TO the subject.
`has-planning`, `has-implementation`, `has-review` and `has-bug-analysis` run
from ticket/bug to the checklist. `rela flow validate` refuses the draft: `names
"has-planning", which does not link planning-checklist to ticket`; the inverse
name `plans` is refused as undeclared.
- R3. On the file backend the `_flow-run` entity is a markdown file under
`entities/_flow-runs/` (committed with the data) while its working state is in
the local `.rela/flow-runs.json`. Agents work in many worktrees and merge
through git, so other checkouts would see runs that cannot advance, and an open
run blocks renaming or converting its subject on every surface.
- R4. Flow outputs have no `if_exists: skip` and no condition. A state graph
can avoid re-entry with distinct states and set `started` only in the first one,
so this is a cost, not a hard stop.
- R5. Ticket and bug status are free (no `transitions:`); a flow mirroring them
needs a change branch per ordered pair (56 per type) and only shadows `status`.
`outside_edits:` does not apply to the CLI, so a flow enforces nothing for
agents.
- R6. `waiting_for:` names acl.yaml roles and `outside_edits:` needs acl.yaml;
`tickets/` has no acl.yaml.
- R7 (stale binaries). `~/bin/rela` (5 Nov) predates `flows.yaml` support
(#2104), so it ignores the whole file: `update -P status=planning` succeeded and
created no checklist. A binary between #2104 and #2350 fails to boot on the
unknown `triggers:` key (`strictDecode`, `internal/flowcfg/load.go:196`), so the
silent window is only binaries without `flows.yaml` support. Only a project
version floor could close it, and none exists. The analyze gate
(`planning-ticket-needs-checklist`) catches the result afterwards.
- R8 (current behaviour, not a blocker). A `property:` trigger does not fire
on create (docs/flows.md:716). A ticket created with `status=planning`, a bug
created `in-progress` or a docs-checklist created `in-progress` gets no
checklist and no `started`.
- Validation rules here are warnings, not write refusals: no `enforce: write`
and no `validation: strict`, so `partitionValidationErrors`
(`internal/entitymanager/validation.go:84`) downgrades them on the CLI and in
`rela test`. The gates (checklists done, no open critical or significant review
responses) hold through `analyze` and the existing `rela validate` step of the
`rela-tickets` CI job. This ticket does not change that.

**Scope:**

In: an end-to-end `rela test` suite for the tickets checklist workflow (trigger
effects only), run in CI, with a coverage test that fails when a trigger has no
case; a CLAUDE.md note on stale binaries and on creating in a status; an updated
ticket body; a decision entity with an M5f sketch; follow-up tickets for R1, R3,
R4, the M5f flow and the managed workflow docs.

Out: a stateful `flows:` entry for ticket or bug (blocked by R1 to R6); engine
changes; `transitions:` on ticket_status/bug_status (a behaviour change that
needs Jeroen's decision); acl.yaml for tickets; making validation rules refuse
writes; edits to `@managed: claude-workflow` files (ticket.md, bug.md,
implement.md and the CLAUDE.md workflow block), which the template overwrites.

**Acceptance Criteria:**

1. `rela test` in `tickets/` passes and covers every status that has a trigger,
for ticket and bug (scenarios under Test Plan).
2. CI runs `rela flow validate` and `rela test --require-tests` for
`tickets/`, so a PR that breaks a trigger or deletes the tests fails.
3. A Go test fails when a trigger key in `tickets/flows.yaml` has no case
named after it in the suite; removing one trigger was also checked by hand to
fail the suite.
4. The three gates stay clean; no data migration is needed.
5. A decision entity records why the lifecycle stays trigger-driven, and
follow-up tickets exist for R1, R3, R4 and the stateful flow (M5f), linked with
`depends-on`.

## Research

- [x] ~~For larger features: run `/research` to create a structured research doc~~ (N/A: the findings above come from running the engine on a scratch copy; no open design space beyond the decision below)
- [x] ~~Searched for existing libraries that solve this problem~~ (N/A: dogfood of rela's own config)
- [x] Checked codebase for similar patterns or reusable code
- [x] ~~Looked for reference implementations in other projects~~ (N/A: dogfood of rela's own config)
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** N/A

**Existing Solutions:**

- `rela test` (`internal/apptest`): `*.test.lua` cases in a fresh in-memory
world from the project config; triggers run there
(`TestAutomationEffectsInSameCase`). Never opens the live store.
- `internal/flowcfg/states.go:412` (`relate` check, R1);
`internal/appbuild/flowruntime.go:101` (`flowRunBackend`, R3);
`internal/flowcfg/transitions` (headless flows, enforced on every write).
- docs/flows.md: Status note, Edits outside a flow, Moving from schema.yaml
automations. Concept `flows`, FEAT-J0O10QI, TKT-GYXOTQC.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:**

1. `tickets/tests/checklist-workflow.test.lua`: one case per scenario under
Test Plan, each acting as `t.as("agent")` (no acl.yaml, so any name may write).
Case names start with the trigger name they cover. `{{today}}` follows the case
clock, so `t.advance` checks are meaningful.
2. A Go test (next to the existing real-schema tests, e.g.
`internal/apptest/tickets_coverage_test.go`) reads the trigger keys of
`../../tickets/flows.yaml` and the `t.case` names of the suite, and fails on a
trigger without a case.
3. CI: in the `rela-tickets` job (`ci.yml:1654`, which builds `bin/rela`),
add `../bin/rela flow validate` and `../bin/rela test --require-tests` beside
the existing `rela validate` step. No `just` there, so the step calls the binary
directly. Dependabot PRs skip the job; push and merge_group runs still execute
it.
4. CLAUDE.md "Interacting with tickets & docs" (unmanaged) gets two lines:
build `rela` from develop at or after #2350, and `rela --project=tickets flow
validate` must succeed and report 13 triggers (an older binary fails or ignores
the file); and create a ticket or bug in `backlog` or `ready`, then move its
status, because creating it straight in a working status fires no trigger.
5. Ticket body rewritten to the delivered scope. DEC entity "Ticket lifecycle
stays trigger-driven until flows can carry it" with R1 to R8 and the M5f sketch
below. Follow-up tickets: FT-a flow `create` with `relate.from: subject`; FT-b
idempotent flow `create` and conditional outputs; FT-c run working state on
git-shared fs projects; FT-d update the claude-workflow template wording
("Automation creates" in ticket.md, bug.md, implement.md) to "a flows.yaml
trigger creates" plus the create-in-status note; M5f stateful ticket/bug flow,
depends-on TKT-GYXOTQC and FT-a/b/c.
6. GUIDE-flows "Examples": point at `tickets/flows.yaml` and its test;
regenerate `docs/flows.md`.

**M5f sketch (recorded in the DEC and M5f ticket):** flow `ticket-work`, subject
ticket, `per_subject: one`, start `on: create`. States = the status values; each
non-final state has `change` branches with `when: "entity.status == '<next>'"`
driven by M1f change events from CLI edits, so agents keep using `rela update -P
status=...`. `waiting_for: [agent]` needs a tickets acl.yaml with an `agent`
role. `done` and `wont-fix` are final (`completed`, `cancelled`). Checklist
creation and dates stay triggers until FT-a/FT-b land; then they may move into
state outputs. Same shape for `bug-work` with `analyzing`. Run state needs FT-c,
or the tickets project moves to a backend that stores runs with the data.

Alternatives rejected:

- B. Build the stateful flow now: needs R2 (M1f) plus R1/R4 engine work, an
R3 storage answer and acl.yaml (R6). That is several engine milestones, not a
dogfood.
- E. Hybrid: a flow only tracks state and waits, triggers keep creating
checklists. Avoids R1 and R4, but still dies on R2 (no change events, no CLI
step command) and R3.
- C. Manually started shadow flow: runs never advance from CLI edits (R2) and
leave uncommitted state behind (R3).
- D. `transitions:` on the status enums (headless flow, enforced on CLI
writes): would refuse moves agents make today (for example backlog to done for
chores). Possible later as a deliberate change; offered to Jeroen.

**Files to modify:**

- `tickets/tests/checklist-workflow.test.lua` (new)
- `internal/apptest/tickets_coverage_test.go` (new)
- `.github/workflows/ci.yml` (`rela-tickets` job)
- `CLAUDE.md` (unmanaged "Interacting with tickets & docs" section only)
- `docs-project/entities/guides/GUIDE-flows.md` (and regenerated `docs/flows.md`)
- `tickets/entities/...`: this ticket, the DEC, follow-up tickets, relations

**Migration:** none. No property, relation or type changes; existing tickets
(1087) and bugs (295) in every status keep their checklists and dates; no runs
are created.

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:** `tickets/flows.yaml`, `tickets/schema.yaml` and
the new test file are repo config, loaded by the existing validators (`rela flow
validate`, schema load). The test runs in the Lua sandbox (no io, os, debug or
load).

**Security-Sensitive Operations:** none new. No acl, identity, run_as or storage
change.

### Threat model (POLICY-015 §3, PROCEDURE-j5mq step 1)

Assets: integrity of the ticket workflow gates (checklists exist and are done
before status moves on), audit attribution of trigger writes, the live
`tickets/` data. Ticket content is in a public repo; confidentiality is not at
stake.

Actors: agents and Jeroen using the CLI; a PR author changing
`tickets/flows.yaml` or `schema.yaml`; CI.

| Threat (STRIDE) | Vector | Mitigation |
| --- | --- | --- |
| Spoofing | n/a | No identity, principal or run_as change |
| Tampering | A PR drops or breaks a trigger, so checklists silently stop being created | `rela test` in CI fails; the coverage test fails for a trigger without a case |
| Tampering | A PR weakens or deletes the test cases | `--require-tests`, the coverage test, and review |
| Tampering | Status moves past a gate without done checklists (rules are warnings) | Unchanged: `rela validate` in the `rela-tickets` CI job and the three analyze gates; the suite does not claim this |
| Tampering | A binary without flows.yaml support ignores the triggers (R7) | Analyze gates still fail; CLAUDE.md version note. A binary that knows flows.yaml but not `triggers:` fails to boot (strict decode) |
| Repudiation | Trigger writes lose attribution | Unchanged: `triggered_by: automation:<name>`; no trigger is renamed |
| Information disclosure | The test touches live data or the network | `rela test` builds an in-memory world and never opens the live store; sandboxed Lua. `rela flow validate` on the live tree writes only a datamigration baseline into gitignored `tickets/.rela` |
| Denial of service | Slow suite blocks CI | A handful of cases on memstore; seconds |
| Elevation of privilege | A new run_as identity with broad writes | None introduced (option A). Option B would need `system:` run_as plus acl.yaml, recorded in M5f |

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:** (each a `t.case`, asserting checklist type, title `"<Phase>:
<title>"`, status `in-progress`, `started` = today, and the relation from the
ticket/bug)

1. ticket backlog → planning: planning-checklist via `has-planning`;
ticket.started = today.
2. ticket planning → in-progress: implementation-checklist via
`has-implementation`; ticket.started unchanged after `t.advance`.
3. ticket in-progress → review: review-checklist via `has-review`.
4. ticket review → done: ticket.completed = today.
5. ticket backlog → in-progress (planning skipped): started set.
6. bug backlog → analyzing → in-progress → review → done: bug-analysis,
implementation and review checklists; started and completed.
7. bug backlog → in-progress: started set.
8. each checklist type → done: completed = today; docs-checklist →
in-progress: started = today.

**Edge Cases:**

- Re-entering a status (review → in-progress → review) creates no second
checklist (`if_exists: skip`).
- `started` already set is not overwritten (`where: started=`).
- `wont-fix` and `blocked` create nothing and set no `completed`.
- Entering `ready` creates nothing.
- Created directly in a working status (ticket `planning`, bug `in-progress`,
docs-checklist `in-progress`): no checklist and no `started` (R8, pins today's
behaviour so a change to it is deliberate).
- A move that skips a gate (planning → done) is not refused; the case records
that the gates are warnings and documents where they are enforced.

**Negative Tests:** no docs-checklist is auto-created for an enhancement or docs
ticket (it stays manual, matching CLAUDE.md). Acceptance 3 is the mutation check
of the suite itself.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:**

- The suite proves trigger effects only, not the gates (rules are warnings).
Mitigation: stated in scope and threat model; gates stay on `rela validate` in
CI and the analyze gates.
- A new trigger added without a case. Mitigation: the coverage test.
- The brief expects a stateful flow; this plan defers it. Mitigation: stated
up front with evidence; Jeroen decides before phase 2.
- CI cost: one extra build-free step in a job that already builds `rela`.

Effort: s (down from m).

## Documentation Planning

- [x] User-facing docs identified (skip if internal refactor)
- [x] ~~Docs-checklist will be created when entering implementation~~ (N/A: kind is refactor; docs impact is CLAUDE.md, a command file and one guide paragraph, tracked in the implementation checklist)

**Documentation Impact:** CLAUDE.md (binary version and create-in-status notes),
GUIDE-flows Examples paragraph. Managed command files via FT-d.

## Design Review

- [x] Run `/design-review` before starting implementation
- [x] All critical/significant findings addressed in plan

**Design Review Findings:** independent review (cranky-code-reviewer), no
critical findings. Significant: RR-LSYNFLN (R7 misattributed), RR-JKEWK7W (rules
are warnings; suite cannot prove gates), RR-L3KGSTV (property triggers do not
fire on create). Minor/nit: hybrid alternative and R2 as lead blocker, CI
wiring, default principal, repeatable coverage, managed docs, STRIDE rows, M5f
sketch. All addressed in this plan; see the linked review-responses.
