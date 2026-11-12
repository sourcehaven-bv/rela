---
id: REV-W2GBXIZ
type: review-checklist
title: 'Review: Flows M4b: job steps'
started: "2026-11-12"
completed: "2026-11-12"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`just test`)
- [x] Lint clean (`just lint`)
- [x] Comment lint gate clean (`just comment-lint`)
- [x] Coverage maintained (`just coverage-check`)

Run: `go test ./...`; `go test -race` on flowengine, flowcfg, flowrun (all
backends), lua, script, flowview, appbuild and dataentry; the appbuild flow
tests with `-tags sqlite`; golangci-lint on the changed packages (0 issues);
`just arch-lint`, `just comment-lint`, `just plimsoll` clean. No new package, so
no coverage floor changes; the new code has unit and end-to-end tests.
Postgres-tagged wiring tests skip locally and run in CI.

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

**Review Responses:** cranky-code-reviewer: RR-1071W0F, RR-BPECRH2, RR-O0ITDM5
(significant, addressed), RR-BVDRBW4, RR-AC7G2ZR (minor, addressed).
rela-security-reviewer: RR-R16S6VZ (minor, addressed), RR-7Z1EQOO (nit,
wont-fix). Design review: RR-48HQ9T9, RR-5L41TIL, RR-K3DMOQ1 (critical),
RR-0JQWN6W, RR-62RAH37, RR-GZ335ZV, RR-H3Z72TJ, RR-IED8VHF, RR-T77TZ2O
(significant), RR-6X3AUZN, RR-BACHJMD, RR-UQ615NV (minor), all addressed;
RR-QDPDTMV (significant) deferred to TKT-GYXOTQC.

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:**

- Exhausted retries, then operator retry with a fresh count: PASS
(TestJob_ExhaustsItsAttemptsThenOperatorRetryStartsAFreshCount).
- Permanent error: PASS (TestJob_PermanentErrorFailsAtOnce,
TestJob_RetryFalseFailsOnTheFirstError, TestFlowJob_WebhookClientErrorFails,
TestBuildScriptError_RetryFalseIsPermanent).
- Timeout: PASS (TestJob_TimedOutAttempt, TestJob_TimeoutCountsFromTheStart).
- Lost worker: PASS (TestJob_LostWorker).
- Idempotent: PASS (the idempotent subtests of the timeout and lost tests).
- Idempotency key: PASS (TestJob_IdempotencyKeys,
TestFlowJob_WebhookSignsIDsOnly).
- Never-started attempt does not use the budget: PASS
(TestJob_AttemptThatNeverStartedIsQueuedAgainWithoutUsingTheBudget).
- Script, action and webhook targets as run_as: PASS (TestFlowJob_*).

## Documentation (enhancements only)

Skip this section for bugs and internal refactors.

- [x] Docs-checklist created and linked via `has-docs`
- [x] User-facing documentation updated
- [x] Docs-checklist marked as done

**Docs Checklist:** DOCS-6VSU7BY

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
