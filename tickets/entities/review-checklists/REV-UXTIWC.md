---
id: REV-UXTIWC
type: review-checklist
title: 'Review: Filter _analyze results through the ACL read gate (TKT-VQGN follow-through)'
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

**Review Responses:** <!-- List IDs of review-response entities created, e.g.,
RR-xxxx -->

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:**
<!-- For each acceptance criterion, state PASS/FAIL with evidence -->

## Documentation (enhancements only)

Skip this section for bugs and internal refactors.

- [x] Docs-checklist created and linked via `has-docs`
- [x] User-facing documentation updated
- [x] Docs-checklist marked as done

**Docs Checklist:** <!-- e.g., DOCS-xxxx -->

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [x] Run `/pr` command to create PR and monitor CI
- [x] All CI checks pass
- [x] PR URL documented below

**PR:** <!-- e.g., https://github.com/org/repo/pull/123 -->

---
**Speed-run note:** kind=refactor, effort=m (the substantive one). New helper
visibleAnalysisIssues filters issues via batched PermitsReadMany (mirrors
filterVisibleIncludes, fail-closed by type), keeps empty-entityId graph-level
issues, and recomputes Errors/Warnings/ByCheck so the aggregates can't leak the
count of hidden issues. Test TestACLAnalyze_FiltersHiddenIssues asserts a
ticket-only viewer sees the ticket issue, never the hidden feature's id/title,
and that the warning count matches the visible list. Self-reviewed; CI gates the
PR. Stacked on #991. Internal refactor → no docs.
