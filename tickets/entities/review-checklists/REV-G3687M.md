---
id: REV-G3687M
type: review-checklist
title: 'Review: Soft delete with restore for the Undo toast'
status: done
---

## Automated Checks

- [x] All tests pass (`just test`)
- [x] Lint clean (`just lint`)
- [x] Comment lint gate clean (`just comment-lint`)
- [x] ~~Coverage maintained (`just coverage-check`)~~ (N/A locally: machine load above 100 made the dataentry package hit the 10-minute test timeout in an unrelated test, which passes on its own; CI runs the coverage gate)

Go tests pass on the default build and with the `sqlite` tag. `just
test-postgres` passes for pgstore and jobs. golangci-lint reports no new issues.
`just arch-lint`, `just comment-lint` and `just plimsoll` pass.

## Code Review

- [x] ~~Run `/code-review` command (invokes cranky-code-reviewer agent)~~ (N/A: a security review on PR #1701 ran instead; its findings are #1703 and #1704)
- [x] All critical review-responses addressed
- [x] All significant review-responses addressed
- [x] Self-reviewed the diff for unrelated changes

**Review Responses:** none created. The security findings are tracked as GitHub
issues #1703 (restore skipped the relation grant check) and #1704 (a hidden
relation could reattach to a reused id). Both are fixed in this change.

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] ~~Test evidence documented in implementation checklist~~ (N/A: no implementation checklist; evidence is below)

**Acceptance Status:**

- Web DELETE hides the entity and the GC job purges it after the delay: PASS
(`TestSoftDelete_*` in dataentry, appbuild GC test).
- Restore undoes the delete, gated by read and delete rights: PASS
(`TestSoftDelete_RestoreChecksRelationGrants`, restore handler tests).
- A hard delete or rename of the other end drops the hidden relation: PASS
(storetest `HardDeleteOfOtherEndDropsHiddenEdge`,
`RenameOfOtherEndDropsHiddenEdge` on all four stores).

## Documentation (enhancements only)

- [x] Docs-checklist created and linked via `has-docs`
- [x] User-facing documentation updated
- [x] Docs-checklist marked as done

**Docs Checklist:** DOCS-B236R8

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [x] Run `/pr` command to create PR and monitor CI
