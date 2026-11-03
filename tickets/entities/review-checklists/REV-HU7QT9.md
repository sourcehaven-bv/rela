---
id: REV-HU7QT9
type: review-checklist
title: 'Review: Conflict routes are not ACL-gated (content disclosure)'
started: "2026-10-28"
completed: "2026-10-28"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`just test`) (`just coverage-check` ran the race suite: 162 packages pass; cmd/rela-desktop needs the built SPA, absent in a fresh worktree; sqlitestore hit the 10m timeout under load and passes when rerun alone; neither is touched by this diff)
- [x] Lint clean (`just lint`) (golangci-lint on internal/dataentry: 0 issues; vue-tsc clean; eslint: no errors, one advisory max-lines warning on ConflictsView.vue, like 11 other files)
- [x] Comment lint gate clean (`just comment-lint`)
- [x] Coverage maintained (`just coverage-check`) (go-test-coverage: package and total floors PASS, 84.4%)

**Comment findings.** `just comment-report` shows no finding in code this diff
added.

## Code Review

- [x] Run `/code-review` command (invokes cranky-code-reviewer agent) (cranky-code-reviewer and rela-security-reviewer run on the diff)
- [x] All critical review-responses addressed (none raised)
- [x] All significant review-responses addressed (RR-U5TZN0, RR-RAW9PC)
- [x] Self-reviewed the diff for unrelated changes

**Review Responses:** RR-U5TZN0, RR-RAW9PC, RR-AUW0RY, RR-MSZ101, RR-KW56WG,
RR-STNYV5, RR-V8B9BX, RR-AG9R8N, RR-6CH6B8 (deferred), RR-EUC2FW, RR-13S0HY,
RR-JP0KZ2 (deferred)

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist) (bug-analysis checklist BUGA-2FRANS)
- [x] Test evidence documented in implementation checklist (IMPL-0OW35Y)

**Acceptance Status:**
- List needs git:sync: PASS (TestV1Conflicts_GitSyncGate list/*, golden conflicts.*)
- Detail needs git:sync, no file existence signal: PASS (detail/*, detail of a missing file/*, golden conflict-hidden.viewer is 403)
- Resolve needs git:sync and keeps its write check: PASS (resolve/*, TestV1ConflictResolveACLDenied, golden conflict-resolve.*)
- SPA hides the link without can_sync: PASS (SidebarFooter.conflicts.test.ts)
- SPA names git:sync on the permission 403 only: PASS (ConflictsView.test.ts)

## Documentation (enhancements only)

Skip this section for bugs and internal refactors.

- [x] ~~Docs-checklist created and linked via `has-docs`~~ (N/A: bug fix; the acl-security guide was still updated in docs-project and regenerated)
- [x] ~~User-facing documentation updated~~ (N/A: bug section skipped; GUIDE-acl-security updated anyway)
- [x] ~~Docs-checklist marked as done~~ (N/A: no docs-checklist for a bug)

## Final Checks

- [x] Commit message explains the why, not just what (minimal message per the owner's preference; the ticket carries the why)
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [x] Run `/pr` command to create PR and monitor CI (PR opened with gh pr create after done, as the workflow requires)
