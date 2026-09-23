---
id: IMPL-1HI1UX
type: implementation-checklist
title: 'Implementation: PostgreSQL read-path follow-ups: keyset position, title-ranked free text, header-backed view collections, bounded gantt drill-down'
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

- [x] Feature manually tested end-to-end
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified

**Verification Evidence:**
<!-- Document what you tested and the results -->

## Quality

- [x] Code follows project patterns (check similar code)
- [x] Checked for DRY opportunities — repeated literals, expressions, or
patterns extracted to a helper / constant / type where it sharpens the contract
(don't extract for its own sake; CLAUDE.md "three similar lines is better than a
premature abstraction" still holds)
- [x] No security issues introduced
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind

**Verification evidence (2026-09-20, seeded perf project, scale 1, manager principal, idle machine, third warmed pass):**

| request | before | after |
|---|---|---|
| `_position` (list scope sorted by due) | 80 ms wall, 50 ms db, whole type read and sorted in Go | 19 ms, one window statement |
| `_search q=telemetry` (term in most rows) | 1,240 ms | 216 ms |
| `_search q=TSK-0500` (rare term) | not measured before | 1.7 ms |
| `_views/project/PRJ-0001` | 149 ms wall, 48 ms db | 19 ms wall, 15 ms db |
| `_gantts/delivery?root=PRJ-0001` | 57 ms | 25 ms |
| `_gantts/delivery` (full, out of scope) | 145 ms | 92 ms |

Tests: storetest `GraphPosition` and `ListEntityIDsLargeBatch` on fs, mem, pg, sqlite; `TestPosition_StoreMatchesGoPath`, `TestPosition_StoreDeclines`, `TestGraphPosition_OneStatementAtAnySize`, `TestSearch_RanksByConfiguredTitle`, `TestViewBodies_*`, `TestGantt_SubtreeDrillBoundsItsEdgeRead`, `TestSimpleGraphSQL_*`. Gates: golangci-lint 0 issues, plimsoll, comment-lint, arch-lint, `just coverage-check` (79.9%), postgres and sqlite tagged suites.
