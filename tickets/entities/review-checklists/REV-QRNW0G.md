---
id: REV-QRNW0G
type: review-checklist
title: 'Review: Cardinality analysis: batch relation counts and count visible edges only'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`just test`)
- [x] Lint clean (`just lint`)
- [x] Comment lint gate clean (`just comment-lint`)
- [x] Coverage maintained (`just coverage-check`)

## Code Review

- [x] Run `/code-review` command (invokes cranky-code-reviewer agent)
- [x] All critical review-responses addressed
- [x] All significant review-responses addressed
- [x] Self-reviewed the diff for unrelated changes

**Review Responses:** RR-YHKWB8, RR-7A1C52, RR-6UZ85W (significant,
addressed); RR-01AH5L, RR-8TM8XD, RR-7QQPYC, RR-N22CGX (addressed);
RR-X873OB, RR-W2RINP (deferred); RR-JH5WPS, RR-F1FYU1 (wont-fix).

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:**

- AC1 PASS: `TestCheckCardinality_ReadBudget` (raw) and
  `TestScriptReader_CardinalityReadBudget` (gated) read the same number of
  times at 10 and 50 subjects.
- AC2 PASS: data-entry and MCP visible-edge tests; a gate fault fails the
  check (`TestScriptReader_ListRelationsStrictReturnsGateFaults`,
  `TestAnalyzeCardinality_GateFaultReportsOneIssue`).
- AC3 PASS: the CLI runs `schema.Ungated(store)` through the same checker.

## Documentation (enhancements only)

- [x] Docs-checklist created and linked via `has-docs`
- [x] User-facing documentation updated
- [x] Docs-checklist marked as done

**Docs Checklist:** DOCS-UGZLQ8

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [x] ~~Run `/pr` command to create PR and monitor CI~~ (N/A: PR opened directly against faces-intrinsic as the coordinator directed)
