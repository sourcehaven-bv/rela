---
id: RR-U99GDV
type: review-response
title: pg_advisory_lock two-key form cast to ::bigint does not resolve — postgres backend fails on first acquire
finding: |-
    internal/store/pgstore/keyedlock.go cast the class key as `$1::bigint` in both the lock and unlock statements:

      SELECT pg_advisory_lock($1::bigint, hashtext(current_schema() || '/' || $2::text))

    PostgreSQL overloads pg_advisory_lock as (bigint) and (integer, integer) ONLY — there is no (bigint, bigint) two-argument form. hashtext() returns integer, so the call resolves to (bigint, integer), which does not exist. Verified against a live PostgreSQL 15:

      ERROR:  function pg_advisory_lock(bigint, integer) does not exist (SQLSTATE 42883)

    This is not a corner case: EVERY acquire on the postgres backend would fail, and the release path was broken identically (pg_advisory_unlock(bigint, integer) likewise does not exist). The cross-process tier — the entire reason the postgres backend exists — was non-functional as committed.

    The new code was the sole outlier in the repo. Every pre-existing advisory-lock call site uses `$1::int`: derivedschema.go:131, sweep.go:509 (tryAdvisoryLock/advisoryUnlock), migrate.go:66, tx.go:72, and both scope tests. The value itself (0x52454c4b = 1380273227) fits in int4, so ::int is correct and loses nothing.

    ROOT CAUSE (the systemic finding worth recording): the defect shipped because no test ever executed the SQL. The DB-gated tests in keyedlock_test.go called skipOrFailWithoutDSN(t) UNCONDITIONALLY rather than guarding on os.Getenv(testDBEnv) == "" first, the way every correct call site does (listener_test.go:27, or via testDSN()). Under a normal run they skipped; under RELA_TEST_DATABASE_REQUIRED=1 they hard-failed on the DSN check before ever reaching the store. Either way the SQL was never sent to a server. A skip and a pass are indistinguishable in go test exit codes — precisely the failure mode requireDBEnv (RR-0EWZQW) was introduced to close, defeated here by an incorrect call.
severity: critical
resolution: |-
    Fixed in 28ac61f0. Both statements now cast `$1::int`, matching every other advisory-lock call site in pgstore; a doc comment on keyedLockClassKey records the overload constraint so the cast is not 'tidied' back to bigint. The test gating was corrected to `_ = testDSN(t)`, which checks the env var before deciding to skip, so the suite actually runs when a DSN is present.

    Verified against a live PostgreSQL 15 (not merely compiled): the full DB-gated suite passes with -race — TestKeyedLock_Conformance (all locktest cases including DistinctKeysDoNotContend, SameKeyExcludes, ContextCancelWhileWaiting, ReleaseIsIdempotent, SerializesConcurrent), plus ExclusiveAcrossStores, DistinctKeysDoNotContendAcrossStores, ScopedPerSchema and KeyedLockerFor_DiscoversCapability. Before the fix these same tests failed with 42883, so they now demonstrably exercise the SQL rather than skipping past it.
status: addressed
---
