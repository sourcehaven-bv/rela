---
id: REV-B17UR4
type: review-checklist
title: 'Review: Relation edits silently dropped when a linked entity is outside the picker''s first-100 candidate window'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass — frontend 2473/2473 (152 files). No Go files changed, so the Go suite is untouched.
- [x] Lint clean — `npm run lint` 0 errors; the four changed files produce 0 warnings. `just arch-lint` OK.
- [x] ~~Comment lint gate clean~~ (N/A: `commentlint` is a Go-source gate; this change is frontend-only)
- [x] ~~Coverage maintained~~ (N/A: `go-test-coverage` floors are Go-only; the frontend has no coverage enforcement, per CLAUDE.md)

## Code Review

- [x] Run `/code-review` command (cranky-code-reviewer, two passes)
- [x] All critical review-responses addressed — RR-HS1TRV (post-mount re-resolve)
- [x] All significant review-responses addressed — RR-5FDUBP, RR-NQQ70Z, RR-I4WO8W, RR-D0NWIR addressed; RR-25GKS7 wont-fix with reason
- [x] Self-reviewed the diff for unrelated changes — the diff is four files, all BUG-LSCDJK. Prettier reformatting of untouched regions was reverted deliberately (the repo is not prettier-clean; reformatting would have buried the fix).

**Review Responses:** RR-HS1TRV (critical); RR-5FDUBP, RR-NQQ70Z, RR-I4WO8W,
RR-D0NWIR, RR-25GKS7 (significant); RR-E8LMA0, RR-W5T5Z7 (minor)

**One reviewer claim was checked and rejected:** the review asserted the
`max-lines` warning on RelationPicker.vue was pre-existing. Stashing the change
and re-linting showed no warning on the original file — the change introduced
it. Fixed properly by extracting `outOfPageLinks.ts` rather than accepting a
warning on a false premise.

## Acceptance Verification

- [x] Each acceptance criterion tested
- [x] Test evidence documented in implementation checklist

**Acceptance Status:**

- *A relation edit saves regardless of target-type volume* — **PASS**. Live server, 126 labels: the candidate page cannot resolve the linked label, the relations endpoint returns `type: label` for it, and the resulting PATCH returns HTTP 200 with the out-of-page link preserved.
- *The out-of-page link is visible and removable* — **PASS** (unit: chip rendering, dedupe, removal).
- *A post-mount reload does not reintroduce the abort* — **PASS** (`resolves an out-of-page id that arrives after mount`, written failing first).
- *A failed resolve degrades rather than blocking* — **PASS** (`still emits types for in-page ids when the relations lookup fails`).

## Documentation (enhancements only)

Skipped: bug fix, no user-facing surface change.

- [x] ~~Docs-checklist created and linked via `has-docs`~~ (N/A: bug fix)
- [x] ~~User-facing documentation updated~~ (N/A: no behaviour to document; the fix removes a failure)
- [x] ~~Docs-checklist marked as done~~ (N/A: bug fix)

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [x] ~~Run `/pr` command to create PR and monitor CI~~ (N/A here: `/pr` gates on the ticket already being `done` and validating clean, so this item cannot be satisfied before the checklist closes — see TKT-UFV01M. The PR is opened after this checklist is done.)
