---
id: TKT-CPCBR7
type: ticket
title: Migration lock as a pluggable mini-service (postgres advisory lock, fs lock file)
kind: enhancement
priority: medium
effort: m
status: review
---

## Problem

The data-migration system (TKT-0C57FS) serializes nothing across processes:
`state.KV` has no compare-and-swap, so the applied-state marker and drift ledger
are last-write-wins. A gate adoption racing a `migrate data --apply`, or two
concurrent `migrate data` runs against the same postgres schema, degrade safely
today (idempotent steps, Resolve skips reached shapes — see RR-H16NMN) but can
lose an `Applied` entry, double-run steps, or interleave GC deletions with a
migration. RR-H16NMN's resolution explicitly names a cross-process lock as the
documented upgrade once multi-writer postgres deployments migrate under live
traffic routinely.

## Approach: a lock mini-service with per-backend implementations

Not one lock — a small service seam with different implementations, chosen at
the wiring site the way `state.KV` already is (`stateKVFor`: backend-provided
capability wins, filesystem fallback).

**Consumer-side interface** (declared in `internal/datamigration`, per the
consumer-side-interfaces rule):

```go
// MigrationLock serializes the writers of the migration marker/ledger and
// the destructive GC path across processes sharing one store.
type MigrationLock interface {
    // TryAcquire returns a release func, or ErrLockHeld naming the holder
    // context when another migration/GC run is active. Never blocks long —
    // an operator command should fail fast with "another migration is
    // running", not hang.
    TryAcquire(ctx context.Context) (release func(), err error)
}
```

**Implementations:**

- **postgres**: `pg_try_advisory_lock` on a dedicated key. Two hard-won rules from existing pgstore locks apply:
  - schema-qualify the key (`hashtext(current_schema())` mixed in) — the un-qualified-lock mistake is BUG-CA3VY0;
  - session-scoped advisory locks must live on ONE held pool connection for the whole critical section (the version-sweep precedent: issuing them through the pool silently voids the guarantee).
Consider sharing/coordinating with `sweepAdvisoryLockKey` so a migration
excludes a sweep tick outright.
- **filesystem**: lock file under `.rela/` (`O_CREAT|O_EXCL` with pid+timestamp payload, stale-lock detection on crash) — single-machine by nature, which matches fsstore's single-writer reality.
- **memory**: process-local mutex (tests, memorybackend).

**Wiring** (`internal/appbuild`): a `migrationLockFor(store, paths)` helper
mirroring `stateKVFor` — postgres capability wins, fs lock file otherwise.
Handed to the three writer call sites:

- `datamigration.Runner.Run` with apply=true (whole run under the lock; dry-run lock-free)
- `datamigration.GC.Tick` with apply=true (and `Scan`, which writes the ledger)
- gate adoption writes (`Gate.Evaluate` when it would move the marker) — optional: adoption races are content-identical between gates; the lock only matters for gate-vs-runner, so acquiring with a short timeout and proceeding on failure (log) may be the right posture here to keep startup non-blocking.

## Non-goals

- NOT a generic distributed-lock or unit-of-work abstraction (DEC-8UIL0: the one transaction seam is `store.Store.Tx`; this is an operational mutual-exclusion primitive like the sweep's advisory lock, scoped to data migration).
- No blocking waits in servers: startup and sweep ticks skip-and-log on contention; only the operator CLI may retry.
- Does not attempt marker CAS semantics — the lock makes the existing read-modify-write safe instead.

## Acceptance sketch

1. Two concurrent `migrate data --apply` against one pg schema: second fails fast with "another migration is running"; marker/Applied never loses an entry (pg-gated test, two connections).
2. GC apply and migration apply are mutually exclusive on pg (lock contention test).
3. fs: second concurrent apply on the same project dir fails fast; a stale lock from a killed process is detected and broken with a warning.
4. Dry-runs and read paths never touch the lock.
5. Wiring follows the stateKVFor pattern; memorybackend/tests get the in-process implementation; no store interface changes (capability type-assert or recipe-supplied, like the version service).

## Context

- RR-H16NMN (TKT-0C57FS code review) documents today's race analysis and why it currently degrades safely.
- `SaveMarker`'s godoc documents last-write-wins and points at this upgrade.
- BUG-CA3VY0 (unqualified advisory locks) is the trap the pg implementation must not repeat.
