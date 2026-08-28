---
id: RR-SCXP1
type: review-response
title: Version-sweep advisory lock was database-global, starving parallel-schema sweeps
finding: 'The version-reconciliation sweep gates on pg_try_advisory_lock(sweepAdvisoryLockKey) with a constant key, but PostgreSQL advisory locks are database-GLOBAL. Many schemas run on one database (conformance harness + the new postgres e2e''s isolated schemas). Two rela-server-postgres processes sweeping different schemas on one DB would starve each other — only one wins the lock per tick, the other captures nothing. Under CI parallel e2e workers this is intermittent red masked by retries.'
severity: critical
resolution: 'Fold hashtext(current_schema()) into the two-key form pg_try_advisory_lock($1::int, hashtext(current_schema())) (and the matching pg_advisory_unlock) so each schema''s version lock is independent while staying mutually exclusive WITHIN a schema. purge inherits it via the shared tryAdvisoryLock/advisoryUnlock. Regression test TestSweepAdvisoryLockIsSchemaScoped; postgres e2e now passes under 2 parallel workers (repeat x3).'
status: addressed
---
