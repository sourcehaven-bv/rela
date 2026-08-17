---
id: RR-QY5S4C
type: review-response
title: Reconcile at store-open needs a schema-scoped advisory lock (DDL-concurrency convention)
finding: Reconcile runs at every store-open with bare CREATE/DROP INDEX IF [NOT] EXISTS and no lock. IF EXISTS doesn't prevent a create/drop race across N booting processes with differing metamodels. Take pg_advisory_xact_lock(reconcileKey, hashtext(current_schema())) — a new key — around the whole plan→apply, matching the migrate/sweep/write convention.
severity: significant
resolution: Reconcile acquires pg_advisory_lock(reconcileAdvisoryLockKey=0x52454c44 'RELD', hashtext(current_schema())) on a single pooled connection for the whole plan→apply, released on return. New key distinct from migrate/write/sweep. Session-scoped (not xact) because each DDL runs in its own implicit tx so one failed CREATE doesn't abort the batch.
status: addressed
---

Every concurrent-DDL path in pgstore takes a schema-scoped advisory lock:
migrate uses `pg_advisory_xact_lock(migrateAdvisoryLockKey,
hashtext(current_schema()))` (migrate.go:66); sweep/purge use
sweepAdvisoryLockKey; writes use writeAdvisoryLockKey. The plan runs reconcile
at EVERY store-open with bare CREATE INDEX IF NOT EXISTS / DROP INDEX IF EXISTS
and NO lock. IF [NOT] EXISTS prevents duplicate-name errors but NOT a
create/drop race: process A (new metamodel) creates rela_derived_uniq__X while
process B (old metamodel no longer declaring X) drops it → nondeterministic with
add+drop in one loop across N booting processes.

REQUIRED: take `pg_advisory_xact_lock(reconcileKey, hashtext(current_schema()))`
(a NEW key, distinct from migrate/sweep/write) around the whole plan→apply. Note
tension with CONCURRENTLY (finding on lock hazard) — CONCURRENTLY can't run in a
tx.
