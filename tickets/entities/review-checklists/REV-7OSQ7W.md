---
id: REV-7OSQ7W
type: review-checklist
title: 'Review: Scheduler stalls: durable jobs over 30s never complete and block their idempotency key'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`just test`)
- [x] Lint clean (`just lint`)
- [x] Comment lint gate clean (`just comment-lint`)
- [x] Coverage maintained (`just coverage-check`)

**Comment findings.** `just comment-report` lists the advisory rules
(duplication, nil-contract, param-contract, restatement). They are not a merge
gate, but a finding your diff *introduces* should be fixed or suppressed — don't
grow the backlog.

Every rule is a heuristic over prose, so false positives are expected. To
suppress one, prefer the inline form on the declaration line, which travels with
the code and is reviewed in this diff:

```go
func f(p string) {} //commentlint:ignore param-contract  p is contained by Clone
```

Use `.commentlint.yml` (`ignore:` path globs, `allow-phrases:`) only when the
same prose recurs across many sites. A reason is required either way — an
unexplained suppression is a finding nobody can re-evaluate later.

## Code Review

- [x] Run `/code-review` command (invokes cranky-code-reviewer agent)
- [x] All critical review-responses addressed
- [x] All significant review-responses addressed
- [x] Self-reviewed the diff for unrelated changes

**Review Responses:** Security review: no findings. Code review: RR-W90XOO,
RR-5VZWU7, RR-3WK5WQ, RR-25450U (significant, addressed); RR-5HF18L,
RR-2HJXEF, RR-5Z2VPA, RR-AEW5FM, RR-1TWZ95, RR-T3U8C1 (minor, addressed);
RR-BCBX0O (minor, wont-fix); RR-C6V3AB (minor, deferred); RR-3L3OU7 (nit,
addressed).

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:**

- Scheduler enqueues and continues without waiting: PASS
(TestTick_NeverWaitsAndSkipsWhileActive).
- Durable, queryable run state: PASS (schedulerstatetest on kvstate and
pgschedstate).
- Idempotent run creation across processes: PASS (ConcurrentCreateAdmitsOne,
TestTick_TwoSchedulersShareOneStore).
- Recovery after worker or process failure: PASS
(TestRun_LostRunIsAbandonedAndRetried, ReapAbandonsOnlyExpiredRuns).
- Missed-run detection and retry ladder preserved: PASS (TestTick_DueDecisions,
TestRun_FailureAdvancesLadder, TestRun_RetryLadderReplacesSchedule).
- Long durable job completes once: PASS
(TestPostgresQueue_HandlerOutlivesDefaultIdleTxTimeout).

## Documentation (enhancements only)

Skip this section for bugs and internal refactors.

- [x] ~~Docs-checklist created and linked via `has-docs`~~ (N/A: bug fix; scheduled-tasks and postgres-backend docs updated in the diff)
- [x] ~~User-facing documentation updated~~ (N/A: bug fix; docs updated in the diff anyway)
- [x] ~~Docs-checklist marked as done~~ (N/A: bug fix)

**Docs Checklist:** <!-- e.g., DOCS-xxxx -->

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [x] Run `/pr` command to create PR and monitor CI

<!--
Deliberately NOT tracked here: the PR URL and whether CI passed.

Both post-date this checklist. `/pr` requires the ticket to be `done` and
validating clean before it opens the PR, and a `done` review-checklist may have
no unchecked items — so an item asking for the PR URL can only be satisfied by a
PR that does not exist yet. Checking it early would mean asserting "CI passed"
before CI ran, which turns the checklist from evidence into a formality.

GitHub records both authoritatively, and the branch and commit messages carry
the ticket ID, so the ticket-to-PR link is recoverable without duplicating it
here. See TKT-UFV01M. -->
