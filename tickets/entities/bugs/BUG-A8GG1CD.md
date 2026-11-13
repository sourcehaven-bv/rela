---
id: BUG-A8GG1CD
type: bug
title: Server fails to start when RELA_DATABASE_URL sets pool_max_conns
description: tenant.RefuseTenantDatabase and other tenant pgx.Connect calls forward pool_max_conns to Postgres, which rejects the startup.
priority: high
status: backlog
---

## Summary

`rela-server` and `rela-docs` fail to start when `RELA_DATABASE_URL` carries a
pool-only parameter such as `pool_max_conns`:

```
FATAL: unrecognized configuration parameter "pool_max_conns" (SQLSTATE 42704)
```

The startup guard `tenant.RefuseTenantDatabase`
(`cmd/rela-server/tenantmode.go`, `cmd/rela-docs/main.go`) calls
`pgx.Connect(ctx, dsn)` on the full DSN. `pgx.Connect` does not know the
`pool_*` keys, so it forwards them to the server as runtime parameters, and
Postgres rejects the startup.

This is the same defect as BUG-JQO2PH (the pgstore listener), in other call
sites.

## Affected call sites

- `internal/tenant/erase_postgres.go`: `pinSchema`, `dropSchema`,
`RefuseTenantDatabase`, `CheckBareTarget` (all four `pgx.Connect` calls).
- `internal/tenant/provision_postgres.go`: `Provisioner.connect` and
`checkLanding`.
- `internal/appbuild/backendtest/backendtest_postgres.go`: `SchemaIn` (test
helper, same pattern).

## Fix idea

Strip the pool settings before connecting, as
`internal/docscapture/scratch_postgres.go` does: parse with
`pgxpool.ParseConfig` and connect with `pgx.ConnectConfig(ctx,
poolCfg.ConnConfig)`. Put this in one helper in `internal/tenant` so every
direct connection uses it.

## Reproduction

```bash
RELA_DATABASE_URL='postgres://.../db?pool_max_conns=10' rela-server-postgres
```

## Prevention

Extend the `pgstore-pool-dsn-direct-connect` measure, or add an archguard test,
so no non-test code calls `pgx.Connect` with a configured DSN.
