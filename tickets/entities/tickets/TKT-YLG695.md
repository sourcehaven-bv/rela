---
id: TKT-YLG695
type: ticket
title: _nav_status and _nav_items probe local roles per row (slow on atlas postgres)
kind: enhancement
priority: high
effort: m
started: "2026-10-24"
completed: "2026-10-31"
status: done
---

## Description

`GET /api/v1/_nav_status` is slow in production (atlas, rela on the Postgres
backend). Measured from the access log over 12–19 Oct 2026: 3,844 requests over
1 s, 93 over 5 s, max 25.4 s. About 36 % of request time is in the database, so
most of it is spent in Go (computation, lock contention or waiting). On 21 Oct:
1,133 over 1 s, max 9.1 s. No single SQL statement crosses the 500 ms slow log.

The endpoint is called often: the SPA recounts on every route change and after
every `entity:changed` event, so its cost multiplies with users and writes.

Goals:

- Find out what it computes per request.
- Decide whether it can be cached, maintained incrementally or made cheaper.
- Reproduce locally with a realistically sized dataset and measure.
- Check for contention with the other slow endpoints in the same period
(`/api/v1/taken`, `/api/v1/_analyze`): shared locks, connection-pool exhaustion.

## Findings (2026-10-24)

Root cause: `navStatusRows` redacts every row of the list before it evaluates
the list `condition:` and each rule, without `primeVerdicts`. Each redaction
resolves the principal's local roles with one `GetRelation` per (role relation ×
member × ancestor) — the BUG-E2W1LS N+1 that `listPage` already avoids. Details
and plan in PLAN-KQCSH3.

## Measurements

Local copy of atlas-dev (restored production data, 155 `taak`, 95 open), atlas
`origin/main` config, rela develop at 47206052b, postgres 18, GOMAXPROCS=2,
`pool_max_conns=4` (atlas's effective default on a cx23). "Primed" is a
throwaway one-line `primeVerdicts` in `navStatusRows`, reverted.

| Case | Statements | Single request | Throughput (closed loop) |
| --- | --- | --- | --- |
| `_nav_status`, develop | 867 (858 `GetRelation`) | 47 ms | ~48 req/s |
| `_nav_status`, primed | 10 | 11 ms | ~470–550 req/s |
| `_nav_status` at 20× tasks, develop | 16,833 | 770–810 ms | |
| `_nav_status` at 20× tasks, primed | 16 | 50–100 ms | |
| `/api/v1/taken?list_id=mijn_taken` | 14 | 5–7 ms | |
| `_nav_items?space=projecten` | 184 (168 `GetRelation`) | 35–45 ms | |
| `_analyze` | 11,234 (8,274 `GetRelation`) | 1.4–1.8 s | |

Open-loop mix for 20 s: `_nav_status` at R req/s, `mijn_taken` list at 5 req/s,
`_analyze` every 2 s.

| Build | R | nav p50 / max | taken p50 / max | analyze p50 / max |
| --- | --- | --- | --- | --- |
| develop | 20 | 47 / 571 ms | 7 / 67 ms | 1.48 / 2.30 s |
| develop | 40 | 523 / 1,460 ms | 14 / 38 ms | 6.35 / 9.40 s |
| primed | 20 | 6 / 45 ms | 8 / 45 ms | 1.40 / 1.81 s |
| primed | 40 | 5 / 51 ms | 7 / 44 ms | 1.45 / 1.65 s |

Pool effect, develop, R=40 plus `_analyze`: with pool 4, nav p50 412 ms and
analyze p50 5.2 s, 14 goroutines waiting in `pgxpool.Acquire`; with pool 16, nav
p50 62 ms, analyze p50 1.8 s, none waiting. `db_ms` excludes pool waits, which
explains why production shows most time "in Go".
