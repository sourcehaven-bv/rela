---
id: BUG-3U61TX
type: bug
title: pgxpool DSN params (pool_max_conns) are passed to the change-feed listener's raw connection, silently disabling cross-process events
description: pgstore.Open parses the DSN with pgxpool.ParseConfig (which consumes pool_* params) but hands the raw DSN string to startListener. PostgreSQL rejects pool_max_conns as an unrecognized runtime parameter, so the LISTEN/NOTIFY connection fails and cross-process change events degrade to a warning.
priority: medium
effort: xs
why1: Cross-process change events stop working whenever an operator tunes the pool via the DSN.
why2: startListener connects with the raw DSN string, which still carries pgxpool-only parameters.
why3: pgxpool.ParseConfig consumes pool_* keys into its own config; the original DSN string is never rewritten to strip them.
why4: The listener was added later and reused the DSN as passed in, rather than deriving its connection from the already-parsed pgxpool config.
why5: The failure is non-fatal by design (degrades with a warning), so no test covers a DSN carrying pool parameters and the regression is invisible in CI.
status: backlog
---

## Symptom

Setting a pool size in the DSN, e.g.

```
RELA_DATABASE_URL="postgres://user@host:5432/db?sslmode=disable&pool_max_conns=80"
```

produces this at startup:

```
level=WARN msg="pgstore: cross-process change feed unavailable;
  writes from other processes won't be observed live"
  error="failed to connect: server error: FATAL: unrecognized configuration
  parameter \"pool_max_conns\" (SQLSTATE 42704)"
```

The **pool itself is configured correctly** — `pgxpool.ParseConfig` consumes
`pool_max_conns` — but the change-feed listener fails. Observed during unrelated
load testing; the pool size took effect while cross-process events silently
stopped.

## Root cause

`internal/store/pgstore/open.go`. The DSN is parsed for the pool:

```go
cfg, err := pgxpool.ParseConfig(dsn)
...
pool, err := pgxpool.NewWithConfig(ctx, cfg)
```

but the **raw string** is handed to the listener:

```go
l, err := startListener(ctx, st, dsn)
```

`startListener` opens its own dedicated connection (deliberately — a slow
`LISTEN` must not starve query traffic). That connection goes through plain pgx,
which passes unrecognised keys to the server as runtime parameters.
`pool_max_conns` is a pgxpool-only concept, so PostgreSQL rejects it.

Any `pool_*` parameter triggers this: `pool_min_conns`,
`pool_max_conn_lifetime`, `pool_max_conn_idle_time`, `pool_health_check_period`.

## Impact

Low severity, poor failure mode. The store still works and local events still
fire, so a single-process deployment sees nothing wrong. In a multi-process
deployment — exactly where `LISTEN/NOTIFY` matters — cross-process writes stop
being observed live, and the only signal is one WARN line at startup. The
data-entry SSE feed (`App.startStoreEventBridge`) goes quiet for remote writes.

The trap is that tuning the pool is a *normal* operational action that silently
disables an unrelated subsystem.

## Fix direction

Derive the listener's connection from the already-parsed config rather than the
raw string — `cfg.ConnConfig` is a `*pgx.ConnConfig` with the pool keys already
stripped, so `pgx.ConnectConfig(ctx, cfg.ConnConfig.Copy())` is the natural
form. That also keeps the listener honest if pgxpool ever consumes further keys.

Whatever the mechanism, the listener must not receive parameters intended for
the pool.

Secondary consideration: a listener that cannot connect **because the DSN is
malformed** is arguably different from one that cannot connect because the
server is briefly unreachable. The first is a permanent configuration error and
might warrant failing loudly; the second correctly degrades. Worth an explicit
decision rather than treating all listener-start failures identically.

## Acceptance criteria

1. A DSN carrying `pool_max_conns` (or any `pool_*` key) starts the change-feed
listener successfully.
2. The pool still honours those parameters — the fix must not strip them from the
pool's own config.
3. Cross-process events are delivered with such a DSN, verified end to end.

## Test plan

- Regression test: construct a store with a `pool_*`-carrying DSN and assert the
listener starts (must fail on current `develop`).
- Assert the pool's effective `MaxConns` still reflects the DSN value.
- Existing DB-gated cross-process event tests, re-run with a pool-tuned DSN.
- Cover several `pool_*` keys, not just `pool_max_conns`.

## Notes

Found incidentally while load-testing the analyze OOM. Unrelated to those
defects, filed separately.
