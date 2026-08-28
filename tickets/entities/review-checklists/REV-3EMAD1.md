---
id: REV-3EMAD1
type: review-checklist
title: 'Review: Case-variant entity IDs collide in fsstore but coexist in memstore/pgstore'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`just test`)
- [x] Lint clean (`just lint`)
- [x] Coverage maintained (`just coverage-check`)

## Code Review

- [x] ~~Run `/code-review` command (invokes cranky-code-reviewer agent)~~ (N/A: human review on the PR)
- [x] ~~All critical review-responses addressed~~ (N/A: none raised)
- [x] ~~All significant review-responses addressed~~ (N/A: none raised)
- [x] Self-reviewed the diff for unrelated changes

**Review Responses:** None raised; originated as an IB-review finding on PR #1272 (tschmits).

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:**
<!-- For each acceptance criterion, state PASS/FAIL with evidence -->

## Documentation (enhancements only)

Skip this section for bugs and internal refactors.

- [x] ~~Docs-checklist created and linked via `has-docs`~~ (N/A: bug fix, no user-facing docs)
- [x] ~~User-facing documentation updated~~ (N/A: rationale documented in storeutil.FoldID godoc and migration 0007)
- [x] ~~Docs-checklist marked as done~~ (N/A: none created)

**Docs Checklist:** N/A — bug fix.

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [x] Run `/pr` command to create PR and monitor CI
- [x] All CI checks pass
- [x] PR URL documented below

**PR:** <!-- e.g., https://github.com/org/repo/pull/123 -->
