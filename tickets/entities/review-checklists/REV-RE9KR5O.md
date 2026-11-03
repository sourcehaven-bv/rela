---
id: REV-RE9KR5O
type: review-checklist
title: 'Review: Inbox: answer waiting sessions from the inbox row'
started: "2026-11-03"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`just test`, in `just ci`; postgres DB-gated tests with `just test-postgres` on local Postgres.app)
- [x] Lint clean (`just lint`: 0 issues; arch-lint and plimsoll clean)
- [x] Comment lint gate clean (`just comment-lint`)
- [x] Coverage maintained (`just coverage-check`; `internal/channels` fell to 87.0% and is back at 88.6% with `getmessages_test.go`)

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

- [x] Run `/code-review` command (invokes cranky-code-reviewer agent; rela-security-reviewer run in parallel)
- [x] All critical review-responses addressed (none)
- [x] All significant review-responses addressed (RR-69432BJ, RR-U0VGS54)
- [x] Self-reviewed the diff for unrelated changes

**Review Responses:** RR-69432BJ, RR-U0VGS54 (significant, addressed);
RR-E57RFMM (minor, wont-fix), RR-YVYZ6SK, RR-XPFJAAM, RR-IZRK71O, RR-O7UQ4B4,
RR-NVVALGM (minor, addressed); RR-CJAYKCA (nit, includes the security finding),
RR-HBV70HG, RR-XUL7ROY (nit, addressed).

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist (IMPL-SJYXHEE)

**Acceptance Status:**

1. Cause is stored: PASS (`SessionCause`, `SessionCauseKept` conformance on file, sqlite, postgres; migrations tested).
2. Batch message read: PASS (`GetMessages*` conformance; damaged rows; `TestGetMessages_Canceled`).
3. Entries carry the waiting message: PASS (`TestInbox_WaitingMessage`, `TestInbox_WaitingWithoutCause`, `TestInbox_WaitingAfterTimerMove`).
4. Decision inputs match the head: PASS (`TestInbox_DecisionInputsMatchHead`, `TestInbox_DecisionInputs`, `TestInbox_DecisionInputsWithoutPost`).
5. Nothing hidden leaks: PASS (`TestInbox_WaitingMessageHiddenRows`, `TestInbox_WaitingMessagesHiddenRowsRealACL` byte compare).
6. Cost does not grow with rows: PASS (`TestInbox_WaitingMessagesBudget`, `TestInboxBudget_WaitingMessages`).
7. Live round within bound: PASS (file 2.37 s at fill 700, grid 4 s; postgres 31 ms at fill 50).
8. One-click from the row: PASS (`InboxWaitingItem.test.ts`, e2e).
9. Choices from the row: PASS (`InboxWaitingItem.test.ts`, e2e).
10. Form decisions open the channel: PASS (`OpenChannel.start.test.ts`, `InboxView.test.ts`, e2e).
11. Refusals: PASS (`TestWaitingMessages_Refusals`, `InboxView.test.ts`).
12. No buttons without inputs: PASS (`InboxWaitingItem.test.ts`, `TestInbox_DecisionInputsWithoutPost`).
13. End to end: PASS (e2e "the owner answers questions and approves a plan from the inbox").

## Documentation (enhancements only)

Skip this section for bugs and internal refactors.

- [x] Docs-checklist created and linked via `has-docs`
- [x] User-facing documentation updated
- [x] Docs-checklist marked as done

**Docs Checklist:** DOCS-PPFC30E

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [x] Run `/pr` command to create PR and monitor CI (next step, after done)

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
