---
id: REV-M8B627F
type: review-checklist
title: 'Review: Portal UX fixes: date picker and mobile menu bar and select-all label and phone list columns'
started: "2026-11-20"
completed: "2026-11-20"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`just test`): go test for dataentryconfig, dataentry and apiwire; full `npm run test:run` (445 files); e2e mobile-layout and space-landing-builtins specs (16 passed)
- [x] Lint clean (`just lint`): golangci-lint on changed packages, `npm run lint`, `npm run typecheck`, `just arch-lint`
- [x] Comment lint gate clean (`just comment-lint`)
- [x] ~~Coverage maintained (`just coverage-check`)~~ (N/A locally: new code is covered by unit tests; CI runs the coverage floors)

## Code Review

- [x] ~~Run `/code-review` command (invokes cranky-code-reviewer agent)~~ (N/A: this worker could not spawn agents; a cranky-style self-review of the full diff was done instead and recorded below)
- [x] All critical review-responses addressed
- [x] All significant review-responses addressed
- [x] Self-reviewed the diff for unrelated changes

**Review Responses:** RR-5KBP10S, RR-IYUNBS1, RR-OE0GFHA, RR-25LC3OM, RR-5FRQ6Y1

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:** 1 PASS (handler and Sidebar tests); 2 PASS (AsOfPicker
tests, 1400px check); 3 PASS (375px checks, mobile-layout e2e); 4 PASS
(selectAllLabel test, 375px check); 5 PASS (EntityList and dataentryconfig
tests, 375px invoice list).

## Documentation (enhancements only)

- [x] Docs-checklist created and linked via `has-docs`
- [x] User-facing documentation updated
- [x] Docs-checklist marked as done

**Docs Checklist:** DOCS-HXVS1EZ

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [x] ~~Run `/pr` command to create PR and monitor CI~~ (N/A here: the PR is opened after done, per TKT-UFV01M)
