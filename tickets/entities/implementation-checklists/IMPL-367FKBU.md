---
id: IMPL-367FKBU
type: implementation-checklist
title: 'Implementation: Conflict routes answer 500 under a row-dependent read grant on fsstore'
started: "2026-11-05"
completed: "2026-11-05"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code (TestGraphQuery_SkipUnreadable, SkipUnreadable assertion in conferredfaces_test)
- [x] Integration tests written (test full flow, not just units) (fsstore over the shared naive graph query)
- [x] Happy path implemented
- [x] Edge cases from planning handled (flag off fails closed; negated endpoint match fails closed; traversal fold clears the flag)
- [x] Error handling in place (errors surfaced, not swallowed) (only ErrConflictedRow on an ACL read-grant query is skipped)

## Test Quality

- [x] Using fixture builders or factories for test data (conflictedTicketStore)
- [x] No hardcoded values in assertions when object is in scope
- [x] Only specifying values that matter for the test
- [x] Interpolated values constructed from objects, not hardcoded
- [x] Property comparisons use original object, not hardcoded strings

## Manual Verification

- [x] ~~Feature manually tested end-to-end~~ (N/A: store-level fix pinned by tests; no UI change)
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified (TestGraphQuery_SkipUnreadable fails on origin/develop)

**Verification Evidence:** TestGraphQuery_SkipUnreadable fails on origin/develop
with "file has unresolved git conflicts" and passes on the branch. The full
internal test run is clean.

## Quality

- [x] Code follows project patterns (check similar code) (an ACL-only GraphQuery field, like Any)
- [x] Checked for DRY opportunities (one skipUnreadable helper)
- [x] No security issues introduced (skip limited to the allowlist read gate after the security review)
- [x] No silent failures (errors logged AND returned) (the skip is logged at debug level)
- [x] No debug code left behind
