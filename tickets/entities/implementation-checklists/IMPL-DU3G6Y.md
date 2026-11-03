---
id: IMPL-DU3G6Y
type: implementation-checklist
title: 'Implementation: Atlas taken list costs thousands of queries per request and the SPA refetches it in bursts'
started: "2026-10-15"
completed: "2026-10-15"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code (localroleprime_test.go, rolerelationreach_test.go, querystats_test.go, TestQueryShape, TestRequestStats_QueryCeiling, visibleRefetch.test.ts, useLiveSubscription.test.ts)
- [x] Integration tests written (test full flow, not just units) (budget tests drive the list, search, view and include handlers under the realistic ACL; TestACLList_BatchedIncludesKeepGates)
- [x] Happy path implemented
- [x] Edge cases from planning handled (no Request or batcher on ctx, batch read error falls back to the walk, re-priming, a prime read by another Request, nil include map, hidden tab, 409 backoff)
- [x] Error handling in place (errors surfaced, not swallowed) (a failed prime returns ctx unchanged so the per-entity walk answers; an include fault is logged)

## Test Quality

- [x] Using fixture builders or factories for test data (newBudgetAppOn, randomPrimeGraph, primePolicy)
- [x] No hardcoded values in assertions when object is in scope
- [x] Only specifying values that matter for the test
- [x] Interpolated values constructed from objects, not hardcoded
- [x] Property comparisons use original object, not hardcoded strings

## Manual Verification

- [x] Feature manually tested end-to-end
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified

**Verification Evidence:**

Atlas data copied to postgres `atlas-perf`, rela-server-postgres with
`--access-log=stderr`:

| Request | Before | After |
|---|---|---|
| taken?per_page=100 | 1,814 queries | 15 queries, 35 ms |
| taken mijn_taken include=* | 4,026 queries | 21 queries, 38 ms |
| taken all_taken include=* world=actueel | 7,416 queries, 3.4 s | 20 queries, 40 ms |
| taken/{id}?include=* | | 22 queries, 18 ms |
| 70 concurrent all_taken | about 30 s each | 0.57 s total, max 0.37 s |

- The access log carries the query shape: `query="include=*&list_id=all_taken&page=1&per_page=100&world=actueel"`.
- The load warning fires on Atlas: `ACL role relation can never confer its role ... relation=toegewezen_aan`.
- With the prime disabled, the realistic-ACL sweep fails 8 budget tests (1,003 to 2,202 reads at 50 rows).
- 111 golden read counts dropped, one rose by 1 (`list-filter-hidden.stranger`: one batch query on an empty page); no golden body changed.

## Quality

- [x] Code follows project patterns (check similar code) (consumer-side `edgeBatcher`; ctx-scoped prime like PrimeTraversals; budget tests per collection-reads.md)
- [x] Checked for DRY opportunities (localRoles shared by the walk and the batch; one primeVerdicts hook for all 10 read paths)
- [x] No security issues introduced (prime is ctx-scoped and per Request, so writes never read it; the query shape logs config names only, with q, filter values, cursors and anchors masked)
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind
