---
id: TKT-1U8XYN
type: ticket
title: 'PostgreSQL read-path performance audit: per-request query counting, seeded demo with ACL + worlds, slow-query driven optimisation'
kind: enhancement
priority: high
effort: l
status: done
---

## Problem

The data-entry SPA drives `pgstore` through store methods designed around the
filesystem store. Nothing today tells us how many SQL statements a single page
load issues, and the seeded projects used for testing are small enough that N+1
patterns and full-body loads never show up as latency.

## Requested (2026-09-04)

1. Examine all code paths, especially those triggered by the data-entry SPA, for performance issues on PostgreSQL.
2. Fill a demo app with realistic content and inspect the slow-query log.
3. Add query-counting logic so each request reports how many queries it issued.
4. Optimise each hot path: drop the markdown body from listings, add automated indexes, batch per-row loads.
5. Set up a complex demo (or several) including ACL and worlds/faces.

## Related backlog (overlapping, do not duplicate)

- TKT-AIEGHU server-side aggregation for count/breakdown cards (still open; rows are now content-free, so a count card ships 3.5 MB instead of 53 MB at 11k tasks — the aggregate endpoint remains the real fix)
- TKT-8AUD1U dashboard table cards: push limit and sort into the query (still open; `store.GraphQuery.OrderBy/Limit/Offset` now exist to build on)
- TKT-T0DK37 bound the structured search path (still open)
- TKT-ZZL53L drop or use `entities_search_tsv_idx` — **done here** (migration 0014 drops it)
- TKT-L59KUM push field-visibility match provenance into SQL (still open)
- TKT-1ESTYJ (done) introduced `ListEntityHeaders`; this ticket moved lists, search, kanban, scope navigation, neighbour gating and the principal lookup onto headers

## What shipped

1. **Per-request query accounting** — `store.QueryStats` on the request context, filled by the pgx tracer (always attached, no allocation when unused), reported as `Server-Timing: db;dur=<ms>;desc="<n> queries"` and one `request` slog record — only under `-verbose` (RR-64OR7D). `storetest.Counting` decorator + `querybudget_test.go` pin store-call counts at 10 vs 50 rows for list, view section and search.
2. **Perf demo** — `prototypes/perf/project` (8 types, policy draft/published + document en/nl faces, three worlds, ACL with team membership and `visible:` redaction, lists with relation columns/filters, views with table sections, two kanbans, dashboard, gantt, next actions) and `rela dev seed --scale N` (`internal/perfseed`, deterministic, attributed, audited, refuses a non-empty store). Scale 1 into local postgres: 19,117 rows, 47,549 relations, ~40 s.
3. **Batching** — `RelationQuery.EntityIDs` on all four backends; page edges in one query (world path: one per distinct face), neighbour gating over headers, section relation columns per section, view collection loads and traversal per frontier, search hits per face, relation filters per filter; `Declarative.AuthorizeWrite` reuses the request's `acl.Request` (was six membership walks per row); principal lookup pushed into a graph query.
4. **Content-free rows** — lists, search, kanban, scope navigation and neighbour heads read `EntityHeader`s (`store.GraphHeaderQueryer` + generic fallback); `include_content=true` reloads the served page only. Documented in the API reference and OpenAPI.
5. **Paging pushdown** — `GraphQuery.OrderBy/Limit/Offset` (naive + pg), `store.MatchedCounter`; `listpushdown.go` pushes simple list shapes (no free text, no relation filter, `=`/`!=` and sort on string-shaped properties); world resolution collapses to the default world for faceless types; ordering semantics unified (absent value = largest) across Go and SQL.
6. **Indexes and search** — migration 0014: `(type, id)` and id-prefix partial indexes, `entities_search_tsv_idx` dropped, `search_text` rebuilt as id/properties/body; ranking by similarity over the first 1 KB; derived `rela_derived_list__` indexes from list `sort:`/static `=` filters (EXPLAIN-tested); prefilter pushdown widened to enum/date/custom types.
7. **Next actions** — `SnoozedUntilMany`/`LastShownMany` on `userstate.Store` (mem/kv/pg + conformance); the engine judges all candidates in two statements.

## Baseline vs after (alice/manager, warmed second pass, scale 1)

| request | queries before → after | db ms before → after | bytes before → after |
|---|---|---|---|
| `_schema` / `_config` / `_dashboard` / `_sidebar` / `_commands` | 1–3 → 1–3 | 23–29 → 0.3–0.6 | – |
| `_next_action` | 45 → 7 | 188 → 12 | 241 → 241 |
| list tasks page (25 rows, sort due) | 207 → 8 | 180 → 3.1 | 110 KB → 12 KB |
| list open_tasks (filter + sort) | 207 → 8 | 175 → 15 | 139 KB → 14 KB |
| list projects / persons / policys / documents / risks / controls | 207 → 8 | 34–58 → 2–10 | 109–152 KB → 9–22 KB |
| kanban tasks page (100 rows) | 807 → 8 | 215 → 9.8 | 508 KB → 58 KB |
| kanban policys page (100 rows) | 807 → 8 | 61 → 13 | 531 KB → 44 KB |
| `_views/project/PRJ-0001` | 1208 → 42 | 95 → 23 | 99 KB → 99 KB |
| `_views/task` / `policy` / `person` | 33–61 → 23–29 | 13–17 → 3–8 | unchanged |
| GET task / task relations | 15 / 36 → 9 / 22 | 25 / 27 → 1.7 / 1.9 | unchanged |
| `_search q=type:task` (11k hits) | 66,004 → 4 | 3,219 → 41 | 53.5 MB → 3.5 MB |
| `_search q=type:task prop:status!=done` | 49,426 → 4 | 2,420 → 37 | 40 MB → 2.6 MB |
| `_search q=type:task prop:priority=critical prop:status!=done` | 12,346 → 4 | 712 → 10 | 10 MB → 0.65 MB |
| `_search q=telemetry` (free text, matches 59% of rows) | 7,004 → 5 | 5,072 → 1,233 | 1.9 MB → 0.29 MB |
| `_position` (task in list scope) | 4 → 4 | 158 → 40 | – |
| `_gantts/delivery` | 6 → 6 | 87 → 74 | 376 KB |
| list policys `?world=published` / `editorial` | 207 → 8 | 34–44 → 4–6 | 152 KB → 11 KB |
| `_views/policy/POL-0001?world=published` | 59 → 28 | 16 → 4 | unchanged |

Editor and reader principals track these numbers; a denied type is 5 queries and
empty.

## Left for follow-up tickets

- `_position` and the gantt still load a whole type (headers): position could rank inside the store; the gantt could load the forest by frontier from visible roots.
- Free-text search on a term present in most rows is bounded by the trigram similarity over the prefix (~1.2 s at 19k rows); a rarer term uses the trgm index.
- Views with `display: content` sections still load collected entities whole (needed for the body).
- The fs backend seeds slowly (one file + index write per row); the seeder documents using a small scale there.

Reproduce: seed as above, `rela-server-postgres -verbose`, run
`.ignored/perf/baseline.sh` twice and read the second pass.
