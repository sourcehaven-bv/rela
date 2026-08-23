---
id: IMPL-20W3ZF
type: implementation-checklist
title: 'Implementation: storetest: cover Freshness.LastModified and declare the Tx tier in Capabilities'
status: done
---

<!-- @managed: claude-workflow v1 -->

Branch `test/storetest-freshness-txtier-TKT-8TJ2WN` · PR #1419.

## Development

- [x] Unit tests written for new code — `storetest/freshness.go`, five subtests
- [x] Integration tests written — the suite runs against all three real
backends via their existing conformance entry points, pgstore against a live
database. That IS the integration test: a conformance suite that only ran
against a fake would prove nothing about the implementations it exists to
constrain.
- [x] Happy path implemented
- [x] Edge cases from planning handled — clock granularity, empty store, reads
not advancing the timestamp
- [x] Error handling in place — every store call is `require.NoError`'d, so a
backend erroring where it should succeed fails loudly rather than being read as
a zero time

## Test Quality

- [x] Using fixture builders or factories for test data — the suite's own
`Factory` and `seedEntities`, so these tests get the same data shape as every
other area
- [x] No hardcoded values in assertions when object is in scope — assertions
compare `before`/`after` readings, never literal timestamps
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
| 1 — five properties covered | **PASS** | `EmptyStoreReturnsZeroTime`, `AdvancesOnEntityWrite`, `CoversRelationWrites`, `StableWithoutWrites`, `UTCComparable` |
| 2 — all three backends pass unchanged | **PASS** | fsstore, memstore and pgstore green; no backend code changed |
| 3 — non-vacuous | **PASS** | see below |
| 4 — `TxRollback` runs from `RunAll` | **PASS** | pgstore's `TestConformance/TxRollback` now runs the four subtests; its separate `TestTxRollback` entry point is gone |
| 5 — lint + arch-lint | **PASS** | `golangci-lint` 0 issues (default and `--build-tags postgres`); arch-lint "OK - No warnings found" |

**AC-3 verified by deliberate regression, not by going green.** I removed the
relation scan from memstore's `LastModified` — the exact mistake a new backend
makes, since every entity-only test still passes without it:

```
--- FAIL: TestConformance/Freshness/CoversRelationWrites
    LastModified must cover relation writes (... -> ...); a store that only
    scans entities leaves relation-only changes invisible to every consumer
    maintaining derived state
```

Then restored it. This check mattered: TKT-415WA7's review found one of my tests
had a tautologically-false assertion that passed whether or not the code worked,
so "it went green" is not evidence on its own.

**Test runs:** fsstore 2.9s · memstore 2.5s · pgstore 27.9s (default build) ·
pgstore 30.4s (`-tags postgres`, live DB) · storetest 0.8s · storeutil 1.0s.

## Quality

- [x] Code follows project patterns — one exported `Run*Tests` per area, gated
from `RunAll`, `Capabilities` for optional areas (the `Attachments` precedent)
- [x] Checked for DRY opportunities — `waitForClock` is one helper rather than
four inline sleeps, and it carries the reason (filesystem mtime granularity) so
it is not "simplified" to a shorter sleep that makes the suite flaky
- [x] No security issues introduced — test-only; no production code changed
- [x] No silent failures — the whole ticket exists to remove two silent ones
- [x] No debug code left behind
