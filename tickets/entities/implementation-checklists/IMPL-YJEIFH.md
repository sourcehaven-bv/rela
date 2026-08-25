---
id: IMPL-YJEIFH
type: implementation-checklist
title: 'Implementation: Postgres job queue cannot initialize against a schema-pinned DSN'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code
- [x] Integration tests written (test full flow, not just units)
- [x] Happy path implemented
- [x] Edge cases from planning handled
- [x] Error handling in place (errors surfaced, not swallowed)

Three changes:

1. **Vendored neoq fork** — `sourcehaven-bv/neoq`, branch
   `fix/schema-qualified-sequence` off `v0.72.1`, pinned by a documented
   `replace` in `go.mod`. The `make-job-id-bigint` migration now resolves the
   sequence with `pg_get_serial_sequence('neoq_jobs', 'id')` inside a `DO`
   block instead of naming `public.neoq_jobs_id_seq`. Offered upstream as
   acaloiaro/neoq#149; the `replace` comment points at that PR and says to
   drop it when the fix lands.
2. **`TestPostgresQueue_SchemaPinnedDSN`** (`internal/jobs/pgqueue_test.go`).
3. **Fixture diagnostics** (`e2e/tests/fixtures.ts`) — `spawnServer` handles
   the `'error'` event, races startup against child exit, and puts the child's
   stderr in the thrown error.

Error handling is the point of change 3: the failure was fully diagnosed by the
server *and* captured by the fixture, then thrown away because the fixture only
attached logs on a test failure, not a fixture-setup failure.

## Test Quality

- [x] Using fixture builders or factories for test data
- [x] No hardcoded values in assertions when object is in scope
- [x] Only specifying values that matter for the test
- [x] Interpolated values constructed from objects
- [x] Property comparisons use original object, not hardcoded strings

The regression test creates its own schema and derives the pinned DSN from
`RELA_TEST_DATABASE_URL` by parsing and re-serializing it, rather than
string-concatenating a `?options=` — the same approach `pgstore`'s
`dsnForSchema` uses. Schema identifiers go through `pgx.Identifier.Sanitize()`.

## Manual Verification

- [x] Feature manually tested end-to-end
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified

**Reproduction first.** Confirmed the failure before changing anything: built
`bin/rela-server-postgres` at the branch head, ran `history-url-params.spec.ts`
against a local PostgreSQL — 6 failed with the reported `spawn ... ENOENT`.

**Root cause, not symptom.** Instrumented `spawnServer` with a temporary
`exit`/`error` logger, which surfaced the real error:
`relation "public.neoq_jobs_id_seq" does not exist`. Reduced it out of Playwright
entirely into a plain Go test against a hand-made schema, which failed the same
way — so this is a product bug, not an e2e artifact.

**The SQL was verified in psql before it was committed**, both ways: in a pinned
schema (sequence ends up `bigint` in that schema) and in `public` (unchanged
behaviour, so the upstream path does not regress).

**AC 1 — both specs pass with a database configured.** `history-url-params`
(6) + `relation-history` (2): **8 passed**. Full suite: **265/265 passed**
(2.6m), with `RELA_E2E_DATABASE_URL` set.

**AC 2 — the failure mode reports the real error.** Verified while the bug was
still live: the fixture now throws `server exited during startup (code 1,
signal null)` followed by the server's stderr, instead of a 30s timeout plus a
bare ENOENT about a binary that exists and runs.

**Go suites.** `go test -tags postgres ./internal/jobs/... ./internal/appbuild/...`
all ok, including the postgres job-queue conformance suite (125s) against a real
database.

## Quality

- [x] Code follows project patterns (check similar code)
- [x] Checked for DRY opportunities
- [x] No security issues introduced
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind

The regression test follows the schema-pinning pattern already used by
`pgstore`'s `listener_test.go` and `dataentry`'s `store_bridge_postgres_test.go`.

**Rejected alternatives**, both recorded in the ticket: sharing one
`public.neoq_jobs` across tenants (rela uses a single queue name and neoq's
trigger does `pg_notify(NEW.queue)`, so tenants would consume each other's
jobs — a cross-tenant leak), and a `public.neoq_jobs_id_seq` shim (puts one
tenant's object in the shared schema to satisfy another's migration).

Temporary instrumentation and the throwaway probe test were removed, not
committed — verified with `git status`.
