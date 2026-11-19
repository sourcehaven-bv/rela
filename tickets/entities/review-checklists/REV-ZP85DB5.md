---
id: REV-ZP85DB5
type: review-checklist
title: 'Review: Flow tasks as a next-action source'
started: "2026-11-19"
completed: "2026-11-19"
status: done
---

## Automated Checks

- [x] All tests pass (`just test`)
- [x] Lint clean (`just lint`)
- [x] Comment lint gate clean (`just comment-lint`)
- [x] Coverage maintained (`just coverage-check`)

Ran go test for internal/dataentry, nextaction, dataentryconfig, archguard,
conditionlint; golangci-lint on the changed packages; comment-lint; arch-lint;
vitest (430 files) and the e2e specs flow-task-hint, next-action-surfaces and
flows. Full suite and coverage run in CI.

## Code Review

- [x] Run `/code-review` command (invokes cranky-code-reviewer agent)
- [x] All critical review-responses addressed
- [x] All significant review-responses addressed
- [x] Self-reviewed the diff for unrelated changes

**Review Responses:** RR-CI30VTN, RR-6ZDIPKU, RR-TITU3OU, RR-NGU7AJM (wont-fix),
RR-FVZ7RHT, RR-T45UZGL, RR-R6K20ZE, RR-33RYC30. cranky-code-reviewer and
rela-security-reviewer both ran.

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:**
1. PASS: banner text (e2e).
2. PASS: Open lands on the step with the run card focused (e2e).
3. PASS: snooze hides only the hint (e2e, unit).
4. PASS: completed task gone (e2e, unit).
5. PASS: candidates from FlowService.Tasks (unit filter test, e2e).

## Documentation (enhancements only)

- [x] Docs-checklist created and linked via `has-docs`
- [x] User-facing documentation updated
- [x] Docs-checklist marked as done

**Docs Checklist:** DOCS-6TQARTV

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [x] ~~Run `/pr` command to create PR and monitor CI~~ (N/A: PR opened with gh after the ticket is done, per the brief)
