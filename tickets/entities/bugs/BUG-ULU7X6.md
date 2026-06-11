---
id: BUG-ULU7X6
type: bug
title: LinearSearch returns a nondeterministic, arbitrarily-truncated result set
description: LinearSearch.Search ranged over its backing map (randomized Go iteration order) and broke mid-loop at the limit. So the same query returned a different ordering each call, and with a limit a different arbitrary subset. The Service wrapper always calls the backend with limit=0 then applies its own q.Limit over the returned order, so the unstable order still fed pagination; tests on the memorybackend build (where LinearSearch is the live backend) saw flakiness.
priority: medium
why1: Search appended IDs while ranging over a map and truncated with a mid-loop break at limit, so both order and limited-subset depended on Go's randomized map iteration.
why2: The backend had no relevance scoring and no fallback deterministic ordering, so 'collect then order' was never established.
why3: Truncation happened during iteration rather than after collecting all matches, conflating 'which match' with 'iteration order'.
why4: The Search contract promised relevance ordering that a brute-force substring backend cannot provide, and no substitute ordering (ID) was specified.
why5: No test asserted determinism or limit semantics for LinearSearch, so the randomized behavior went unnoticed on the memorybackend path.
prevention: Search now collects all matches, sorts by natural-sort ID order (natsort.Strings), then truncates — so the order is stable across calls and a limit returns the first-N of a defined order. Documented that LinearSearch (no scoring) returns ID order, not relevance. Regression tests assert repeated-query stability, natural-sort order (REQ-2 before REQ-10), and deterministic limit subsets; they fail without the fix on the first run.
status: done
---

## Bug

Found in the 2026-06-09 backend review (C3).

`LinearSearch.Search` (`internal/search/linearsearch.go`):

```go
for _, e := range l.entities {        // map: randomized iteration order
    if MatchText(e, text) {
        ids = append(ids, e.ID)
        if limit > 0 && len(ids) >= limit { break }  // arbitrary subset
    }
}
```

Two defects:

1. **Nondeterministic order** — the same query returns a different ordering on each call (Go randomizes map iteration). The `Service` wrapper (`index.go:35`) always calls `backend.Search(text, 0)` and applies `q.Limit` itself over the returned order, so the unstable order feeds pagination; the `memorybackend` build (where LinearSearch is the active backend) sees test flakiness.
2. **Truncate-during-iteration** — the mid-loop `break` at `limit` picks an arbitrary subset rather than the first-N of any defined order.

## Fix (PR pending)

Collect all matches, sort by natural-sort ID order, then truncate:

```go
for _, e := range l.entities {
    if MatchText(e, text) { ids = append(ids, e.ID) }
}
natsort.Strings(ids)
if limit > 0 && len(ids) > limit { ids = ids[:limit] }
```

LinearSearch is brute-force substring matching with no relevance scoring, so the
contract's "ordered by relevance" cannot apply — ID order is the honest, stable
substitute (documented in the godoc). `natsort` (already a common component)
gives `REQ-2 < REQ-10`, matching the codebase's ID-ordering convention.

## Tests

`internal/search/linearsearch_test.go`: `TestLinearSearch_DeterministicOrder`
(50 runs, identical natural-sort order), `TestLinearSearch_LimitReturnsFirstN`
(first-N deterministic), `TestLinearSearch_NoMatchesEmpty`. Verified they **fail
without the fix** on the first run (randomized order / arbitrary subset).
