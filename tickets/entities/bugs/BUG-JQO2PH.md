---
id: BUG-JQO2PH
type: bug
title: pgstore change-feed listener fails when RELA_DATABASE_URL sets pool_max_conns
description: The listener connects with pgx.Connect on the full DSN; Postgres rejects pool_max_conns as an unknown runtime parameter, so the cross-process change feed is silently disabled.
priority: high
effort: s
why1: The change-feed listener opens its dedicated connection with pgx.Connect(ctx, dsn). pgx.ParseConfig treats every key it does not know as a server runtime parameter, so pool_max_conns is sent in the startup packet and Postgres refuses the connection (SQLSTATE 42704).
why2: The pool and the listener parse the same DSN with different parsers. pgstore.NewPool uses pgxpool.ParseConfig, which removes the pool_* keys; the listener uses pgx.ParseConfig, which keeps them.
why3: pgstore.Open receives the DSN as a raw string and leaves every consumer to parse it itself, so nothing ties the listener to the parse the pool already did.
why4: The listener tests build their DSN by re-serializing host, port, user, dbname and search_path only, so no test ever passed a pool parameter through the listener.
why5: The startup failure is deliberately non-fatal and only logged once, so a misparsed DSN degrades to a running store without a change feed and nothing in CI or at startup turns it into a failure.
prevention: The listener parses its DSN with pgxpool.ParseConfig, the parser the pool uses, so one DSN string can no longer mean two things. TestListenerConnConfig_StripsPoolKeys runs without a database in the default test job, and TestCrossProcessPropagation_PoolTunedDSN runs the full feed on a pool-tuned DSN. The related DSN re-serialization defect is tracked as BUG-BR9CXF.
started: "2026-10-01"
completed: "2026-10-01"
status: done
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
writes are no longer seen live. The process also never publishes: the store
resolves its schema only when the listener starts, and `Store.notify` sends
nothing without one. Correctly configured peers then miss its writes until their
safety catch-up runs.

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
