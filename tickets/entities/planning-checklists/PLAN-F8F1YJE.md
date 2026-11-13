---
id: PLAN-F8F1YJE
type: planning-checklist
title: 'Planning: Flows docs and CLI promise retries that do not exist (#2170)'
started: "2026-11-11"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:**

In: the text that promises an automatic retry of a failed flow step or a
next attempt: GUIDE-flows (status banner, "Running flows", "Doing a
step"), GUIDE-mcp-server (`complete_flow_step`), the `rela flow start`
error, the `complete_flow_step` MCP description, the
`flowview.Completed.Problem` comment and the web app's pending and
settling texts (en, nl).

Out: the worker that calls `Engine.Due` (TKT-FWQZGAM). The `next_attempt`
field stays in the API, unchanged.

**Acceptance Criteria:**

1. `rela flow start` on a failed first step names the run and says to
   retry it in the web app or cancel it (unit test in
   `internal/cli/flow_runs_test.go`).
2. A grep for "retried", "next attempt" and "tries again" in the flows
   docs, CLI, MCP and web app finds no promise of an automatic retry
   (checked by hand in review).

## Research

- [x] ~~For larger features: run `/research`~~ (N/A: docs and message fix)
- [x] ~~Searched for existing libraries~~ (N/A: no code to add)
- [x] Checked codebase for similar patterns or reusable code
- [x] ~~Looked for reference implementations~~ (N/A: docs and message fix)
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** N/A (small change)

**Existing Solutions:** The issue text names the two places; a grep found
the MCP description, the flowview comment, GUIDE-mcp-server and the web app
strings with the same promise.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:** Rewrite the text to describe this release: a
failed step stays pending until someone presses Retry (or calls `_retry`),
or cancels the run. Automatic retries and timers come with the worker of
TKT-FWQZGAM. The web app no longer shows a "next attempt" time, because
nothing acts on it.

Alternative rejected: add the worker here. That is TKT-FWQZGAM, a
separate and larger ticket.

**Files to modify:** `docs-project/entities/guides/GUIDE-flows.md`,
`docs-project/entities/guides/GUIDE-mcp-server.md` and their generated
`docs/` files, `internal/cli/flow_runs.go` and its test,
`internal/mcp/tools_flows.go`, `internal/flowview/flowview.go`,
`frontend/src/components/flows/FlowWaiting.vue`, and the en and nl
locales.

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] ~~Input validation approach defined~~ (N/A: no new input)
- [x] ~~Security-sensitive operations identified~~ (N/A: text only, no behaviour change)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:** None new. The CLI error adds the run id,
which the message already named.

**Security-Sensitive Operations:** None.

**Threat model (POLICY-015 §3, PROCEDURE-j5mq step 1).** Assets: none
new. The change alters text and one error message; it does not change
what a run does, who may retry or cancel it, or what is logged. The one
risk is a wrong instruction: an operator who believes a run is retried
automatically leaves a subject locked by `per_subject: one`. Fixing the
text removes that risk. No new entry points, data flows or trust
boundaries.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] ~~Edge cases identified and documented~~ (N/A: text only)
- [x] Negative test cases defined (invalid input, error conditions)
- [x] ~~Integration test approach defined~~ (N/A: text only; the CLI unit test covers the message)

**Test Scenarios:** Criterion 1: extend `TestFlowRunCmds` to check the
message names `rela flow cancel` and does not say "will be retried".
Criterion 2: grep in review.

**Edge Cases:** None.

**Negative Tests:** The failing start in `TestFlowRunCmds` is the negative
case.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:** The text goes stale when TKT-FWQZGAM lands. Mitigation: a note
on TKT-FWQZGAM to restore the retry text.

## Documentation Planning

- [x] User-facing docs identified (skip if internal refactor)
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:** GUIDE-flows and GUIDE-mcp-server, regenerated
into `docs/`.

## Design Review

- [x] ~~Run `/design-review` before starting implementation~~ (N/A: a text and message change with no design; the code review covers it)
- [x] ~~All critical/significant findings addressed in plan~~ (N/A: no design review)

**Design Review Findings:** N/A
