---
id: BUG-JQO2PH
type: bug
title: pgstore change-feed listener fails when RELA_DATABASE_URL sets pool_max_conns
description: The listener connects with pgx.Connect on the full DSN; Postgres rejects pool_max_conns as an unknown runtime parameter, so the cross-process change feed is silently disabled.
priority: high
status: backlog
---

## Summary

With `pool_max_conns` in `RELA_DATABASE_URL`, the pgstore change-feed listener
cannot connect. It calls `pgx.Connect(ctx, dsn)` with the full DSN
(`internal/store/pgstore/listener.go`, `startListener` and `reconnect`).
`pgx.Connect` does not know the pool-only parameters, so it forwards them to the
server as runtime parameters, and Postgres rejects the startup:

```
FATAL: unrecognized configuration parameter "pool_max_conns" (SQLSTATE 42704)
```

The store then degrades with a single warning: "cross-process change feed
unavailable; writes from other processes won't be observed live". Other nodes'
writes are no longer seen live.

## Impact

`pool_max_conns` is the documented way to size the pool, and the mitigation the
BUG-9TGOH1 report suggested. Setting it silently disables the cross-process
change feed in a multi-node deployment.

## Reproduction

Start `rela-server-postgres` with
`RELA_DATABASE_URL=postgres://...?...&pool_max_conns=2` and read the startup
log. Observed during BUG-9TGOH1 verification (2026-10-01).

## Likely fix

Strip the pgxpool-only parameters (`pool_*`) before `pgx.Connect`, or parse with
`pgxpool.ParseConfig` and connect with its `ConnConfig`. Check any other direct
`pgx.Connect` on the same DSN.
