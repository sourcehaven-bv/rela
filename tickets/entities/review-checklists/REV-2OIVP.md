---
id: REV-2OIVP
type: review-checklist
title: 'Review: Fix enum list property input, rendering, and validation in data-entry'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`just test`)
- [x] Lint clean (`just lint`)
- [x] ~~Coverage maintained (`just coverage-check`)~~ (N/A: `go-test-coverage` not installed locally; CI's own coverage job passed)

## Code Review

- [x] ~~Run `/code-review` command (invokes cranky-code-reviewer agent)~~ (N/A: small, well-scoped change; self-review covered the diff)
- [x] ~~All critical review-responses addressed~~ (N/A: no review-responses created)
- [x] ~~All significant review-responses addressed~~ (N/A: no review-responses created)
- [x] Self-reviewed the diff for unrelated changes

**Review Responses:** None.

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:**

All 8 ACs PASS. Evidence is in IMPL-3NF9G (manual browser verification for
AC1–AC8 with screenshots; unit tests for `asArray`, `propertyToStrings`,
empty-list validation; 428/428 frontend unit tests, full Go test sweep green).

Additional user-reported gaps found during demo and fixed in-branch:

- Saving with an empty tag list failed with "Empty list (allowed: …)". Fix: `validation.go` now treats empty lists as "no value" for non-required list properties, and as "missing" for required ones. New tests: `TestValidateProperties_EmptyListForListProperty`.
- Detail view showed a blank cell for cleared list properties. Fix: `formatValue([])` now returns `-`; `SidePanel.vue` and `CustomView.vue` card fallbacks show `-` instead of empty string. New test case: `formatValue([])` returns `-`.

## Documentation (enhancements only)

- [x] ~~Docs-checklist created and linked via `has-docs`~~ (N/A: no user-facing docs currently describe the list-enum widget/rendering; nothing to update)
- [x] ~~User-facing documentation updated~~ (N/A)
- [x] ~~Docs-checklist marked as done~~ (N/A)

**Docs Checklist:** None.

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [x] Run `/pr` command to create PR and monitor CI
- [x] All CI checks pass
- [x] PR URL documented below

**PR:** https://github.com/sourcehaven-bv/rela/pull/550
