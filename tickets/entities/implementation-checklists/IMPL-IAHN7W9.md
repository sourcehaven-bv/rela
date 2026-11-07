---
id: IMPL-IAHN7W9
type: implementation-checklist
title: 'Implementation: Store content hash at write time and add a fingerprint column'
started: "2026-11-07"
completed: "2026-11-07"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code (pgstore `element_test.go`, `element_explain_test.go`; sqlitestore `element_test.go`; sqlitedb migration and soft-delete column tests updated)
- [x] Integration tests written (test full flow, not just units) (`storetest.RunElementTests` runs through `RunAll` on pg and sqlite and compares every stored element with the element computed from what the store reads back, after create, update, an update with an unchanged hash, rename, endpoint swap, family replace, soft delete, restore, face delete and family delete)
- [x] Happy path implemented (pg migration 0042 and sqlite v34 add `element_hash` and `element_fp0..3`; every single-row write sets them in the write statement; bulk writes fill them in the same transaction)
- [x] Edge cases from planning handled (unchanged canonical hash refilled after the trigger clears it; raw SQL change cleared by the trigger and healed by the sweep; heal guarded by xmin on pg and by one transaction on sqlite; restored rows rehashed; jsonb-equal write; non-ASCII address order; v33 to v34 and 41 to 42 upgrades with existing rows)
- [x] Error handling in place (errors surfaced, not swallowed) (write-path element errors fail the write; heal errors are logged and never abort version capture, by design)

## Test Quality

- [x] Using fixture builders or factories for test data (`elementEntity`, `seedElementRows`, storetest factories)
- [x] No hardcoded values in assertions when object is in scope (expected elements are computed from the rows the store returns)
- [x] Only specifying values that matter for the test
- [x] Interpolated values constructed from objects, not hardcoded (addresses come from `reconcile.EntityAddress`)
- [x] Property comparisons use original object, not hardcoded strings

## Manual Verification

- [x] Feature manually tested end-to-end
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified

**Verification Evidence:** Built `rela` with the sqlite tag and ran `db
import-fs` on a copy of the tickets project: 11499 entities and 13751 relations,
all with `element_hash` set. `update TKT-1G2XR1R -P priority=low` and `delete
RR-FQX8KBJ --force --cascade` left every row hashed. A raw `UPDATE entities SET
content = ...` cleared that row's element (trigger). Acceptance: no NULL after a
write is pinned on both database backends by `RunElementTests`; the range design
is pinned by `TestElementRangeSumUsesIndex` (pg generic plan uses
`entities_element_idx`) and the sqlite EXPLAIN check (covering index). The heal
is pinned by `TestElementRawWriteClearedAndHealed`, `TestElementHealBatches` and
`TestElementHealSkipsRowChangedAfterRead`. Full `just test-postgres` and `go
test ./internal/store/... ./internal/sqlitedb/...` pass.

## Quality

- [x] Code follows project patterns (check similar code) (mirrors the content_hash trigger-clears, sweep-fills pattern and the sweep's xmin guard and `noteBatch` rule)
- [x] Checked for DRY opportunities — repeated literals, expressions, or
patterns extracted to a helper / constant / type where it sharpens the contract
(don't extract for its own sake; CLAUDE.md "three similar lines is better than a
premature abstraction" still holds) (one `fillElements` per backend over an
`elementTable` descriptor; the address expressions are single constants)
- [x] No security issues introduced (no new input surface; stored values are derived from row content; SQL fragments are package constants)
- [x] No silent failures (errors logged AND returned) (heal failures are logged with a stall warning for a full batch without progress)
- [x] No debug code left behind
