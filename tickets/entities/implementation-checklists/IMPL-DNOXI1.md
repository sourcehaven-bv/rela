---
id: IMPL-DNOXI1
type: implementation-checklist
title: 'Implementation: Kanban board silently drops entities beyond page 1 (no pagination in KanbanView)'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code — `frontend/src/api/entitiesListAll.test.ts`: 6 cases covering single-page passthrough, multi-page data+included merge, `_actions` from first page, dedupe by ID (later page wins), response-driven advance when the server ignores `per_page`, and the 50-page cap preserving `has_more: true`.
- [x] Integration tests written (test full flow, not just units) — `frontend/src/views/KanbanView.pagination.test.ts` mocks only the HTTP client (`@/api/client`), so the **real** `listAllEntities` loop runs inside the mounted component (the RR-5YVXMK seam): asserts a page-2-only entity renders in its column, page-2 request issued, no banner on complete fetch; and the cap-hit case renders the truncation banner while still showing fetched cards.
- [x] Happy path implemented — `listAllEntities` in `frontend/src/api/entities.ts`; `KanbanView.vue` boardQuery swapped to it. Cache key, optimistic drag-drop, `_actions` gating, SSE invalidation untouched.
- [x] Edge cases from planning handled — dedupe by ID (RR-QD46GS), response-driven paging (RR-JUVDUW), 50-page cap with `has_more: true` + visible board banner (RR-0Y3Q6T, confirmed in scope by user).
- [x] Error handling in place (errors surfaced, not swallowed) — a failing page request rejects the whole query; the board renders its existing error-state (unchanged behavior, no partial render pretending success).

## Test Quality

- [x] Using fixture builders or factories for test data — `makeEntity`/`makeTicket`/`page()` helpers in both new test files.
- [x] No hardcoded values in assertions when object is in scope
- [x] Only specifying values that matter for the test — `page()` supplies defaults, cases override only what they assert.
- [x] Interpolated values constructed from objects, not hardcoded — banner assertion uses the fixture's total (9999) and cap-derived count.
- [x] Property comparisons use original object, not hardcoded strings — dedupe test compares via `res.data.find(...)`.

## Manual Verification

- [x] Feature manually tested end-to-end
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified — see evidence.

**Verification Evidence:**

Built `rela-server` with the fixed SPA (`just build-server`), ran it against a
scratch project reproducing the reporter's case (entity type `taak`, kanban
`taken_bord` on `status` with 4 columns), verified in a headless browser:

- **46 entities (reporter's exact scenario):** API confirmed the bug shape first (`GET /api/v1/taken` → 25 items, `has_more: true`). Board rendered **all 46 cards** (Todo 11 / Bezig 12 / Wachten 12 / Gereed 11), including entities beyond default page 1; no truncation banner.
- **130 entities (forces the live multi-page loop):** SPA issued exactly `?page=1&per_page=100` then `?page=2&per_page=100`; board rendered **130 unique cards** (32/33/33/32), `TASK-130` present, no duplicates (`new Set(ids).size === 130`), no banner.
- Cap-hit banner verified in the component test (50-page fixture → "Showing 50 of 9999 items — the board is incomplete."); not reproducible live without ~5,000 entities, by design.

Automated checks: full frontend suite 77 files / **1215 tests pass**; `vue-tsc`
typecheck clean; ESLint 0 errors (4 warnings on touched files all pre-existing:
`max-lines` on KanbanView.vue, type-assertion warnings in the old test file).

## Quality

- [x] Code follows project patterns (check similar code) — api-layer helper stays store-free per the module's documented purity rule; test seams mirror `entitiesPlural.test.ts` (client mock) and `KanbanView.test.ts` (module mock); banner styling follows the view's existing scoped-CSS conventions.
- [x] Checked for DRY opportunities — `page()` fixture helper per test file (intentionally local; the two files mock different seams). No production-code duplication introduced.
- [x] No security issues introduced — read path only; every page request re-passes the server's ACL read gate; no new user input.
- [x] No silent failures (errors logged AND returned) — cap truncation is the one degraded mode and it is user-visible (banner), not console-only.
- [x] No debug code left behind
