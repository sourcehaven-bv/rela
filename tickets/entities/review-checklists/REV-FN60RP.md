---
id: REV-FN60RP
type: review-checklist
title: 'Review: SQLite store backend: conformance-passing minimal store behind a sqlite build tag'
status: done
---

<!-- @managed: claude-workflow v1 -->

PR #1421 · branch `feat/sqlite-backend-TKT-G91TBK`.

## Automated Checks

- [x] `go test ./internal/store/...` — all backends green, pgstore against a
live PostgreSQL
- [x] **`go test -race`** on sqlitestore — clean (16.7s)
- [x] `RunTxStressTest` at `RELA_STRESS_SECONDS=30` — passes post-fix
- [x] `golangci-lint run ./internal/store/...` — 0 issues
- [x] `just arch-lint` — clean; `just plimsoll` — clean
- [x] `GOOS=windows go build` — cross-compiles
- [x] Default and postgres builds verified NOT to link `modernc.org/sqlite`

## Code Review

- [x] Reviewed by the `cranky-code-reviewer` agent
- [x] All critical and significant findings addressed

**The review found three critical bugs, and two of them would have bricked a
user's database permanently.**

The transaction did not implement the tier it claimed. `Tx` registered `defer
conn.Close()` with **no deferred ROLLBACK**, so a panic in `fn` returned the
driver connection to the pool with `BEGIN IMMEDIATE` still open. Every later
transaction then failed with "cannot start a transaction within a transaction",
*and* the uncommitted write became durable and visible.

I reproduced the mechanism independently before fixing, in a standalone program
with no rela code: **10/10 subsequent connections poisoned, leaked row
visible.** The same hole existed on the `COMMIT` path via a cancelled context —
a closed browser tab mid-save — with no panic required.

What makes this worth recording: I had already applied `context.WithoutCancel`
to the ROLLBACK path *and written a comment explaining why*, reasoning about
"leaving an open transaction on a connection returning to the pool". I applied
that reasoning to the path that could not leave a transaction open, and not to
the one that could.

| ID | Severity | Status | Finding |
|----|----------|--------|---------|
| RR-LODAPD | critical | addressed | Panic in `fn` poisons the store and commits the rolled-back write |
| RR-BZBZXB | critical | addressed | Cancelled ctx at COMMIT poisons identically, no panic needed |
| RR-8DR44S | critical | addressed | Observers fire for rolled-back writes → phantom entity in the search index |
| RR-W4XTKE | significant | addressed | `HighestID` relied on LIKE's case-insensitivity by accident; `DeleteEntity` was four unsynchronized statements |
| RR-BUZ9QX | significant | addressed | Windows `lockFile` checked only one of two contention codes |
| RR-OYZR4P | minor | addressed | `isUniqueViolation` string-matched the driver's message |

**Every fix verified by deliberate regression.** Reverting all three criticals
makes the new tests fail with their intended messages:

```
--- FAIL: TestTxAbnormalExit/PanicInFnLeavesStoreUsable
    a panicking transaction must not commit its writes
--- FAIL: TestTxAbnormalExit/ContextCancelledDuringTxLeavesStoreUsable
    the store must remain usable after a cancelled Tx
--- FAIL: TestTxObserverIsolation/RollbackDoesNotNotifyObservers
    an observer must not see a write that was rolled back
```

**The tests went into `storetest`, not this package.** All three bugs lived in
the same blind spot: `RunTxRollbackTests` only ever returns an *error* from `fn`
— it never panics, never cancels, never inspects observers. A backend can pass
it while being permanently unusable after a panic. `RunTxAbnormalExitTests` and
`RunTxObserverIsolationTests` now hold every backend to it, which is the right
home given `Capabilities.TxRollback` is a *declaration* a backend makes about
itself.

The reviewer also confirmed several things are correct and I am not changing
them: keyset pagination at boundaries, the `` cursor separator (both validators
reject bytes < 0x20, so it cannot occur in a component), the cursor-predicate
composition with the parenthesized OR group, the sidecar lock design, and
`RenameEntity`'s type assertion.

## Acceptance Verification

| AC | Verdict | Evidence |
|----|---------|----------|
| 1 — `RunAll` | **PASS** | `{Attachments: true, TxRollback: true}` |
| 2 — stress 30s | **PASS** | no watchdog dump, no lost updates, pair atomicity |
| 3 — `RunTxRollbackTests` | **PASS** | plus the two new abnormal-exit suites the tier actually needs |
| 4 — six fuzz targets | **PASS** | |
| 5 — second opener refused | **PASS** | error names the holding pid |
| 6 — WAL guard | **PASS** | `Open` refuses non-WAL |
| 7 — CI isolation | **MOVED** | TKT-L1A3PH; the verifiable half holds today |
| 8 — arch-lint / lint / coverage | **PASS** | 0 issues, coverage 83.6% |

## Documentation

- [x] ~~User-facing docs~~ (N/A: nothing selectable until TKT-L1A3PH wires it)
- [x] The poisoning failure mode is documented **in `tx.go` where the mistake
would be repeated**, with the measured numbers — not in a commit message nobody
reads at the point of change
- [x] Follow-up filed — TKT-L1A3PH (`ready`) carries the wiring plus the
`cli/db_nonpostgres.go` seam and the three `!postgres` no-ops that need deciding
rather than inheriting
