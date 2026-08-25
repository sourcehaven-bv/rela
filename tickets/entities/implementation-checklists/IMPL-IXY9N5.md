---
id: IMPL-IXY9N5
type: implementation-checklist
title: 'Implementation: Investigate SQLite as a third store backend for single-server and desktop deployments'
status: done
---

<!-- @managed: claude-workflow v1 -->

**This ticket is investigation-only.** The "implementation" is a throwaway spike
whose only product is evidence (see PLAN-ZLVJC3). Items about shipping code are
marked N/A with a reason rather than silently ticked.

Branch: `spike/sqlite-tx-TKT-TWIO11` — **never merge**. Raw numbers:
`internal/store/sqlitespike/RESULTS.md`.

## Development

- [x] Unit tests written for new code
- [x] Integration tests written (test full flow, not just units) — the spike
runs the SHARED conformance harness (`storetest.RunTxStressTest`,
`RunTxRollbackTests`, 4 fuzz targets) rather than bespoke tests, which is the
point: it measures against the real contract.
- [x] Happy path implemented — the `store.Store` subset the Tx tests exercise
- [x] Edge cases from planning handled — nested Tx joins without a pool
acquire (`pgstore/tx.go:61-63` shape); `journal_mode` return value checked, not
assumed; fuzz temp-dir lifecycle; ctx-cancellation rollback via
`context.WithoutCancel`
- [x] Error handling in place — every unimplemented method returns
`errUnsupported` loudly. A silent stub in a spike produces confident, wrong
evidence, so none were used.

## Test Quality

- [x] Using fixture builders or factories for test data — `storetest.Factory` /
`FuzzFactory`, plus a table-driven `arms()` matrix
- [x] No hardcoded values in assertions when object is in scope
- [x] Only specifying values that matter for the test — each arm varies ONLY
pool size, Tx mode and event mode; everything else is held constant, which is
what makes B a valid control for A
- [x] Interpolated values constructed from objects, not hardcoded
- [x] Property comparisons use original object, not hardcoded strings

## Manual Verification

- [x] Feature manually tested end-to-end
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified

**Verification Evidence:**

**Headline finding — `busy_timeout` must come from the DSN, not `db.Exec`.** A
PRAGMA is per-connection, so `db.Exec` configures one pooled connection and
leaves the rest at 0 — while reading back correctly. Measured with a holder
keeping a write tx open 2s:

| Configuration | Second writer |
|---|---|
| `db.Exec("PRAGMA busy_timeout=5000")` | fails at **0.00s**, SQLITE_BUSY |
| DSN `?_pragma=busy_timeout(5000)` | waits **2.05s**, succeeds |

Reproduces upstream #192, and plausibly explains #232 (mattn takes
`_busy_timeout` in the DSN, so a straight port loses it). **Neither reproduced
as a driver bug once the DSN form was used.** Guarded by `verifyBusyTimeout`,
which pins two connections open at once and asserts both agree.

**AC-3 — PASSES.** Arm A (4 conns, `BEGIN IMMEDIATE`) at 30s: **31,106 counter
commits, 22,036 pair txs**, final sweep clean, no watchdog dump. Controls make
it a measurement rather than an assumption:

| Arm | Result |
|---|---|
| A — 4 conns, IMMEDIATE | PASS (30s soak) |
| B — 4 conns, deferred | **FAIL**: `counter tx: update STRS-CTR: database is locked (5)` → `BEGIN IMMEDIATE` is load-bearing |
| C — `MaxOpenConns(1)` | **HANGS** — `database/sql` pool starvation, NOT a SQLite lock; reproduced in ~30 lines with no rela code |
| D — ship shape, inline events | PASS — 3,451 commits |
| E — `RunTxRollbackTests` | **PASS**, incl. `NoEventsOnRollback` + `EventsDeliveredAfterCommit` |

Arm C retroactively vindicates design-review finding C3: had it stayed primary
as first planned, the spike would have hung and been misread as "modernc's
locking is broken" — the opposite of the evidence.

**Arm E is the material surprise.** SQLite gives the STRONG Tx contract free.
Under DEC-8UIL0 a SQLite backend sits at **pgstore's tier, not fsstore's**,
giving up only cross-process serialization.

**AC-4 — answered.** (a) `unique:` is an untransacted `ListEntities` scan
(`entitymanager/unique.go:82`, zero `.Tx(` sites in the package), so the race is
not even multi-process-specific; a multi-process SQLite backend inheriting the
`!postgres` no-op would have NO uniqueness backstop. (b) 8 racers on one ID →
**1 winner, 7 `ErrConflict`, 0 other**: SQLite's own constraints ARE
cross-connection; the application scan is not.

**AC-5 — passes with large margin** (10k entities, local APFS):

| | Measured | Gate |
|---|---|---|
| Cold open | **2.23ms** | <1s (~450x) |
| Steady-state write | **127µs** | <50ms (~390x) |
| Bulk load | 223ms (44,749/s) | not gated |

Cold open is the clearest win over fsstore, which parses and indexes all 10k
markdown files at startup — the residency that motivated the investigation.

**Fuzz — 4/4 pass, 68k execs, and one found a REAL gap.**
`FuzzRelationKeyCollision` seed #4 failed initially: `CreateRelation` accepted
an empty relation type because the spike skipped
`storeutil.ValidateRelationType`. Fixed. Evidence that the conformance harness
earns its keep — a hand-written store WILL miss validation the oracle enforces.

**AC-6 — `synchronous=NORMAL`** is recorded as a durability decision, not a
knob: in WAL it means a crash can lose recently committed transactions, which
matters for a backend whose purpose is replacing git as the history story.

**Arm F — NOT RUN.** No network storage available (operator decision,
2026-08-23). Carried forward as an **accepted, unmeasured limitation**, not a
result. `TestJournalMode` honours `RELA_SPIKE_DB_DIR` and is ready to run if a
mount appears.

## Quality

- [x] Code follows project patterns — Tx shape from `pgstore/tx.go`, graph
query delegated to `graphquerynaive` exactly as fsstore does, `storeutil`
validators used as the oracle
- [x] Checked for DRY opportunities — one `querier` seam serves both pooled and
tx-bound execution so method bodies are not duplicated; `buildRelationQuery` is
shared by list and count so filter semantics cannot drift
- [x] No security issues introduced — spike takes no untrusted input;
parameterized SQL throughout; no credential handling (SQLite has no DSN secret)
- [x] No silent failures — unimplemented methods return `errUnsupported`;
`verifyBusyTimeout` converts a silent misconfiguration into a startup error
- [x] No debug code left behind
- [x] ~~Shipping code reviewed for production readiness~~ (N/A: throwaway
spike, build-tag gated, never merged; default build verified unaffected via `go
build ./...`)
