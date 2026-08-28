---
id: BUG-YJEIFH
type: bug
title: Postgres job queue cannot initialize against a schema-pinned DSN
description: neoq's make-job-id-bigint migration hardcodes public.neoq_jobs_id_seq while its tables are created through search_path, so NewPostgresQueue fails on any DSN that pins search_path to a non-public schema. That is how rela scopes a tenant and how the postgres e2e specs connect, so rela-server-postgres refused to start and 8 e2e tests failed with a misleading 'spawn ... ENOENT'. Introduced by TKT-YOED3R; fixed via a vendored neoq fork plus a schema-pinned regression test.
priority: high
status: done
why1: rela-server-postgres exited during startup, so the e2e fixture never got a listening server and every postgres-gated spec timed out setting up serverUrl.
why2: appbuild could not build the job queue - neoq's migration failed with 'relation "public.neoq_jobs_id_seq" does not exist'.
why3: neoq creates its tables through search_path (so they land in the tenant schema) but one migration names the sequence as public.neoq_jobs_id_seq, which only resolves when the tables happen to be in public.
why4: rela's own postgres conformance suite passes RELA_TEST_DATABASE_URL through unchanged, which resolves to public - so no test ever exercised the schema-pinned DSN that every tenant and the e2e fixture actually use.
why5: the queue was adopted as a dependency without checking it against rela's schema-per-tenant contract, and that contract is documented in prose (docs/postgres-backend.md) rather than pinned by a test that new storage-touching dependencies must pass.
prevention: internal/jobs now has TestPostgresQueue_SchemaPinnedDSN, which initializes the queue against a freshly-created non-public schema and asserts neoq_jobs lands there. Any dependency that writes to PostgreSQL must be exercised through a schema-pinned DSN, not just the bare test DSN.
---

## Symptom

Every postgres-gated e2e spec fails when `RELA_E2E_DATABASE_URL` is set:

- `e2e/tests/history-url-params.spec.ts` (6 tests)
- `e2e/tests/relation-history.spec.ts` (2 tests)

```text
Test timeout of 30000ms exceeded while setting up "serverUrl".
Error: spawn <root>/bin/rela-server-postgres ENOENT
```

## Root cause

The server never starts. Its stderr — which the fixture discarded — said:

```text
appbuild: build job queue: jobs: init postgres backend:
unable to run migrations, could not apply up migration: migration failed:
relation "public.neoq_jobs_id_seq" does not exist
  in: ALTER TABLE neoq_jobs ALTER COLUMN id SET DATA TYPE bigint;
      ALTER SEQUENCE public.neoq_jobs_id_seq AS bigint;
```

neoq's `CREATE TABLE neoq_jobs` is unqualified, so `SERIAL` mints
`<search_path_head>.neoq_jobs_id_seq`. The later `make-job-id-bigint`
migration then names `public.neoq_jobs_id_seq` explicitly. The two agree only
when the queue's tables are in `public`.

The e2e fixture connects with `-c search_path=<test schema>,public` (per-test
schema isolation), so they never agree there — and neither do they in a
schema-per-tenant deployment, which is the documented multi-tenant mode.

Sharing one `public.neoq_jobs` across tenants is not an acceptable workaround:
rela submits every kind to a single queue name and neoq's insert trigger does
`pg_notify(NEW.queue, ...)`, so tenants would consume each other's jobs.

## Why the ENOENT was misleading

`spawnServer` never registered a `'error'` listener on the child, so Node
re-raised the spawn-side event out of band while `waitForServer` polled a dead
port for the full 30s. The reported error was therefore both wrong (the binary
exists and runs) and late. The child's stderr was captured but only attached on
a *test* failure, not on a fixture-setup failure.

## Not pre-existing

Earlier triage concluded this reproduced on unmodified `develop`. It does not:
`internal/jobs` does not exist on `develop` (`git ls-tree origin/develop
internal/jobs` is empty), so `develop` has no job queue to fail. The comparison
run must have used a binary built from the branch.

## Fix

1. **Vendored neoq fork** (`sourcehaven-bv/neoq`, branch
   `fix/schema-qualified-sequence`, pinned by a `replace` in `go.mod`): the
   migration resolves the sequence with `pg_get_serial_sequence('neoq_jobs',
   'id')` instead of naming a schema. Correct in `public` and in any other
   schema. Offered upstream as acaloiaro/neoq#149; drop the `replace` when
   it lands.
2. **Regression test** `TestPostgresQueue_SchemaPinnedDSN` — initializes the
   queue against a fresh non-public schema and asserts `neoq_jobs` is created
   there.
3. **Fixture diagnostics** — `spawnServer` now handles `'error'`, fails fast
   when the child exits during startup, and puts the server's stderr in the
   thrown error.

## Acceptance

- [x] Both specs pass with `RELA_E2E_DATABASE_URL` set.
- [x] The failure mode reports the server's actual startup error.
