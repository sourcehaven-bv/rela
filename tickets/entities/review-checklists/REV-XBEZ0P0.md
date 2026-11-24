---
id: REV-XBEZ0P0
type: review-checklist
title: 'Review: Export items in the list more menu render in capitals'
started: "2026-11-23"
completed: "2026-11-23"
status: done
---

## Automated Checks

- [x] Frontend type-check, ESLint and ExportMenu tests pass; CI runs the rest
- [x] ~~Lint clean (`just lint`)~~ (N/A: frontend-only change, ESLint run)
- [x] ~~Comment lint gate clean (`just comment-lint`)~~ (N/A: no Go comments changed)
- [x] ~~Coverage maintained (`just coverage-check`)~~ (N/A: no Go change)

## Code Review

- [x] ~~Run `/code-review` command~~ (N/A: one CSS class and its test)
- [x] ~~All critical review-responses addressed~~ (N/A: none)
- [x] ~~All significant review-responses addressed~~ (N/A: none)
- [x] Self-reviewed the diff for unrelated changes

## Acceptance Verification

- [x] Each acceptance criterion tested: the more-mode item has no uppercase class
- [x] Test evidence documented: ExportMenu.test.ts

## Documentation (enhancements only)

- [x] ~~Docs-checklist created and linked via `has-docs`~~ (N/A: bug)
- [x] ~~User-facing documentation updated~~ (N/A: bug)
- [x] ~~Docs-checklist marked as done~~ (N/A: bug)

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [x] Run `/pr` command to create PR and monitor CI
