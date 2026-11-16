---
id: REV-TPSI701
type: review-checklist
title: 'Review: One step at a time over a set of entities'
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
- [x] All significant review-responses addressed: RR-MMESVPT fixed
- [x] Self-reviewed the diff for unrelated changes

**Review Responses:** RR-MMESVPT (addressed), RR-U2ESX0P (addressed), RR-X5SKUT3
(addressed), RR-B849JMC (wont-fix), RR-P82SE6I (wont-fix)

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:**

- PASS: a `walk: true` list step shows one row with "N of M" and previous and next row links (e2e).
- PASS: the row is in the URL as `?item=`; a step opened without it opens the first row (e2e, Vitest).
- PASS: PageDown and PageUp walk the rows first, then the steps (e2e, Vitest).
- PASS: rows come from the gated list API; `/_position` applies the list's condition through `list_id` (scope_listid_test.go).

## Documentation (enhancements only)

- [x] Docs-checklist created and linked via `has-docs`
- [x] User-facing documentation updated: GUIDE-data-entry "One row at a time"
- [x] Docs-checklist marked as done

**Docs Checklist:** DOCS-48IRUBM

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [x] Run `/pr` command to create PR and monitor CI (done after this checklist, as the brief describes)
