---
id: BUGA-VHOI2A
type: bug-analysis-checklist
title: 'Analysis: Kanban board silently drops entities beyond page 1 (no pagination in KanbanView)'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Reproduction

- [x] Bug reproduced locally — confirmed by code trace: `frontend/src/views/KanbanView.vue:60-68` issues a single `listEntities(config.entity)` call with no `page`/`per_page` params and renders only `data.value?.data`; `meta.has_more` is never read anywhere in the component. Server side, `parseV1Pagination` (`internal/dataentry/api_v1.go:2285-2302`) defaults to `per_page=25` and caps it at 100. The failing regression test written first during implementation is the executable reproduction.
- [x] Minimal reproduction steps documented — in BUG-5OAQUG body: any kanban whose entity type exceeds 25 entities; reporter's concrete case: type `taak`, 46 entities, TASK-RGC5 (status `wachten`, page 2) absent from the board while page-1 `wachten` tasks render.
- [x] Environment/conditions noted — any backend/browser; triggers whenever an entity type's total exceeds the API default page size (25). No config escape hatch: `dataentryconfig.Kanban` has no `page_size` field, and even `per_page=100` (the server cap) only postpones the truncation.

## Root Cause

- [x] Immediate cause identified (why1) — KanbanView fetches exactly one page and ignores `meta.has_more`, so entities sorted onto page 2+ silently never enter the board's data.
- [x] Contributing factors found (why2-3) — why2: the board renders the full set client-side (partitions by column property) but consumes a paginated endpoint as if it returned the whole collection; developed/tested against fixtures under the page size, where one fetch looks complete. why3: the client API layer (`listEntities`) returns one page and leaves paging to each caller with no "fetch complete set" helper, so every full-set consumer must remember `has_more` — a known failure class (same as the `per_page=1000` scope-navigation truncation, issue #844).
- [x] Systemic cause explored (why4-5) — why4: no test or lint contract forces list-consuming views to prove they handle `has_more: true`; component fixtures default to single-page responses (`KanbanView.test.ts` `seedBoard` hardcodes `has_more: false`). why5: the pagination contract lives only in convention/docs, not in a typed seam that makes "one page" vs "complete set" explicit at the call site.

## Fix Planning

*(Revised after /design-review — RR-QD46GS, RR-0Y3Q6T, RR-5YVXMK, RR-JUVDUW
addressed; RR-1IBKZ0 deferred; RR-7YDNSN wont-fix.)*

- [x] Fix approach determined — **`listAllEntities(type, params)` in `frontend/src/api/entities.ts`**:
  - Requests `per_page: 100`; advance is **purely response-driven** (RR-JUVDUW): loop while `meta.has_more`, requesting `meta.page + 1` — never derive offsets from the requested page size, since `parseV1Pagination` silently falls back to 25 on out-of-range values.
  - **Dedupe by entity ID** (RR-QD46GS): merge pages into a `Map<string, Entity>` (later page wins — fresher copy); store order is deterministic on both backends (fsstore sorted keys, pgstore `ORDER BY id ASC`) but concurrent writes between page fetches shift offsets — duplicates would break `v-for :key` and `beginOptimistic`; misses self-heal via the SSE invalidation the same write triggers.
  - **Safety cap: 50 pages (~5,000 entities)** with visible truncation (RR-0Y3Q6T): on cap hit the merged response keeps `meta.has_more: true`; the api layer stays store-free (its documented purity rule) and **KanbanView renders a board-level warning banner** when `data.meta.has_more` is true: "Showing N of TOTAL items — board truncated". A complete fetch always returns `has_more: false`.
  - Merged response shape is a normal `ListResponse`: concatenated deduped `data`, merged `included`, `_actions` from the first page, `meta.total` from the last page. KanbanView's `boardQuery` swaps `listEntities` → `listAllEntities`; cache key, optimistic drag-drop, `_actions` gating, and SSE background refetch untouched.
  - Rejected: option 2 (per-column filtered fetches — N requests, still truncates within a column past the cap, fragments the optimistic-update cache); `page_size` in Kanban config (unnecessary once the fetch completes); parallel page fetches (RR-7YDNSN wont-fix — one extra round-trip for realistic boards, sequential composes with the response-driven contract; documented as a future perf option). Deferred: AbortSignal plumbing for mid-loop cancellation (RR-1IBKZ0 — stale loops waste requests but cannot corrupt state; Colada keys per type).
- [x] Regression test planned — **test seam chosen explicitly** (RR-5YVXMK: mocking `'@/api'` cannot intercept `listAllEntities`'s module-internal call to `listEntities`):
  - **API-layer unit tests** mocking the HTTP client (`'@/api/client'` `api.get`): 2-page merge, `included` merge, dedupe of an entity appearing on two pages, response-driven advance when the server ignores `per_page`, cap hit ⇒ `has_more: true` preserved.
  - **Component regression test in a new file** `frontend/src/views/KanbanView.pagination.test.ts` mocking `'@/api/client'` only, so the **real pagination loop runs inside the mounted component**: 2-page fixture (`has_more: true` → `false`), assert cards from both pages render and no truncation banner; cap-hit fixture asserts the banner renders. (The existing `KanbanView.test.ts` keeps its `'@/api'` mock — its `listEntities` mock switches to `listAllEntities`.)
  - Tracked as automated-measure `kanban-full-fetch-pagination-test`.
- [x] Related areas checked for similar issues — audited all `listEntities` callers: `EntityList.vue` paginates properly (user-facing pager); `useBacktickAutocomplete.ts` caps at MAX_RESULTS deliberately (suggestion list); `stores/entities.ts` legacy store passes params through (callers own paging); `bridge/relaBridge.ts` exposes params to apps (paging is the app's contract). Only KanbanView consumes a paged endpoint as a complete set.
