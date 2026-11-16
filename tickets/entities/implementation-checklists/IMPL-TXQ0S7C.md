---
id: IMPL-TXQ0S7C
type: implementation-checklist
title: 'Implementation: Store-side paging for list_entities filters that do not lower'
started: "2026-11-14"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code
- [x] Integration tests written (test full flow, not just units)
- [x] Happy path implemented
- [x] Edge cases from planning handled
- [x] Error handling in place (errors surfaced, not swallowed)

## Test Quality

- [x] Using fixture builders or factories for test data
- [x] No hardcoded values in assertions when object is in scope
- [x] Only specifying values that matter for the test
- [x] Interpolated values constructed from objects, not hardcoded
- [x] Property comparisons use original object, not hardcoded strings

## Manual Verification

- [x] ~~Feature manually tested end-to-end~~ (N/A: an internal read path; the handler-level differential and budget tests drive list_entities end to end over memstore, sqlite and postgres)
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified

**Verification Evidence:**

- `TestListEntities_ConditionMatchesGoPath` (memstore, sqlite, postgres): 13 filters x 5 principals x 4 pages. Rows and totals match the Go path, and each case asserts which path served it.
- `TestHandleListEntities_ConditionScaleFree`: 9 lowered filters cost the same at 100 and 1,000 tickets. It fails with the lowering disabled.
- `TestHandleListEntities_CurrentUserFilterAnswersForCaller` and `TestHandleListEntities_NoIdentityFailsClosed`.
- `TestQueryConditions_Lower` and `TestGatedReads_ConditionTraversalsGatedByReader` in appbuild; traversal cases in `TestScriptReader_ListEntityPage`.
- go test -race on mcp, visibility, appbuild, archguard, cli, dataentrywire; golangci-lint (default, sqlite, postgres tags), arch-lint and comment-lint clean.

## Quality

- [x] Code follows project patterns (check similar code)
- [x] Checked for DRY opportunities — repeated literals, expressions, or
patterns extracted to a helper / constant / type where it sharpens the contract
(don't extract for its own sake; CLAUDE.md "three similar lines is better than a
premature abstraction" still holds)
- [x] No security issues introduced
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind
