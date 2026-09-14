---
id: REV-4TZ3ZS
type: review-checklist
title: 'Review: No migration step can assign rows to a face, so bare_face adoption silently relabels every existing row'
status: done
---

## Automated Checks

- [x] All tests pass — `go test ./internal/...` clean across the repo.
- [x] Lint clean — `just lint` (whole repo): 0 issues.
- [x] Comment lint gate clean — `just comment-lint`: no unresolvable doc links
across 13999 comments. `just comment-report` shows no advisory findings in any
file this diff touches.
- [x] Coverage maintained — `just coverage-check` passes; datamigration 69.7%,
metamodel 87.0%, total 79.4%.

Also run: `just arch-lint` OK, `just plimsoll` clean, `just lint-md` 0 issues.

## Code Review

- [x] Run `/code-review` command (invokes cranky-code-reviewer agent)
- [x] All critical review-responses addressed
- [x] All significant review-responses addressed
- [x] Self-reviewed the diff for unrelated changes

**Review Responses:** RR-5KU4RS (critical), RR-YP42WK (critical), RR-0NCVBS
(significant), RR-OHKFRO (significant), RR-0HB6OX (significant), RR-MWDDBH
(minor), RR-Y9V0SP (minor). All `addressed`.

The review found the branch had **not fixed the bug**: it shipped a step that
could express the confirmation with nothing that required it, so a file with no
steps still parsed, applied and reported the schema in sync. I verified this by
applying an unedited draft against a scratch project — it succeeded. That is the
defect verbatim, and my earlier report that the fix was complete was wrong.

Two findings were scoped out rather than fixed, both with tickets and a written
exemption in `resolvingSteps`: TKT-L3P8I6 (`bare_face_changed` onto an occupied
face) and TKT-1YBNQJ (`bare_face_removed` still drafts an empty file).

## Acceptance Verification

- [x] Each acceptance criterion tested
- [x] Test evidence documented in implementation checklist

**Acceptance Status:**

1. *A migration can express which face existing rows become* — PASS.
`confirm_face` with an exhaustive mapping, or the whole-type form for types with
no enum to key on.
2. *A value with no sensible destination is caught at authoring time* — PASS.
`TestConfirmFace_RefusesNonExhaustiveMapping`; a non-bare target is refused with
guidance (`TestConfirmFace_RefusesNonBareTargetWithGuidance`), verified end to
end against a real project.
3. *A file that does not confirm the change is refused* — PASS. This is the
criterion the first pass missed.
`TestParseFile_RefusesUnconfirmedBareFaceAdoption`, verified end to end: a
do-nothing file is now rejected naming the entity and the delta.
4. *`migrate gen` drafts something that resolves the delta* — PASS. It emits a
live `confirm_face` step (a commented one could no longer parse), pre-filled
with the only answer the store permits.
5. *The class of defect cannot recur silently* — PASS.
`resolvingSteps` maps every `TierMigration` delta kind to the steps that resolve
it; `TestResolvingSteps_CoversEveryMigrationDeltaKind` fails on an unlisted
kind, and `TestMigrationDeltaKinds_MatchesTheClassifier` scans `shapecompare.go`
so the kind list cannot drift from the classifier. This is
AM-migration-delta-kinds-have-resolving-steps, implemented.

## Documentation

Bug fix, but it adds a user-facing step, so `docs/data-migration.md` was
updated: the step table, and a new section on adopting content states covering
both forms, the enforcement, why every value must map to the bare face, and the
`bare_face_changed` exclusion. No separate docs-checklist created — the
documentation change is in this diff and reviewed with it.

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [x] ~~Run `/pr` command to create PR and monitor CI~~ (N/A here: the PR
  post-dates this checklist — `/pr` gates on the bug already being `done`.
  Branch `fix/BUG-TMGWIN-map-face-step` is ready; see TKT-UFV01M.)
