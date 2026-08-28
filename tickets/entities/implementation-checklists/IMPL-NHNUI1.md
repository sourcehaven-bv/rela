---
id: IMPL-NHNUI1
type: implementation-checklist
title: 'Implementation: SQLite store backend: conformance-passing minimal store behind a sqlite build tag'
status: done
---

<!-- @managed: claude-workflow v1 -->

Branch `feat/sqlite-backend-TKT-G91TBK` · PR #1421, stacked on #1420 → #1419 →
#1417. Wiring split out to TKT-L1A3PH.

## Development

- [x] Unit tests written for new code — lock/WAL tests
- [x] Integration tests written — `storetest.RunAll` + `RunTxStressTest` +
six fuzz targets. The shared conformance suite IS the integration test: it
exercises the backend through the same contract every other store is held to,
which is what makes the backends substitutable.
- [x] Happy path implemented — all 26 `store.Store` methods
- [x] Edge cases from planning handled — nested Tx, ctx cancellation, empty
store, case-variant IDs, quotes in property names, attachment cascade, lock
release on Close
- [x] Error handling in place — sentinels preserved through `%w`; no silent
fallbacks

## Test Quality

- [x] Using fixture builders or factories for test data — `storetest.Factory`
and `FuzzFactory`
- [x] No hardcoded values in assertions when object is in scope
- [x] Only specifying values that matter
- [x] Interpolated values constructed from objects, not hardcoded
- [x] Property comparisons use original object, not hardcoded strings

## Manual Verification

- [x] Feature manually tested end-to-end
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified

**Verification Evidence:**

| AC | Verdict | Evidence |
|----|---------|----------|
| 1 — `RunAll` | **PASS** | with `{Attachments: true, TxRollback: true}` |
| 2 — `RunTxStressTest` 30s | **PASS** | **135,374 counter commits, 110,318 pair txs**, sweep clean, no watchdog dump |
| 3 — `RunTxRollbackTests` | **PASS** | via `Capabilities.TxRollback`, so it runs inside `RunAll` |
| 4 — six fuzz targets | **PASS** | all six |
| 5 — second opener refused | **PASS** | `TestSecondOpenIsRefused`; error names the holding pid |
| 6 — WAL guard | **PASS** | `TestWALIsEnabled`; `Open` refuses non-WAL |
| 7 — CI isolation | **MOVED** | to TKT-L1A3PH — nothing to assert about until a sqlite binary exists. The verifiable half holds: neither default nor postgres links `modernc.org/sqlite` |
| 8 — arch-lint / lint / coverage | **PASS** | arch-lint clean, `golangci-lint` 0 issues, coverage 83.6% |

**Seven real bugs, every one caught by the conformance suite rather than by
reading the code:**

| Bug | Would have caused |
|---|---|
| `q.IDs` ignored by `ListEntities`/`CountEntities` | filtered queries returning everything |
| `DeleteResult.DeletedEntities` unpopulated | callers unable to report what was deleted |
| `HighestID` missing the `-` separator | id generation restarting at 1 forever |
| Attachments not cascading on entity delete | orphaned blobs |
| Case-variant IDs coexisting | BUG-3RCWNS — backends disagreeing on identity |
| JSON numbers as `float64` not `int` | silent type divergence from fs/mem |
| `'$.<name>'` built by string concatenation | **fuzz-found**: a `"` in a property name produced a bad-JSON-path error |

That last one is the most interesting: the fix was not to escape the name but to
restructure the query so escaping is unnecessary. `json_each` yields keys as
VALUES, so the name is a bound parameter and never enters a second grammar
nested inside SQL — the shape that goes wrong quietly later.

**And one the race detector caught in CI that local runs missed.**
`processLock.release()` had a data race on `l.file`: `Close` is documented as
idempotent and callers wire it into `defer` and `t.Cleanup`, so two goroutines
calling it at once is ordinary usage, not misuse. Fixed with a mutex; verified
locally with `go test -race`, which now passes.

## Quality

- [x] Code follows project patterns — Tx shape and case-identity index from
pgstore, `graphquerynaive` delegation from fsstore, `HighestID` convention from
memstore, `storeutil` for cursors/validation/`TopValues`
- [x] Checked for DRY opportunities — one `querier` seam serves pooled and
tx-bound execution; `buildRelationQueryFrom` is shared by list, count and page
so filter semantics cannot drift between them
- [x] No security issues introduced — all SQL parameterized; IDs, relation
types, property names and file names validated through `storeutil`/`store`
helpers; no credential handling (SQLite has no DSN)
- [x] No silent failures — `verifyBusyTimeout` fails at `Open` rather than as
mysterious `SQLITE_BUSY` under load; the WAL guard refuses rather than
degrading; the lock refuses rather than corrupting
- [x] No debug code left behind
