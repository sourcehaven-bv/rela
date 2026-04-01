---
id: REV-TU5R
type: review-checklist
title: 'Review: Add checklist validation for markdown content'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`just test`)
- [x] Lint clean (`just lint`)
- [x] Coverage maintained (`just coverage-check`)

## Code Review

- [x] Run `/code-review` command (invokes cranky-code-reviewer agent)
- [x] All critical review-responses addressed
- [x] All significant review-responses addressed
- [x] Self-reviewed the diff for unrelated changes

**Review Responses:** RR-4Z4P (wont-fix: out of scope), RR-5ML6 (wont-fix: intentional behavior), RR-F1V3 (addressed), RR-A3YJ (addressed: removed unnecessary parser context)

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:**

1. Define checklist validation rule: **PASS** - Test metamodel loads with `content.checklist.all-checked`
2. Count checkboxes correctly: **PASS** - Unit tests verify extraction
3. All-checked validation: **PASS** - TSK-001 fails, TSK-002 passes
4. Allow-skipped option: **PASS** - TSK-004 with strikethrough passes
5. Integration with analyze_validations: **PASS** - Output shows violations correctly
6. Works with when conditions: **PASS** - TSK-003 (pending) not flagged

## Documentation (enhancements only)

Skip this section for bugs and internal refactors.

- [x] ~~Docs-checklist created and linked via `has-docs`~~ (N/A: internal feature, no user-facing docs needed yet)
- [x] ~~User-facing documentation updated~~ (N/A: will document when feature is used)
- [x] ~~Docs-checklist marked as done~~ (N/A)

**Docs Checklist:** N/A - Documentation will be added when the feature is used in a metamodel

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [x] Run `/pr` command to create PR and monitor CI
- [x] All CI checks pass
- [x] PR URL documented below

**PR:** https://github.com/sourcehaven-bv/rela/pull/267
