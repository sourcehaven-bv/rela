---
id: IMPL-2G01E8Y
type: implementation-checklist
title: 'Implementation: Store capability for range fingerprints'
started: "2026-11-07"
completed: "2026-11-07"
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

- [x] ~~Feature manually tested end-to-end~~ (N/A: store capability with no caller yet; the endpoint is step 5. Verified on all four backends by the conformance suite, including a live PostgreSQL database)
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified

**Verification Evidence:**
- AC1: `storetest.RunElementRangeTests` passes on memstore, fsstore, sqlitestore and pgstore (local PostgreSQL via `RELA_TEST_DATABASE_URL`).
- AC2: `TestElementRangeDifferential` (300 generated scopes and ranges against elementnaive) passes on sqlitestore and pgstore.
- AC3: conformance cases `relations_need_both_endpoints`, `relation_gate`, `entities`.
- AC4: `unknown_visibility_is_pending`, `rollup_name_with_no_condition_is_pending`, `TestElementRange_NullElementIsPending` on pg and sqlite.
- AC5: `TestElementRange_OneStatementAtAnySize` (pg, 1 statement at 10 and 50 rows), `TestRanges_ReadsDoNotGrowWithRows` (elementnaive, Counting), `TestElementRange_ExplainUsesElementIndex` on pg and sqlite.
- Validation and windows: `TestPlanElementRange_*`, `TestValidateRangeList`, conformance `invalid_requests`.
- Callers: `TestNoUnlistedElementRangeReads` (archguard).

## Quality

- [x] Code follows project patterns (check similar code)
- [x] Checked for DRY opportunities — repeated literals, expressions, or
patterns extracted to a helper / constant / type where it sharpens the contract
(don't extract for its own sake; CLAUDE.md "three similar lines is better than a
premature abstraction" still holds)
- [x] No security issues introduced
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind
