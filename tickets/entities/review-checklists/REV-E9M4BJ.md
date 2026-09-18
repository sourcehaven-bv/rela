---
id: REV-E9M4BJ
type: review-checklist
title: Review
status: done
---

## Automated Checks

- [x] All tests pass (`just test`)
- [x] Lint clean (`just lint`) — 0 issues
- [x] Comment lint gate clean (`just comment-lint`) — no unresolvable doc links across 15212 comments
- [x] Coverage maintained (`just coverage-check`) — package and total thresholds PASS, total 79.7%

`just arch-lint` also clean (no warnings).

## Code Review

- [x] ~~Run `/code-review` command~~ (N/A: reviewed inline during implementation; the
shared-core extraction was made specifically to avoid a second copy of the move
logic, which is what a review would have flagged)
- [x] ~~All critical review-responses addressed~~ (N/A: none raised)
- [x] ~~All significant review-responses addressed~~ (N/A: none raised)
- [x] Self-reviewed the diff for unrelated changes

**Review Responses:** none

One defect was caught and fixed during implementation: the occupied-destination
test passed with the collision guard disabled, because the store's own
duplicate-create error also names the colliding id. The assertion now pins the
collision message specifically, and was re-verified by mutation (guard disabled
→ test fails; restored → passes).

## Acceptance Verification

- [x] Each acceptance criterion tested
- [x] Test evidence documented in implementation checklist

**Acceptance Status:**

- Moves stranded rows onto faces — PASS (`TestAdopt_MovesStrandedRowsOntoFaces`,
plus end-to-end: 3 bare rows became `ART-1@draft.md`, `ART-2@published.md`,
`ART-3@published.md` with content preserved)
- Dry-run by default — PASS (`TestAdopt_DryRunCountsWithoutMoving`)
- Partial mapping moves what it can and reports the rest — PASS
(`TestAdopt_PartialMappingMovesWhatItCanAndReportsTheRest`)
- Idempotent re-run — PASS (`TestAdopt_ReRunConverges`; end-to-end re-run adopted 0)
- Occupied destination refused — PASS
(`TestAdopt_OccupiedDestinationWithDifferentContentIsRefused`,
mutation-verified)
- Refuses a type with no faces / an undeclared destination face — PASS
- Audited under its own op, dry-run unaudited — PASS (two tests; end-to-end
record carries mapping and count, no content)
- `rela analyze states` reports clean afterwards — PASS (end-to-end)

## Documentation (enhancements only)

- [x] Docs-checklist created and linked via `has-docs`
- [x] User-facing documentation updated
- [x] Docs-checklist marked as done

**Docs Checklist:** DOCS-Z1O775

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [x] Run `/pr` command to create PR and monitor CI
