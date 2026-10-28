---
id: IMPL-0OW35Y
type: implementation-checklist
title: 'Implementation: Conflict routes are not ACL-gated (content disclosure)'
started: "2026-10-28"
completed: "2026-10-28"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code (TestV1Conflicts_GitSyncGate: list, detail, missing-file detail, resolve, missing-file resolve across no policy, read-only, policy with and without git:sync, broken and unwired ACL; ConflictsView.test.ts; SidebarFooter.conflicts.test.ts)
- [x] Integration tests written (test full flow, not just units) (golden corpus drives the production router with real identities: conflicts.*, conflict.*, conflict-hidden.*, conflict-resolve.*)
- [x] Happy path implemented (git:sync holder lists, views and resolves; golden admin cases record 200)
- [x] Edge cases from planning handled (resolve keeps its write check: TestV1ConflictResolveACLDenied; missing file gives the same 403; nil and broken ACL fail closed)
- [x] Error handling in place (errors surfaced, not swallowed) (403 problem type permission_required names git:sync; SPA shows it without Retry, other errors keep the server message)

## Test Quality

- [x] Using fixture builders or factories for test data (newConflictTestApp, writeProjectFile, mustNewACL, the golden fixture)
- [x] No hardcoded values in assertions when object is in scope (assertions use acl.PermGitSync and the conflictedTicket constant)
- [x] Only specifying values that matter for the test
- [x] Interpolated values constructed from objects, not hardcoded (request paths built from the rel constant)
- [x] Property comparisons use original object, not hardcoded strings (file content compared with conflictedTicket)

## Manual Verification

- [x] ~~Feature manually tested end-to-end~~ (N/A: verified through the golden corpus, which runs the production router with header identities on a fixture holding a conflicted file; no browser session run)
- [x] Each acceptance criterion verified with test scenario from planning (viewer/editor/outsider/stranger get 403 on list, detail and resolve; admin with git:sync gets 200; SPA link hidden without can_sync; 403 message names git:sync)
- [x] Edge cases manually verified (inspected the re-recorded goldens: conflict-hidden.viewer is 403 without TKT-4 values, conflict-hidden.admin is 200 with both sides)

**Verification Evidence:** go test ./internal/dataentry/...
./internal/archguard/... pass; TestGoldenParity re-recorded and passing; vitest
for ConflictsView and SidebarFooter pass; vue-tsc clean; just arch-lint and just
comment-lint clean.

## Quality

- [x] Code follows project patterns (check similar code) (same gate and 403 shape as POST /_git/sync; permission_required matches actions and documents)
- [x] Checked for DRY opportunities (one permitsGitSync and one writeGitSyncForbidden shared by sync, list, detail and resolve)
- [x] No security issues introduced (rela-security-reviewer: no critical or significant findings)
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind
