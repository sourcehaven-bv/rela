---
id: REV-SUHI3LL
type: review-checklist
title: 'Review: done_when per step'
started: "2026-11-15"
completed: "2026-11-15"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass: `go test` on dataentry, dataentryconfig and conditionlint; Vitest `PageView.test.ts` (41); `pages.spec.ts` e2e (11). The full `just test` runs in CI.
- [x] Lint clean: golangci-lint on the changed packages (0 issues), `just arch-lint`, `vue-tsc -b`, eslint and prettier on changed frontend files
- [x] ~~Comment lint gate clean (`just comment-lint`)~~ (N/A locally: the brief limits local runs to targeted checks; CI runs the gate)
- [x] ~~Coverage maintained (`just coverage-check`)~~ (N/A locally: the brief forbids `just coverage-check`; CI runs it)

## Code Review

- [x] Run `/code-review` command (invokes cranky-code-reviewer agent)
- [x] All critical review-responses addressed (none raised)
- [x] All significant review-responses addressed: RR-69MGJNZ fixed
- [x] Self-reviewed the diff for unrelated changes

**Review Responses:** RR-69MGJNZ (addressed), RR-C50M208 (deferred), RR-YY5ABNW
(addressed), RR-E4ZC2XX (addressed), RR-Y4P4KL4 (addressed)

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:**

- PASS: `done_when` compiles as conditionlint kind `page_done`; a bad condition fails the load (viewcondition_test.go).
- PASS: a done step shows a check on its number and Next stays enabled (e2e, Vitest).
- PASS: judged per principal over the gated rows; hidden rows do not count (page_status_test.go).
- PASS: judged in the requested world (TestPageStatus_JudgesTheWorldsRows).
- PASS: no per-row lookups (page_status_budget_test.go).

## Documentation (enhancements only)

- [x] Docs-checklist created and linked via `has-docs`
- [x] User-facing documentation updated: GUIDE-data-entry "Marking a step done"
- [x] Docs-checklist marked as done

**Docs Checklist:** DOCS-0IHCDSP

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [x] Run `/pr` command to create PR and monitor CI (done after this checklist, as the brief describes)
