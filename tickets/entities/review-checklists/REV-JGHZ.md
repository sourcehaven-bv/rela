---
id: REV-JGHZ
type: review-checklist
title: 'Review: CLI validation command for CI integration'
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

**Review Responses:** RR-TVBX (addressed), RR-FYP2 (addressed), RR-D2IY
(deferred), RR-YGW5 (deferred), RR-YYMA (wont-fix), RR-H6RI (wont-fix), RR-383J
(wont-fix)

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:**

1. Add `--check` flag to validate command - PASS (tested via unit tests and
   manual verification in IMPL-62IM)
2. Support cardinality, properties, validations, all check types - PASS
3. Filter validations by rule name - PASS (`rela validate --check
   validations:rule-name`)
4. Filter validations by entity type - PASS (`rela validate --check
   validations:@ticket`)
5. Multiple filters work as union - PASS (multiple --check flags supported)
6. Exit code 1 on errors - PASS (tested with errors.NewExitError)
7. JSON output with -o json - PASS (tested in TestRunValidationChecks_JSONOutput)
8. Quiet mode with -q - PASS (progress messages suppressed)

## Documentation (enhancements only)

Skip this section for bugs and internal refactors.

- [x] ~~Docs-checklist created and linked via `has-docs`~~ (N/A: CLI enhancement
  with self-documenting --help flag, no external docs needed)
- [x] ~~User-facing documentation updated~~ (N/A: usage documented in command
  help text)
- [x] ~~Docs-checklist marked as done~~ (N/A: see above)

**Docs Checklist:** N/A - command help is self-documenting

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [x] Run `/pr` command to create PR and monitor CI
- [x] All CI checks pass
- [x] PR URL documented below

**PR:** https://github.com/sourcehaven-bv/rela/pull/273
