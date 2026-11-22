---
id: REV-Y67F6XQ
type: review-checklist
title: 'Review: List title link colour without link:, nested and card sections on phones'
started: "2026-11-22"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (focused vitest and vue-tsc; the full suite runs in CI)
- [x] Lint clean (eslint on the changed files)
- [x] ~~Comment lint gate clean~~ (N/A: no Go changes)
- [x] ~~Coverage maintained~~ (N/A locally: CI checks the floors)

## Code Review

- [x] Run `/code-review` command (invokes cranky-code-reviewer agent)
- [x] All critical review-responses addressed (none)
- [x] All significant review-responses addressed (RR-7EDF7L3, RR-34R6C73)
- [x] Self-reviewed the diff for unrelated changes

**Review Responses:** RR-7EDF7L3, RR-34R6C73, RR-OUUY5HC, RR-NBS2322, RR-UM9098S, RR-VTT09RI

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:**
- Title link colour without link: PASS (EntityList.titleLink.test.ts)
- Nested section on a phone: PASS (browser check at 375px)
- Cards fit a narrow section: PASS (CSS min(); browser check)
- Wider sections unchanged: PASS (browser check on desktop)
- Documented: PASS (docs/data-entry.md and GUIDE-data-entry)

## Documentation (enhancements only)

- [x] ~~Docs-checklist created and linked via `has-docs`~~ (N/A: bug)
- [x] User-facing documentation updated (docs/data-entry.md and GUIDE-data-entry)
- [x] ~~Docs-checklist marked as done~~ (N/A: bug)

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [x] PR created with `gh pr create` after the bug is done; CI watched there
