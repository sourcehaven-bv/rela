---
id: REV-SEDF2L
type: review-checklist
title: Review
status: done
---

## Automated Checks

- [x] All tests pass (`just test`)
- [x] Lint clean (`just lint`) — 0 issues
- [x] Comment lint gate clean (`just comment-lint`)
- [x] Coverage maintained (`just coverage-check`) — thresholds PASS

## Code Review

- [x] ~~Run `/code-review` command~~ (N/A: prose and comment corrections only, no
logic change; each claim was verified against the code before rewriting)
- [x] ~~All critical review-responses addressed~~ (N/A: none raised)
- [x] ~~All significant review-responses addressed~~ (N/A: none raised)
- [x] Self-reviewed the diff for unrelated changes

**Review Responses:** none

## Acceptance Verification

- [x] Each acceptance criterion tested
- [x] Test evidence documented in implementation checklist

**Acceptance Status:**

- Stale export-and-reimport advice removed — PASS. The passage now points at
`migrate_face` and states that a file spanning the change without one is
refused, which is what `validateDeltasResolved` enforces.
- `bare-row-on-faced-type` remedy is accurate — PASS. It named `migrate_face`,
whose `Validate` refuses outside a `faces_introduced` edge; it now names `rela
migrate adopt-face --entity <type>`. Verified end-to-end: the finding prints the
command, and running it clears the finding.
- Stale `store.EntityWriter` comment corrected — PASS. It claimed
`DeleteEntityState` refuses a default-face delete while named faces remain; no
implementation refuses (grep for `ErrInvalidQuery` in fsstore/memstore returns
nothing on that path), and `TestFaces_RowCanLeaveTheZeroCoordinate` asserts the
opposite.
- `faces_removed` mirror sentence left intact — PASS. That direction genuinely
has no step (TKT-1YBNQJ).

## Documentation (enhancements only)

- [x] Docs-checklist created and linked via `has-docs`
- [x] User-facing documentation updated
- [x] Docs-checklist marked as done

**Docs Checklist:** DOCS-B8C8VI

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [x] Run `/pr` command to create PR and monitor CI
