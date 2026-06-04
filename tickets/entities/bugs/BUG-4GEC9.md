---
id: BUG-4GEC9
type: bug
title: 'Entity scope navigation total wrong: per_page=1000 silently capped to 25'
description: |-
    The entity scope navigator in `EntityDetail` (the `[current/total]` counter with Prev/Next) showed a wrong total — e.g. "1/25" for a type with 93 entities. `useScopeNavigation` requested `per_page=1000` to fetch the whole set and locate the current entity's position client-side, but `parseV1Pagination` in `api_v1.go` silently ignores any `per_page > 100` and falls back to the default of 25. Position was then computed over a truncated 25-row array.

    **Impact:** Navigation across any list with more than 25 entities of a type was truncated, and the counter under-reported the true total. The list view itself was unaffected (it paginates correctly); only the detail-page scope navigator was wrong.

    **Reproduce:**

    1. Have more than 25 entities of a type (e.g. 93 `maatregel` entities).
    2. Open the list view and navigate into any entity.
    3. Observe the navigation counter shows "1/25" instead of "1/93", and Prev/Next only walk the first 25.

    **Fix:** Stop shipping the whole ordered set to the client to derive four scalars. Added a server-side `GET /api/v1/_position?id=&scope=` endpoint that runs the same filter/sort pipeline as the list endpoint (shared `scopedSortedEntities` helper) and returns `{prev, next, current, total}` directly. Scope is encoded as an unsigned, URL-encoded JSON descriptor (`{source, type, filters, sort, q}`) validated by a strict decoder. `useScopeNavigation` now calls `_position` instead of fetch-all-and-scan, so navigation is correct at any set size and the `per_page` cap is no longer load-bearing. As a bonus, the descriptor carries `q`, so scope navigation now honors an active free-text search (resolving the known limitation at `EntityList.vue:475`).
priority: medium
effort: m
why1: The scope navigator showed a total of 25 regardless of the real entity count.
why2: The composable requested `per_page=1000` and derived position from the returned array, but the backend silently capped `per_page` at 100 and fell back to the default of 25 when the value was out of range.
why3: The `parseV1Pagination` helper rejected out-of-range `per_page` by *ignoring* it (silent reset to the default) rather than clamping or erroring, so the frontend's intent was discarded without any signal.
why4: The frontend used a large page size as a stand-in for "give me the whole ordered set" — pagination was being abused as a bulk-fetch, coupling navigation correctness to a magic-number cap on an unrelated endpoint.
why5: There was no first-class notion of a navigable "scope" — the ordered result set the user is viewing — so each consumer reassembled it from loose params, and the only way to get position was to materialize the entire set client-side. The fix introduces a scope descriptor + server-side position resolution so the set is never shipped.
prevention: |-
    Backend tests in `internal/dataentry/scope_test.go` pin the contract:
    `TestV1Position` (middle/first/last/filtered/search/404),
    `TestV1PositionBadRequest` (strict-decoder rejections), and crucially
    `TestV1PositionMatchesListOrdering`, which asserts `_position` observes
    the same ordered set as the list endpoint for the same scope — so the
    two pipelines cannot silently diverge.

    Frontend tests in `useScopeNavigation.test.ts` cover the descriptor
    construction (filters, sort, default-sort, and `source: 'search'` for
    `q`) and the position-derivation/404 behavior.

    The deeper systemic preventive: position is now resolved from a single
    shared `scopedSortedEntities` helper that the list endpoint also uses,
    rather than a parallel client-side reimplementation gated on a page-size
    cap. New scope sources extend the descriptor's `source` field instead of
    adding new fetch-all call sites.
status: done
---

See GitHub issue #844 and PR #894.
