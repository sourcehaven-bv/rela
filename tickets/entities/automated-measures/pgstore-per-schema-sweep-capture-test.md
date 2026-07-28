---
id: pgstore-per-schema-sweep-capture-test
type: automated-measure
title: 'Test: migrate and write Tx on one schema are not blocked by another schema''s lock'
description: Drives the real Migrate and Store.Tx code paths while another schema holds BOTH the scoped and legacy advisory key spaces, asserting the work completes rather than blocking. Holding the legacy space is what makes it falsifying — Postgres treats one-key and two-key advisory locks as disjoint, so a regression to a bare key would otherwise take a lock nobody holds and pass.
kind: test
location: internal/store/pgstore/advisorylock_scope_test.go (DB-gated on RELA_TEST_DATABASE_URL; runs under `just test-postgres`)
status: active
---

## What it pins

Two rela schemas in one PostgreSQL database must migrate, and run write
transactions, independently. PostgreSQL advisory locks are database-wide, so a
bare key makes unrelated schemas serialize against each other.

## The falsifiability trap this measure exists to avoid

The obvious test — hold schema A's *scoped* lock, assert schema B proceeds —
**passes even against the un-fixed code**. Postgres treats
`pg_advisory_xact_lock(k)` and `pg_advisory_xact_lock(k, s)` as disjoint lock
spaces, so a regression to the bare one-key form takes a lock nobody is holding
and sails through.

The tests therefore hold **both** key spaces on the blocking schema, and drive
the real `pgstore.Migrate` / `Store.Tx` rather than re-stating the SQL literal.
A mirrored literal tests the test, not the code.

Verified by patching production back to the bare key: both fail with "blocked on
schema A's lock", and pass once restored.

## Tests

- `TestMigrateDoesNotBlockAnotherSchema` — real `Migrate` against schema B while
A holds both migrate key spaces; must complete inside 10s.
- `TestWriteTxDoesNotBlockAnotherSchema` — real `Store.Tx` write against B while
A holds both write key spaces.
- `TestXactAdvisoryLocksAreSchemaScoped` — lock-level table test over both
classes: independent across schemas, still mutually exclusive within one.
- `TestSweepCapturesWhileAnotherSchemaHoldsLock` — complements the upstream
lock-level sweep test (`TestSweepAdvisoryLockIsSchemaScoped`, added by #1217) by
asserting the observable consequence: **captured versions**, not lock
acquisition. The original symptom was silent, so the assertion has to be on the
data.

DB-gated on `RELA_TEST_DATABASE_URL` like the rest of the pgstore suite.
