---
id: AM-pgstore-listener-tolerates-pool-dsn-params
type: automated-measure
title: The change-feed listener starts with a DSN carrying pgxpool-only pool_* parameters
description: A DSN containing pool_max_conns (and other pool_* keys) must start the LISTEN/NOTIFY listener successfully while the pool still honours the parameter.
kind: test
location: internal/store/pgstore/ (DB-gated test to be written with the fix)
status: proposed
---

Pins BUG-3U61TX.

Opening a store with a DSN carrying `pool_max_conns` (and the other pgxpool-only
keys: `pool_min_conns`, `pool_max_conn_lifetime`, `pool_max_conn_idle_time`,
`pool_health_check_period`) must:

1. Start the change-feed listener **successfully** — no
`FATAL: unrecognized configuration parameter` and no degradation warning.
2. Leave the pool's effective `MaxConns` reflecting the DSN value; the fix must not
strip the parameter from the pool's own config.
3. Deliver cross-process events end to end with such a DSN.

Cover **several** `pool_*` keys, not just `pool_max_conns` — the defect is
generic to any parameter `pgxpool.ParseConfig` consumes, and a single-key test
would let the others regress.

Must **fail on current `develop`**: today `startListener` receives the raw DSN
string and PostgreSQL rejects the pool key.

DB-gated on `RELA_TEST_DATABASE_URL`, like the rest of the pgstore suite.

This measure guards a *silent* failure: the store keeps working and local events
still fire, so without an explicit assertion the regression is invisible in CI —
which is why it went unnoticed in the first place (why5 on the bug).
