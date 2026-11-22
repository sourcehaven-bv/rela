---
id: REV-8EDY0WS
type: review-checklist
title: 'Review: Read-only record actions, ways out of a closed workspace, banner placement, people list and dark mail body'
started: "2026-11-22"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (focused Go tests and vitest; the full suites run in CI)
- [x] Lint clean (golangci-lint on the changed packages, eslint on the changed files)
- [x] Comment lint gate clean
- [x] ~~Coverage maintained~~ (N/A locally: CI checks the floors)

## Code Review

- [x] Run `/code-review` command (invokes cranky-code-reviewer agent) (a critical self-review of the diff)
- [x] All critical review-responses addressed (none)
- [x] All significant review-responses addressed (RR-OI85UDC, RR-ZLV3VZJ)
- [x] Self-reviewed the diff for unrelated changes

**Review Responses:** RR-OI85UDC, RR-ZLV3VZJ, RR-J7VTRHZ, RR-01FG2ZL

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:**
- Read-only record actions hidden: PASS (workspaceReadOnly.test.ts, EntityDetail.header.test.ts)
- Workspace pages logo and links: PASS (WorkspaceAccessScreen.test.ts, TestBootShell, TestShellBootFrom)
- Contact text: PASS (WorkspaceAccessScreen.test.ts, TestValidateConfig_Account)
- Banner placement: PASS (ReadOnlyBanner.placement.test.ts)
- People list provisioning: PASS (TestProvision_MePost, me.test.ts)
- Dark mail body: PASS (TestCompat_DarkModePaintsTheBody; golden delta is the one rule)
- Documented: PASS (data-entry, postgres-backend and acl-security guides)

## Documentation (enhancements only)

- [x] ~~Docs-checklist created and linked via `has-docs`~~ (N/A: bug)
- [x] User-facing documentation updated (GUIDE-data-entry, GUIDE-postgres-backend, GUIDE-acl-security and docs/)
- [x] ~~Docs-checklist marked as done~~ (N/A: bug)

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [x] PR created with `gh pr create` after the bug is done; CI watched there
