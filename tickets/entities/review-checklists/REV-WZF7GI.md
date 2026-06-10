---
id: REV-WZF7GI
type: review-checklist
title: 'Review: ACL read-side (PR 2/2): list endpoints + sidebar counts + pagination headers'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`just test`) — full `go test ./internal/... ./cmd/...` green (race detector via CI)
- [x] Lint clean (`just lint`) — 0 issues after `just lint-fix` (one gofmt nit)
- [x] Coverage maintained (`just coverage-check`) — 74.5% total, all package floors pass; `just arch-lint` pass

## Code Review

- [x] Run `/code-review` command (invokes cranky-code-reviewer agent)
- [x] All critical review-responses addressed
- [x] All significant review-responses addressed
- [x] Self-reviewed the diff for unrelated changes

**Review Responses:** RR-TLT9CR (critical — `_position` source=search gate
bypass; fixed via `readableSubset`), RR-HPJYYX (significant — write⊆read
invariant overstated; verified affordance grants never authorize writes,
narrowed docs, added pinning test), RR-4KBP7W (significant — AllowAll silent
truncation; `errListLoad` + `writeListPipelineError`), RR-BW2Y8J (minor — raw
error echo in `acl_query_failed`; deferred to a cross-cutting 5xx-detail
hardening follow-up, consistent with RR-89XK), RR-OBXBWL (nit — sidebar per-item
recompute; documented, accepted). All fixes in commit 622b6cf7.

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:**

| AC | Status | Evidence |
|---|---|---|
| AC1 type-level read grant on list | PASS | TestACLList_TypeLevelReadGrant + live curl |
| AC2 role-relation + inheritance on list | PASS | TestACLList_RoleRelationInheritance + live curl (alice editor-of PRJ-42) |
| AC3 eight pagination leak surfaces | PASS | TestACLList_PaginationLeakSurfaces (asserts "10" appears in no body/header) + live headers |
| AC4 DenyAll shape + _actions.create=false | PASS | TestACLList_DenyAllShape + live bob curl |
| AC5 DenyAll search short-circuit | PASS | TestACLList_DenyAllSearchShortCircuit (recording searcher, 0 calls) |
| AC6 sidebar counts match list | PASS | TestACLSidebar_CountsMatchList (5 of 10) + live sidebar |
| AC7 sidebar count under config filter | PASS | TestACLSidebar_ConfigFilterIntersection (visible∩open = 3) + live |
| AC8 policy-load rejects write-without-read | PASS | TestLoadPolicy_WriteWithoutRead_Rejected (7 cases) + live boot failure |
| AC9 search-after-ACL ordering | PASS | TestACLList_SearchAfterACLOrdering (order-recording store + searcher) |
| AC10 per-principal caching headers | PASS | TestACLList_VaryHeader + live `Vary: X-Rela-User` |
| AC11 NopACL regression | PASS | acl_list_regression_test.go + TestACLSidebar_NopACLFullCounts |
| AC12 GUIDE-acl-security updated | PASS | read-path rewrite; docs/acl-security.md regenerated |

## Documentation (enhancements only)

- [x] Docs-checklist created and linked via `has-docs`
- [x] User-facing documentation updated
- [x] Docs-checklist marked as done

**Docs Checklist:** DOCS-X1K2FE

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [x] Run `/pr` command to create PR and monitor CI
- [x] All CI checks pass — monitored after push; stacked PR, base feat/acl-readside-tkt-vqgn (#939)
- [x] PR URL documented below

**PR:** https://github.com/sourcehaven-bv/rela/pull/949
