---
id: REV-GUBE0D
type: review-checklist
title: 'Review: Family delete authorizes one face and deletes all faces'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`just test`): entitymanager under -race; family-delete, delete and concurrency tests on memstore, fsstore, sqlite, postgres and postgres-two-processes
- [x] Lint clean (`just lint`): golangci-lint 0 issues; arch-lint and plimsoll clean
- [x] Comment lint gate clean (`just comment-lint`)
- [x] Coverage maintained (`just coverage-check`): entitymanager at 87.3% against the 50% package floor; the new branches are covered by the TOCTOU tests

## Code Review

- [x] Run `/code-review` command (invokes cranky-code-reviewer agent)
- [x] All critical review-responses addressed (none)
- [x] All significant review-responses addressed (two addressed; the cascadehost one is deferred as out of scope)
- [x] Self-reviewed the diff for unrelated changes

**Review Responses:** see the has-review-response relations on BUG-1YN750.

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:**

- Delete authorized on every removed face, fail closed with no store write: PASS (TestFamilyDelete_DeniedUnlessEveryFaceIsDeletable, TestFamilyDelete_FaceAddedDuringDeleteIsAuthorized/before-tx)
- One version capture and one audit record per removed face: PASS (TestFamilyDelete_EveryFaceCapturedAndAudited)
- Faces appearing mid-delete are authorized; fs records what it removed: PASS (TestFamilyDelete_FaceAddedDuringDeleteIsAuthorized/before-store-delete)
- Every caller path reaches the fix: PASS (CLI, MCP, Lua, HTTP bare-id delete and CalDAV all call Manager.DeleteEntity)

## Documentation (enhancements only)

- [x] ~~Docs-checklist created and linked via `has-docs`~~ (N/A: bug fix)
- [x] User-facing documentation updated (docs/acl-security.md: Deleting an entity needs delete on every face)
- [x] ~~Docs-checklist marked as done~~ (N/A: bug fix)

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [x] Run `/pr` command to create PR and monitor CI (PR opened against faces-intrinsic after this checklist, per TKT-UFV01M)
