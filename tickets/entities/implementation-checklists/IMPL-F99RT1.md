---
id: IMPL-F99RT1
type: implementation-checklist
title: 'Implementation: Version sweep starves: edits after the first Batch settled rows are never captured'
started: "2026-10-08"
completed: "2026-10-08"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code (`storetest.RunSweepBacklogTests`, `TestContentHashTriggers`, `TestMigrateToContentHash`, `TestSweepWriteBackSkipsARowWrittenAfterTheRead` on both backends)
- [x] Integration tests written (test full flow, not just units) (conformance suite against a real postgres and sqlite store, driving real sweep ticks)
- [x] Happy path implemented
- [x] Edge cases from planning handled (never-captured rows, an edit to each hashed column behind a full batch, uint64 above 2^53, unchanged save after force-live purge, concurrent write during write-back via xmin/content guard)
- [x] Error handling in place (errors surfaced, not swallowed) (write-back errors are returned and logged per row like capture errors)

## Test Quality

- [x] Using fixture builders or factories for test data (`newEntity`, `newRelation`, `relKey`, `seed` helpers)
- [x] No hardcoded values in assertions when object is in scope
- [x] Only specifying values that matter for the test
- [x] Interpolated values constructed from objects, not hardcoded
- [x] Property comparisons use original object, not hardcoded strings

## Manual Verification

- [x] Feature manually tested end-to-end (backlog tests fail on origin/develop's sweep and on the first column-gate attempt, and pass with the stored hash, on both backends)
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified (atlas data read-only: 181 never-captured and 110 changed entities pending)

**Verification Evidence:**
`RELA_TEST_DATABASE_REQUIRED=1 go test -race -tags postgres ./internal/store/pgstore/...` and
`go test -race -tags sqlite ./internal/store/sqlitestore/... ./internal/sqlitedb/...` pass. `SweepBacklog`
against origin/develop's `sweep.go` and against the first column-gate attempt fails on both backends (every
seeded row carries a uint64 above 2^53). Mutation checks: removing the xmin or content write-back guard, `type`
from the entity trigger, or `from_id` from the relation trigger each fails a test.

## Quality

- [x] Code follows project patterns (check similar code) (conformance test next to `RunSweepAttributionTests`; migration rung like v9's `addColumns`)
- [x] Checked for DRY opportunities
- [x] No security issues introduced (no new inputs; the trigger only clears a derived column)
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind
