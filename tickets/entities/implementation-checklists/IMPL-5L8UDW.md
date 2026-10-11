---
id: IMPL-5L8UDW
type: implementation-checklist
title: 'Implementation: Version sweep selects only rows without a stored hash'
started: "2026-10-09"
completed: "2026-10-09"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code (`TestVersionTriggersClearContentHash` and `TestSweepWriteBackSkipsARowWithANewerVersion` on both backends; `TestRelationVersionTriggersClearContentHash`, `TestMigrateClearsV14Hashes` on sqlite)
- [x] Integration tests written (test full flow, not just units) (storetest `SweepBacklog/VersionWrittenOutsideTheSweep`, both backends)
- [x] Happy path implemented
- [x] Edge cases from planning handled
- [x] Error handling in place (errors surfaced, not swallowed)

## Test Quality

- [x] ~~Using fixture builders or factories for test data~~ (N/A: storetest seed helpers and raw SQL rows)
- [x] No hardcoded values in assertions when object is in scope
- [x] Only specifying values that matter for the test
- [x] Interpolated values constructed from objects, not hardcoded
- [x] Property comparisons use original object, not hardcoded strings

## Manual Verification

- [x] Feature manually tested end-to-end
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified

**Verification Evidence:**
- EXPLAIN after sweeping 5,000 entities and 2,999 relations: pgstore scans `entities_unhashed_idx` and `relations_unhashed_idx` (0.089 ms and 0.026 ms); sqlite `SCAN e USING INDEX entities_unhashed_idx` and the relation equivalent.
- Mutations: removing the hash condition from the sqlite version triggers fails all three `VersionWrittenOutsideTheSweep` cases; removing the latest-version write-back guard fails `TestSweepWriteBackSkipsARowWithANewerVersion` on both backends.
- `go test -race` on pgstore (postgres tag), sqlitestore and sqlitedb (sqlite tag), and store plus entitymanager (default tag): all pass.

## Quality

- [x] Code follows project patterns (check similar code)
- [x] Checked for DRY opportunities — repeated literals, expressions, or
patterns extracted to a helper / constant / type where it sharpens the contract
(don't extract for its own sake; CLAUDE.md "three similar lines is better than a
premature abstraction" still holds)
- [x] No security issues introduced
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind
