---
id: TKT-T2I7P2
type: ticket
title: '/_search: client cancels are logged as failures and answered 500; measure and reduce per-keystroke search cost'
kind: enhancement
priority: medium
effort: m
started: "2026-10-24"
completed: "2026-10-31"
status: done
---

## Description

Production (atlas: rela-server-postgres behind pratique) reported, over 12–19
Oct 2026, on `GET /api/v1/_search`:

- 1,103 requests the client abandoned (rela logged `context canceled`);
- 408 requests pratique answered with a 5xx.

Response times are modest: on 19 Oct, 6 requests over 1 s, max 3.4 s, 86 % of
wall time in the database.

Hypothesis to verify, not assume: the UI searches per keystroke and aborts the
previous request, so cancellations are expected, and the 5xx are the proxy
reporting the canceled upstream.

Questions:

1. Frontend: is there a debounce, and is AbortController used?
2. Server: does a cancel stop the database query? Does a canceled
request produce an error log line or a 5xx?
3. Is the fix in the client (debounce), the server (cheaper search,
proper cancel handling, no error log for a cancel), or both?

Plan with measurements from a local reproduction.

## Findings (planning, 2026-10-24)

Reproduced locally: `rela dev seed --scale 1` (19,117 entities,
prototypes/perf/project, `default_world: published` like atlas's `actueel`),
rela-server-postgres on develop (4720605) and on v26.10.10 (what atlas ran 13–19
Oct), PostgreSQL 18, `--access-log=stderr`, `log_min_duration_statement=0`.
Scripts in `.ignored/repro/` (not committed).

### 1. Frontend

Palette, entity picker and mention menu debounce 150 ms and abort the previous
request (AbortController). RelationCards debounces 200 ms with no abort.
SearchView searches on Enter. No caller sends `limit`.

Typing simulation (`.ignored/repro/typesim`: same debounce + abort logic against
the real server; "gateway policy", 14 keys, 5 runs each, inter-key time ±50 %):

| mean key interval | debounce | sent | aborted | final result after last key |
|---|---|---|---|---|
| 200 ms | 150 ms | 46 | 41 | 380–730 ms |
| 200 ms | 300 ms | 5 | 0 | 505–644 ms |
| 280 ms | 150 ms | 63 | 54 | 380–470 ms |
| 280 ms | 300 ms | 27 | 22 | 560–710 ms |
| 280 ms | 400 ms | 9 | 4 | 620–775 ms |

### 2. What a cancel does on the server

- The database query stops: postgres logs "canceling statement due to
user request" for each abort. pgx then closes the pooled connection; one typing
run with 12 aborts opened 11 new backend sessions.
- Abort sweep (abort at 20..640 ms, 32 points, carol): develop 31× logged
`status=200` with no warning, 1× WARN "dataentry: list free-text search failed
err=free-text search: context canceled" + `status=500`. v26.10.10: 29× 200, 3×
WARN + 500. The 500 comes from `writeListPipelineError`'s default branch
(`loadHitHeaders`, and on 26.10.10 the per-hit field check, wrap the error
without ErrScope). So the "context canceled" count undercounts real cancels, and
the access log records most cancels as 200.

### 3. The 5xx are pratique's own, not a proxied cancel

Caddy (v2.11.4, embedded in pratique) maps a client cancel of a proxied request
to 499 and does not log it as "request failed". The perf report counts
pratique's `request failed ... status=5xx` lines, which only pratique's `fail()`
writes: from `failPage` on a `Resolver.Resolve` error (UserByID / OrgByID /
ActiveMembership on `r.Context()`) or a `MintFor` error, both 502, before the
request reaches rela. A cancel during that step therefore shows as a 502.
`ErrorPage` passes no `LogErr`, so the cause is not in the log. Code reading
only; not reproduced.

### 4. Cost per search (develop, seed scale 1)

| query | user | statements | db_ms | hits sent |
|---|---|---|---|---|
| gateway (9.5k matches) | alice | 9 | 620–670 | 1000 (366 KB) |
| gateway, `limit=50` | alice | 9 | ~650 | 50 (18 KB) |
| ga | alice | 9 | ~700 | 1000 |
| carol (1 match) | alice / carol | 4–5 | 197–318 | 1 |
| tsk-0367 (10 matches) | alice / carol | 5 | 225–274 | 10 |

Three of the statements are the ranking query (one per 500-row keyset page up to
the 1000 cap), each 180–210 ms. `limit` is applied after the search and saves no
database time. The ranking query applies the text LIKE after `DISTINCT ON (id)`,
so the trigram index is never used (Seq Scan + detoast of `search_text`, 42k
buffers).

SQL experiment (EXPLAIN ANALYZE, median of 3; same rows and order, checked by
hash):

| needle | today | prefilter, `id IN` + outer LIKE | matched-set CTE |
|---|---|---|---|
| ga | 200 ms | 281 ms | 246 ms |
| gat / gate | 197 ms | 287 ms | 197 ms |
| gateway | 207 ms | 349 ms | 194 ms |
| carol | 222 ms | 0.4 ms | 0.2 ms |
| tsk-0367 | 256 ms | 1.2 ms | 1.8 ms |
| zq9x (no hit) | 202 ms | 0.1 ms | 0.2 ms |

### Conclusion

Both sides. Client: a longer debounce and a `limit` remove most requests and
most of the payload. Server: a cancel must not log or answer 500 and should be
visible as such in the access log; the limit must reach the search; the
world-scoped search must use the trigram index. pratique and the devops perf
report need their own follow-ups (not in this repo).

## Result (implementation, 2026-10-31)

Same seed and scripts. The laptop was under heavy load during these runs (load
average 15–70), so absolute times are higher than in planning; each comparison
below ran the old and new binary back to back.

### Cancels

Abort sweep on the new binary (carol, "gateway", abort at 20..640 ms, 32
points): 30× `status=499 canceled=true`, 2× `status=200` (the response was
already written), 0 warnings. Postgres still logs "canceling statement due to
user request" for each abort; the pgx connection close is TKT-PVMWOZG.

### Typing

Typing simulation, "gateway policy", 3 runs each:

| mean key interval | debounce | sent | aborted |
|---|---|---|---|
| 200 ms | 150 ms (before) | 8–12 | 7–11 |
| 200 ms | 300 ms + `limit=50` | 1 | 0 |
| 280 ms | 300 ms + `limit=50` | 3–7 | 2–6 |

### Database time per search (`db_ms`, median of 5)

| user | query | params | before | after |
|---|---|---|---|---|
| carol | gateway | none | 540 | 850–990 |
| carol | gateway | `limit=50` | 540–1000 | 510–560 |
| carol | ga | `limit=50` | 560–990 | 330–430 |
| bob | gateway | `limit=50` | 720–1080 | 240–390 |
| bob | ga | `limit=50` | 760–770 | 180–250 |

The ranking query alone (EXPLAIN ANALYZE, median of 3, bob):

| needle | before | after, custom plan | after, generic plan |
|---|---|---|---|
| carol | 206 ms | 0.25 ms | 0.8 ms |
| tsk-0367 | 244 ms | 0.6 ms | 1.1 ms |
| gateway (9.5k matches) | 208 ms | 200 ms | 205 ms |

A selective term is about 200× cheaper. A term that most rows contain costs the
same per page, and without `limit` it reads up to three pages, as before;
without `limit` the after-time for such a term is higher than before in these
runs. All callers now send `limit` (palette and single-type picker 50, mention
menu 100); RelationCards and SearchView do not.

A first version of the CTE joined it with `(id, face) IN (SELECT ...)`. Under a
generic plan PostgreSQL estimated the CTE at a few rows and ran a nested-loop
semi join: 4.5 s for "gateway". The final version carries the matching faces as
an array; `TestWorldSearchExplain` fails on a semi join in either plan.
