---
id: TKT-PV0WT3U
type: ticket
title: Raise rela's default pgx pool size on the postgres backend
kind: enhancement
priority: medium
effort: s
status: backlog
---

## Description

Without `pool_max_conns` in `RELA_DATABASE_URL`, pgxpool uses max(4, NumCPU)
connections. On a 2 vCPU host (atlas, cx23) that is 4. In TKT-YLG695, 4
connections were the bottleneck under concurrent sidebar and dashboard requests:
with the pool at 16, `_nav_status` p50 at 40 req/s fell from 412 ms to 62 ms and
`_analyze` p50 from 5.2 s to 1.8 s.

`db_ms` in the access log excludes time waiting for a connection, so pool
exhaustion shows up as Go time.

Decide a default (and whether to log pool waits), keep an explicit
`pool_max_conns` winning, and document it in docs/postgres-backend.md.
