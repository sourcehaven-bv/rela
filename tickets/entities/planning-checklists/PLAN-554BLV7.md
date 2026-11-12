---
id: PLAN-554BLV7
type: planning-checklist
title: 'Planning: archplan weekly: trace batch, reconcile, conflicts and flows'
started: "2026-11-12"
completed: "2026-11-12"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:** In: the four `security.untraced` findings and two worse ratchet
metrics in GitHub issue 2164, plus the security findings the next weekly run
would add from code merged after it (`handleV1Flows`, `gateReadOr`,
`handleV1GetEntity` denial). Out: the `structure.repeated_orchestration` and
`data.orphan_table` findings, which the weekly issue does not count.

**Acceptance Criteria:**
1. `SEQTRACE_SUITE=routes tools/seqtrace/demo/run.sh` then `archplan ci --json` reports no new security finding and no worse ratchet metric (the weekly workflow's close condition).
2. A trace is named after the handler a request entered, never after a closure defined in it.

## Research

- [x] ~~For larger features: run `/research`~~ (N/A: small tooling fix)
- [x] ~~Searched for existing libraries~~ (N/A: change to in-repo tooling)
- [x] Checked codebase for similar patterns or reusable code
- [x] ~~Looked for reference implementations in other projects~~ (N/A: in-repo tooling)
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** N/A

**Existing Solutions:** The routes suite in `tools/seqtrace/demo/run.sh` already
adds config (commands, actions, transforms, realtime) so permission gates run in
the trace; the new flows.yaml and `git:sync` grant follow it. Baseline
acceptances follow the existing `untested_denial` reasons in
`.archplan/baseline.json`.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:** Add requests to the routes suite for batch, reconcile,
conflict detail and resolve (on a conflicted task file), view entries, and
flows, each with a refused case where the route can refuse. Make
`diagram.Call.Title` skip closures of the call it already chose. Accept the
three by-design `untested_denial` findings and re-record the ratchet from the
traced model with `archplan check --loosen`. Rejected: accepting the four
untraced findings without traces (hides real coverage gaps); dropping the
ratchet from the weekly gate (the ratchet is meaningful once recorded from a
traced model, as ci.yml already says).

**Files to modify:** `tools/seqtrace/demo/run.sh`,
`tools/seqtrace/diagram/diagram.go`, `tools/seqtrace/diagram/diagram_test.go`,
`.archplan/baseline.json`.

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] ~~Input validation approach defined~~ (N/A: no new input path; demo script and trace tooling only)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:** Only the demo script's own requests against a
throwaway database. No production code changes.

**Security-Sensitive Operations:** The `git:sync` grant applies only to the demo
copy of the perf project, in the routes suite.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:**
1. Run the routes suite and `archplan ci`; check the new security findings and worse metrics are empty.
2. `TestTitleSkipsHandlerClosures` in `tools/seqtrace/diagram`.

**Edge Cases:** A real handler called from inside a closure still names the
trace (covered by the test).

**Negative Tests:** Refused requests in the suite: conflict detail as editor,
batch as reader, flow start as reader, hidden view entry, reconcile without
identity.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:** A later loosen from a static model drops the ratchet again; ci.yml
already documents the rule. Effort: s.

## Documentation Planning

- [x] ~~User-facing docs identified~~ (N/A: internal tooling)
- [x] ~~Docs-checklist will be created~~ (N/A: chore, no user-facing docs)

**Documentation Impact:** N/A - internal change.

## Design Review

- [x] ~~Run `/design-review` before starting implementation~~ (N/A: tooling-only change, planned and verified against archplan before review; covered by the code review)
- [x] ~~All critical/significant findings addressed in plan~~ (N/A: no design review run)

**Design Review Findings:** N/A
