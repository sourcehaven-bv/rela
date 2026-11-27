---
id: BUG-JQO2PH
type: bug
title: pgstore change-feed listener fails when RELA_DATABASE_URL sets pool_max_conns
description: The listener connects with pgx.Connect on the full DSN; Postgres rejects pool_max_conns as an unknown runtime parameter, so the cross-process change feed is silently disabled.
priority: high
why1: The change-feed listener could not connect when RELA_DATABASE_URL set pool_max_conns.
why2: It called pgx.Connect with the full DSN, which forwards pool-only parameters to the server, and the server refused them.
why3: The pool and the listener parsed the same DSN with different parsers, and no test used a pool-tuned DSN.
why4: The listener's connection was added later than the pool and reused the raw DSN without the pool's parsing.
why5: 'Systemic: two connection paths were configured from one string with no shared constructor, so a setting valid for one silently broke the other.'
prevention: The listener parses the DSN with pgxpool.ParseConfig, and the docscapture admin connection got the same fix (#1750).
started: "2026-11-27"
completed: "2026-11-27"
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
