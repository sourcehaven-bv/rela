---
id: REV-35BSHKP
type: review-checklist
title: 'Review: Fix _views path in retired-id redirect docs'
started: "2026-11-14"
completed: "2026-11-14"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] ~~All tests pass (`just test`)~~ (N/A: docs-only change, no Go or frontend code touched)
- [x] ~~Lint clean (`just lint`)~~ (N/A: docs-only change)
- [x] ~~Comment lint gate clean (`just comment-lint`)~~ (N/A: no Go comments touched)
- [x] ~~Coverage maintained (`just coverage-check`)~~ (N/A: docs-only change)

## Code Review

- [x] ~~Run `/code-review` command~~ (N/A: one-word docs fix, self-reviewed against the handler instead)
- [x] ~~All critical review-responses addressed~~ (N/A: no review responses)
- [x] ~~All significant review-responses addressed~~ (N/A: no review responses)
- [x] Self-reviewed the diff for unrelated changes

**Review Responses:** none.

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:** AC1 PASS: `grep -rn "_views/<view>" docs-project docs`
returns nothing (IMPL-FI8K2H7).

## Documentation (enhancements only)

- [x] Docs-checklist created and linked via `has-docs`
- [x] User-facing documentation updated
- [x] Docs-checklist marked as done

**Docs Checklist:** DOCS-75FLYYW

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [x] Run `/pr` command to create PR and monitor CI
