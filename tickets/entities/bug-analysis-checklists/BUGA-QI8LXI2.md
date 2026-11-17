---
id: BUGA-QI8LXI2
type: bug-analysis-checklist
title: 'Analysis: Server fails to start when RELA_DATABASE_URL sets pool_max_conns'
started: "2026-11-16"
completed: "2026-11-16"
status: done
---

## Reproduction

- [x] Bug reproduced locally: `RELA_DATABASE_URL='postgres://.../db?pool_max_conns=10' rela-server-postgres` fails with `unrecognized configuration parameter "pool_max_conns" (SQLSTATE 42704)`.
- [x] Minimal reproduction steps documented (in the bug body).
- [x] Environment/conditions noted: postgres build, any DSN with a `pool_*` key, startup guard `tenant.RefuseTenantDatabase` and the other tenant `pgx.Connect` call sites.

## Root Cause

- [x] Immediate cause identified (why1): `pgx.Connect` forwards unknown `pool_*` keys as runtime parameters.
- [x] Contributing factors found (why2-3): every direct-connect site called `pgx.Connect` itself; the BUG-JQO2PH fix stayed local to the listener.
- [x] Systemic cause explored (why4-5): no shared helper and no guard against new `pgx.Connect` calls.

## Fix Planning

- [x] Fix approach determined: one helper `pgstore.ConnectDirect` parses with `pgxpool.ParseConfig` and connects with its `ConnConfig`; every direct connection uses it.
- [x] Regression test planned: `TestNoDirectPgxConnect` (internal/archguard) with an empty allowlist; tenant postgres tests run with a pool DSN.
- [x] Related areas checked for similar issues: erase_postgres.go, provision_postgres.go, backendtest, docscapture; BUG-BR9CXF found in the same area and fixed in the same commit.
