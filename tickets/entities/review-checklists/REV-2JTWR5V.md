---
id: REV-2JTWR5V
type: review-checklist
title: 'Review: Flows M1e: flow worker'
started: "2026-11-11"
completed: "2026-11-11"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`just test`): `just coverage-check` runs the full suite; race runs of appbuild, flowengine, flowrun, cli, mcp, flowview, cmd and pgstore pass; the flow worker, flowengine and keyed-lock tests pass on the sqlite and postgres builds
- [x] Lint clean (`just lint`): golangci-lint on the changed packages with the default, sqlite and postgres tags, run one at a time; `just arch-lint` and `just plimsoll` clean
- [x] Comment lint gate clean (`just comment-lint`): no new `just comment-report` findings in the changed files
- [x] Coverage maintained (`just coverage-check`)

## Code Review

- [x] Run `/code-review` command (invokes cranky-code-reviewer agent), together with the rela-security-reviewer agent
- [x] All critical review-responses addressed (none)
- [x] All significant review-responses addressed: RR-EX1C4PF, RR-0QSRC19, RR-P7AY0NY and RR-T42SFAQ are addressed; design RR-RGJ2GZJ is deferred to TKT-ZOU5BCT, with the reason recorded and the tenant placement put to the user
- [x] Self-reviewed the diff for unrelated changes

**Review Responses:** Code and security review: RR-EX1C4PF, RR-0QSRC19,
RR-P7AY0NY, RR-T42SFAQ, RR-76GV4W9, RR-3O6UE04, RR-3GG3S96, RR-DJ6VY2Q,
RR-F6E43VE, RR-K31DAOU and RR-W9BOQP3 are addressed. RR-I3GBB1K, RR-SC3JL4Q and
RR-E21QI0X are deferred to TKT-7VAHS94, and RR-C7I04SJ to TKT-8137DHO.
RR-PWU8P8O and RR-IJ87FIW are wont-fix. The design review RRs are on the
planning checklist.

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:**

1. PASS: `TestFlowWorker_TimerEscalates` passes on fs, sqlite and postgres, and the timer escalated in the manual check.
2. PASS: `TestFlowWorker_RecoversPendingOutput` checks that the output is written once. `TestDue_CreateOutlivesItsCaller` and `TestDue_TimedOutCreateIsNotWrittenAgain` cover cut creates.
3. PASS: `TestFlowWorker_JoinWhenBecomesTrue` and the flowengine join poll tests.
4. PASS: `TestFlowWorker_OneLeader` passes on all three backends, `OnlyTheLeaderRuns` passes, and in the manual check a second server did not lead.
5. PASS: `TestFlowWorker_TickCap`, `NextTickResumes`, `CutRoundResumes` and `TestDueRound_CapAndCursor`.
6. PASS: `TestFlowWorker_OnlyLongRunningHosts`.

## Documentation (enhancements only)

- [x] Docs-checklist created and linked via `has-docs`
- [x] User-facing documentation updated
- [x] Docs-checklist marked as done

**Docs Checklist:** DOCS-CLEGA1J

## Final Checks

- [x] Commit message explains the why, not just what: minimal message naming the ticket, per the user's rule
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [x] ~~Run `/pr` command to create PR and monitor CI~~ (N/A: the PR is opened by hand per the task brief, after the ticket is done)
