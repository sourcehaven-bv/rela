---
id: BUG-E2W1LS
type: bug
title: Atlas taken list costs thousands of queries per request and the SPA refetches it in bursts
description: GET /api/v1/taken costs 1,800 to 7,400 queries per 100-row page (N+1 local-role probes and per-row include resolution), and the SPA refetches it from every tab at once on wake.
priority: high
effort: l
why1: 'Each list row runs 18 single-edge lookups (StoreGraph.HasEdge) to resolve local roles for its _actions map: 6 members (user + 5 roles) times 3 verbs. With include=* every included neighbour repeats this and the include itself costs 4 queries per row.'
why2: acl.Request.computeForEntity probes every (role relation x member x ancestor) per entity on every call. It has no per-entity memo within the request, no page-level batch and no check that the role relation can connect those endpoint types. The list include path calls resolveV1Includes per row instead of reusing the page edges it already loaded.
why3: The batching work of TKT-1U8XYN covered edges, neighbours and the global membership walk but treated local roles as cheap. That holds for policies without role_relations. Atlas added toegewezen_aan (wrong direction for the resolver) and grew to 5 roles per user, which multiplied the probes.
why4: The query budget tests that guard collection reads use a fixture policy with no role_relations and never request include=* on a list. A cost that only appears under a realistic ACL and the SPA's real query shape is invisible to them. Production records a query count per request but nothing alerts on it.
why5: 'Query cost is guarded only by hand-written per-path budget tests with hand-built fixtures. Nothing enforces a cost ceiling at runtime or derives the tested request shapes from what the SPA actually sends. The frontend also has no global policy for background refetches: no visibility gating and no jitter.'
prevention: '(1) Budget sweep test: drive every list/kanban/nav request shape (with include=* and world) over a fixture with a realistic acl.yaml (role_relations + groups + inherit_roles_through + several roles) and assert the query count is equal at 10 and 50 rows. (2) Per-request query ceiling in store.QueryStats: warn above a soft limit in production; fail the request in e2e/CI strict mode so any SPA flow over budget breaks the build. (3) N+1 detector: log the SQL shape when one statement shape repeats more than K times in one request. (4) ACL load warning for a role relation whose endpoint types can never connect a member to an entity. (5) Monitoring alert on the access log''s queries field.'
started: "2026-10-15"
completed: "2026-10-16"
status: done
---

## Description

`GET /api/v1/taken` in Atlas costs thousands of database queries per request,
and the SPA sends it in bursts. Reported from production: about 8,046 and 2,396
queries for two variants, normally 0.3 to 3 s each. When a laptop wakes, all
open tabs refetch in the same second, 60 to 75 requests pile up, each takes
about 30 s, and some return 500.

## Reproduction (local, postgres)

Copy of `atlas-dev` (151 taak, 4 persoon), this tree's `rela-server-postgres`,
Atlas config, `--access-log=stderr`, user `jvloothuis` (5 roles):

| Request | Queries | Wall |
|---|---|---|
| `taken?per_page=100` | 1,814 | 0.36 s |
| `taken?list_id=mijn_taken&per_page=100` | 1,694 | 0.18 s |
| `taken?list_id=mijn_taken&per_page=100&include=*` | 4,026 | 0.38 s |
| `taken?list_id=all_taken&per_page=100&include=*&world=actueel` | 7,416 | 3.4 s |

## Findings

1. **Per-row local-role probes.** 1,800 of the 1,814 queries are one statement: `SELECT … FROM relations WHERE from_id=$1 AND rel_type=$2 AND to_id=$3 AND from_face=$4`. `acl.Request.computeForEntity` (`internal/acl/resolver.go`) runs `StoreGraph.HasEdge` for every (role relation × member × ancestor) per entity, and the `_actions` affordance map calls it once per verb (`update`, `delete`, `rename`). With 6 members (user + 5 roles) that is 18 point lookups per row. Nothing caches the result per entity within a request, nothing batches across the page, and nothing skips a role relation whose schema endpoints cannot match.
2. **`include=*` on lists resolves per row.** The list handler loads all page edges in one batch (`servedFacePageEdges`), then calls `resolveV1Includes` once per row (`internal/dataentry/api_v1.go` list loop). That costs 4 queries per row, and each included neighbour is serialized with its own `_actions`, so finding 1 repeats for every neighbour of every row (a persoon referenced from 95 rows is probed 95 times). This is the jump from 1,814 to 7,416.
3. **Atlas policy confers nothing but costs everything.** `role_relations.toegewezen_aan` is probed as `member → toegewezen_aan → row`, but the schema allows only `bedrijfsmiddel → persoon`, so a persoon or role can never be the source. The relation never confers `toegewezen` (the acl.yaml comment describes the opposite direction), yet it triggers the probes on every type.
4. **Frontend bursts.** No 30 s timer exists. Refetches come from SSE `entity:changed` (invalidates every query of the type), SSE reconnect + replay, the live-subscription baseline refetch after every reconnect, and Pinia Colada defaults (`refetchOnWindowFocus`, `refetchOnReconnect`). Nothing checks `document.visibilityState`, and every backoff is deterministic with no jitter, so all tabs fire together on wake.
5. **Access log has no query string.** `requestStats` logs the route shape only, so the 8,046 and 2,396 variants of `/api/v1/taken` cannot be told apart in the log.

## Why the budget tests missed it

`querybudget_test.go` uses a policy without `role_relations` and never requests
`include=*` on a list. `faceverdict_budget_test.go` has a role relation but does
not cover list `_actions` or includes.

## Fix plan

Backend, in order of impact:

1. **Batch local roles per page.** Add `acl.Request.PrimeLocalRoles(ctx, ids)`. It runs ONE relation query (role-relation types × members × ids plus their `inherit_roles_through` ancestors) and fills a per-request cache that `computeForEntity` reads. The list handler, include serialization and kanban call it once per page. Also memoize `computeForEntity` per entity id within the request, so the three verbs share one result.
2. **Skip impossible probes.** At policy compile time, record each role relation's declared `from`/`to` types. `computeForEntity` skips a relation that cannot connect a member type to the entity's type. Warn at load when a conferring role relation cannot reach any member type (Atlas `toegewezen_aan`).
3. **List includes from the page batch.** Build `included` from the edges and `visibleNeighbors` that `servedFacePageEdges` already loaded: one header batch for the neighbours, one `primeVerdicts`, one `PrimeLocalRoles`. Nested includes (`a.b`) get one batch per level.
4. **Access log names the request.** Add `list_id`, `include` and `world` values plus the sorted query KEYS to the `request` record. These are config, not data. Never log `q` or filter values.

Frontend:

5. **Visibility gate.** One composable owns background refetches. While `document.visibilityState === 'hidden'` it records invalidations instead of refetching. On becoming visible it waits a random 0–5 s, then refetches once. SSE `entity:changed`, live-subscription baselines, nav/piles handlers and Pinia Colada focus/reconnect refetches all go through it.
6. **Jitter.** Full jitter on the SSE and live-subscription reconnect backoff.

Atlas (separate change in the atlas repo):

7. Fix or drop `role_relations.toegewezen_aan`; as written it confers nothing.

Regression tests:

- `storetest.Counting` budget tests: list with `include=*` and `world`, under a policy with role relations, groups and several roles. Assert an equal count at 10 and 50 rows; pin the constant.
- Unit test: `PrimeLocalRoles` returns the same attributions as the per-entity walk (differential over random graphs).
- Frontend: hidden tab does not refetch; visible again refetches once after the jittered delay.

## Prevention (how to catch and block this class)

1. **Budget sweep.** One table-driven test enumerates every request shape the SPA sends (each list, kanban, nav entry, with and without `include=*` and `world`) over a fixture with a realistic `acl.yaml`. It asserts a flat count at 10 vs 50 rows. New lists and params are covered without writing a new test.
2. **Runtime ceiling.** `store.QueryStats` gets a per-request ceiling. In production, crossing a soft limit logs a warning with route, list and count. In e2e and CI (strict mode) it fails the request, so a page that issues hundreds of queries breaks the e2e suite.
3. **N+1 detector.** In the same stats, count statement shapes; one shape repeated more than K times in one request is logged as a suspected N+1 with the SQL shape. The counting test store asserts the same.
4. **Policy lint.** Warn at `acl.yaml` load for role relations that can never confer.
5. **Monitoring.** Alert on access-log `queries` above a threshold in Atlas production.
