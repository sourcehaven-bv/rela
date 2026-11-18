---
id: PLAN-23K5LAB
type: planning-checklist
title: 'Planning: Flow tasks as a next-action source'
started: "2026-11-18"
completed: "2026-11-18"
status: done
---

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:** In: `source: flow_tasks` with `flows:` filter; message placeholders
`{flow}`, `{step}`, `{subject}`; navigate link with `?run=`; Flows panel focuses
the run; oldest-task pick; config validation; docs. Out: other built-in sources;
a `pick:` policy for other sources; editing harbourmaster.

**Acceptance Criteria:**
1. A reader with a waiting task sees "Complete {step} for {subject}" (e2e flow-task-hint.spec.ts).
2. The button opens the subject page with the run card focused (e2e).
3. Snooze hides the hint, the task stays in the panel (e2e, unit).
4. A completed task is gone on the next resolve (e2e, unit).
5. Candidates equal the Flow tasks list (wiring uses FlowService.Tasks; unit test checks the filter calls).

## Research

- [x] ~~For larger features: run `/research` to create a structured research doc~~ (N/A: extends an existing engine with one source kind)
- [x] ~~Searched for existing libraries that solve this problem~~ (N/A: internal engine)
- [x] Checked codebase for similar patterns or reusable code
- [x] ~~Looked for reference implementations in other projects~~ (N/A: internal engine)
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** N/A

**Existing Solutions:**
- `nextActionCandidates` query and context sources (internal/dataentry/nextaction.go).
- `FlowControl.Tasks` behind the Flow tasks page (internal/appbuild).
- `scopedEntities` ByIDs gated batch read and `redactedForSuggestion`.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:** Config gains `source` and `flows`. The dataentry wiring
turns `FlowService.Tasks` (one call per named flow) into candidates whose entity
is the redacted subject, read in one gated batch per subject type. The engine
keys each candidate by subject plus run/state/branch variant, fills the message
from task values, fills navigate offers, adds a default link, exempts the source
from the impression cooldown unless declared, and picks the oldest eligible
task. Alternative rejected: a query source over `_flow-run` entities, since it
would re-implement the task rules (roles via role relations) outside the task
call.

**Files to modify:** internal/dataentryconfig/nextaction*.go,
internal/nextaction/nextaction.go, internal/dataentry/nextaction*.go,
frontend/src/components/flows/FlowRunsPanel.vue, focusRun.ts, docs guides, e2e
spec.

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:** `data-entry.yaml` (operator config): `source`
allowlisted to `flow_tasks`, `flows` non-empty and unique; undeclared flow names
logged and skipped. Query `?run=` is only used to find a DOM element.

**Security-Sensitive Operations:** Reads only. Tasks come from the ACL-gated
task call; subjects through the read gate and field redaction; a subject the
reader cannot read is named by its address only, which the task list already
shows. Without identity the source contributes nothing.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:** Unit: internal/nextaction/flowtasks_test.go,
internal/dataentry/nextaction_flowtasks_test.go,
internal/dataentryconfig/nextaction_flowtasks_test.go, frontend
focusRun.test.ts. E2E: e2e/tests/flow-task-hint.spec.ts.

**Edge Cases:** No flows wired; no identity; unread subject; run moves to a new
state; on-screen suggestion kept; declared cooldown; source defer scope.

**Negative Tests:** Config: unknown source, flows without source,
empty/duplicate flows, condition/key_props/source_world/pick/pick_one rejected.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:** Cost per resolve: one task call per named flow plus one batch read
per subject type, pinned by a budget test. Overlap with the flow UI PR in
FlowRunsPanel.vue, kept to a small hook.

## Documentation Planning

- [x] User-facing docs identified (skip if internal refactor)
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:**
- [x] docs/data-entry.md - next-action source
- [x] docs/flows.md - pointer from Doing a step

## Design Review

- [x] ~~Run `/design-review` before starting implementation~~ (N/A: small extension of an existing engine; reviewed by cranky-code-reviewer and rela-security-reviewer at review)
- [x] ~~All critical/significant findings addressed in plan~~ (N/A: no design review run)

**Design Review Findings:** N/A
