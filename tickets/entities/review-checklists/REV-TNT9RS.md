---
id: REV-TNT9RS
type: review-checklist
title: 'Review: tenant resolution spine — org_id -> DSN -> Services, fail closed'
status: done
---

## Automated Checks

- [x] `just test` — full suite passes. `internal/tenant` at 85.8% statement coverage.
- [x] `just test -race` on `internal/tenant` — passes, including the concurrent
  acquisition test that exists specifically to be run under `-race`.
- [x] **`just test-postgres` equivalent against a LIVE PostgreSQL** — run as
  `RELA_TEST_DATABASE_URL=... RELA_TEST_DATABASE_REQUIRED=1 go test -race -tags
  postgres ./internal/tenant/`. All three isolation tests pass. `REQUIRED=1` is
  set deliberately so a missing database would fail rather than skip, since a
  skip and a pass are indistinguishable in an exit code.
- [x] `just lint` — clean for `internal/tenant`. Two `//nolint:contextcheck`
  directives, both matching the existing precedent at
  `internal/docscapture/server.go:90` ("teardown is not request-scoped; Close
  takes no ctx") and each carrying its own reason.
- [x] `just arch-lint` — OK, no warnings. New `tenant` component declared,
  depending on `appbuild` only.
- [x] `just plimsoll` — clean; no new type approaches a load line.
- [x] `just comment-lint` — clean across 11116 comments.
- [x] Build-tag isolation re-verified: `go list -deps ./cmd/rela-server` and
  `./cmd/rela` contain no `jackc/pgx`. This is why the DSN derivation is split
  across `dsn_postgres.go` / `dsn_nopostgres.go` rather than importing pgx from
  `config.go`.

## Manual Review

- [x] **The isolation test was verified to be capable of failing.** A passing
  security test proves nothing unless it can fail, so the test was deliberately
  sabotaged (both tenants pointed at one DSN, simulating a `search_path`
  derivation that silently did nothing) and it failed with:

  ```text
  --- FAIL: TestTenantIsolation_OneTenantCannotReadAnother (0.51s)
      isolation_postgres_test.go:125: CROSS-TENANT LEAK: tenant A read foreign
      entity TKT-B1 (map[title:tenant B confidential]); search_path is not
      isolating these tenants
  ```

  The sabotage was then reverted and the suite re-run green. This is the single
  most important line in this checklist: the centrepiece assertion is live.

- [x] Fail-closed reviewed on every path. Unknown org, empty org, open failure,
  and an opener returning `(nil, nil)` each yield an error and a nil lease.
  Pinned by `TestMapResolver_FailsClosed`,
  `TestRegistry_UnknownTenantGetsNoStore`,
  `TestRegistry_OpenFailureYieldsNoLease`, `TestRegistry_NilServicesRefused`,
  and `TestTenantIsolation_UnknownOrgReachesNoDatabase`.
- [x] No zero value can mean "allowed". The registry refuses a nil `*Services`
  before it can enter the resident set, which is the only route by which one
  could reach a request.
- [x] Use-after-close reviewed. Eviction of a referenced store is deferred to
  its last release; `TestRegistry_EvictionWaitsForInFlightUse` pins it, and
  `TestRegistry_DoubleReleaseIsSafe` pins that a defer-plus-explicit release
  cannot drop another holder's reference.
- [x] Duplicate-schema detection reviewed — this is the check for the failure
  that no test downstream would catch, because two orgs sharing a schema
  produces a system where every layer behaves exactly as designed.
- [x] Schema-name validation reviewed against the regex RES-D54281 specifies.
  Rejects uppercase, leading digits, hyphens, embedded quotes, over-length, and
  a name containing a `search_path` separator (`public,tenant_b`) — the last
  being the one that would matter once provisioning derives a name from a claim.
- [x] Shared-base non-mutation is inherited, not re-implemented; already pinned
  upstream by `TestSharedBase_AssemblyDoesNotMutateSharedValues`.
- [x] ~~SPA / frontend review~~ (N/A: no frontend surface; the registry is not
  yet mounted in any HTTP path).
- [x] ~~Migration review~~ (N/A: no schema changes; this ticket adds no SQL and
  `pgstore` is untouched, which was re-verified as still true.)

## Verification

- [x] **Scope honesty.** The registry is NOT mounted in `rela-server`. Nothing
  in the shipping binaries changes behaviour, and tenant isolation is therefore
  not live in any deployment. This is stated in the ticket, in the package doc,
  and in the PR body rather than left for a reader to infer.
- [x] Deferred work is filed, not half-built: provisioning is TKT-TNPRV8 and
  erasure is TKT-TNERAS. No `CREATE SCHEMA` or `DROP SCHEMA` is reachable from
  this code.
- [x] The known pre-existing flake
  (`TestMemoryQueue_Conformance/IdempotencyKeyFreedAfterCompletion`, arrived
  with #1444) was not observed in this run. It is unrelated to this change.
