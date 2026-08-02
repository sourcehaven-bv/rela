---
id: BUG-CA3VY0
type: bug
title: pgstore migrate and write advisory locks are not schema-qualified
description: 'migrateAdvisoryLockKey and writeAdvisoryLockKey are bare compile-time constants, but PostgreSQL advisory locks are database-wide. Two rela schemas in one database therefore serialize their startup migrations, and every write transaction, against each other. The version-sweep lock had the same defect with a worse symptom (silent version-capture loss) and was fixed separately by #1217 (TKT-SCXHUL); these two remained.'
priority: high
why1: Two rela schemas in one database block each other's startup migrations and write transactions despite being independent deployments.
why2: migrateAdvisoryLockKey and writeAdvisoryLockKey are bare constants, while PostgreSQL advisory locks are database-wide — the keys carried no schema dimension.
why3: pgstore was designed single-schema-per-database. The change feed hit the same database-global problem with LISTEN/NOTIFY and schema-qualified its channel, but that reasoning was never generalised to the advisory locks.
why4: 'When the sweep lock was fixed under #1217, the fix was scoped to the lock that caused the observed e2e starvation; the migrate and write locks share the root cause but produce blocking rather than data loss, so nothing surfaced them.'
why5: No test exercised two schemas contending on the same database for the migrate or write paths, so the shared-key assumption was never falsifiable.
prevention: All three advisory locks now use the two-key form pg_advisory_xact_lock(key, hashtext(current_schema())). Pinned by TestMigrateDoesNotBlockAnotherSchema and TestWriteTxDoesNotBlockAnotherSchema, which drive the REAL Migrate/Tx code paths and hold BOTH the scoped and legacy key spaces on the blocking schema — without holding the legacy space a regression to the bare key would take a lock nobody holds and pass. Verified fail-before/pass-after. The sweep's silent lost-lock branch now escalates to a warning after 10 consecutive skips.
status: done
---

## Summary

`migrateAdvisoryLockKey` (`internal/store/pgstore/migrate.go`) and
`writeAdvisoryLockKey` (`internal/store/pgstore/tx.go`) are bare compile-time
constants. **PostgreSQL advisory locks are database-wide, not schema-scoped**,
so two rela schemas sharing one database contend on both.

This is the tail of a defect whose worst instance was already fixed. PR #1217
(TKT-SCXHUL) schema-scoped the **version-sweep** lock after postgres e2e workers
starved each other; its symptom was severe — the sweep acquires with
non-blocking `pg_try_advisory_lock` and silently `return nil`s on failure, so
the losing schema's create/update version capture was dropped with no error. The
migrate and write locks were left on bare keys.

## Impact

Blocking, not data loss — which is why these outlived the sweep fix:

- **Write Tx** (`tx.go`) — every write transaction serializes against writes to
*unrelated* schemas on the same database. On a shared-database deployment this
is a throughput ceiling that scales with the number of unrelated tenants.
- **Migrate** (`migrate.go`) — `pgstore.Open` calls `Migrate` on every store
open, so every starting process is a migrator. Two schemas' startups queue
behind each other. Blocking (`pg_advisory_xact_lock`), so correct but slow.

## Fix

Adopt the same two-key form #1217 used, rather than a competing mechanism:

```sql
pg_advisory_xact_lock($1::int, hashtext(current_schema()))
```

One idiom across all three locks. `hashtext` is Postgres-native, needs no
Go-side schema resolution, and adds no round-trip.

For migrate specifically, `current_schema()` is the right anchor: unqualified
DDL lands there, so it is by definition the schema the migration targets.

## Also in this change

The sweep's lost-lock branch was silent (`return nil` with no log). That silence
is what let the sweep half of this go unnoticed until e2e workers collided. It
now counts consecutive skips and escalates from debug to a **warning** after 10,
stating that this schema's versions are not being captured. Still non-fatal —
same-schema multi-process contention is the legitimate case the lock exists for.

## Test design note

The obvious test — hold the scoped lock on schema A, assert schema B proceeds —
**does not falsify**. Postgres treats the one-key and two-key advisory spaces as
disjoint, so a regression to the bare key would take a lock nobody holds and
pass. The tests therefore hold **both** key spaces on schema A, and drive the
real `Migrate` / `Store.Tx` code paths rather than mirroring the SQL literal.
Confirmed by patching production back to the bare key: both tests fail with
"blocked on schema A's lock", and pass once restored.

## Origin

Found while researching multi-app rela-server (RES-S8CH9C), where
schema-per-project makes this systematic. Independent of that work.
