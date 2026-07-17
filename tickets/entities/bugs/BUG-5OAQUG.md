---
id: BUG-5OAQUG
type: bug
title: Kanban board silently drops entities beyond page 1 (no pagination in KanbanView)
description: KanbanView fetches only the first page of the list API (default per_page 25) and ignores meta.has_more, so any entity sorted onto page 2+ silently disappears from the board. Missing cards are indistinguishable from 'none exist' — a correctness bug that makes boards unreliable once a type exceeds the page size.
priority: high
effort: s
why1: KanbanView issues a single listEntities call and renders only that page's data, never reading meta.has_more — the server pages collections at 25 by default (cap 100), so entities on page 2+ never reach the board.
why2: The board partitions the full entity set client-side but consumes a paginated endpoint as if it returned the whole collection; it was built and tested against fixture datasets smaller than one page, where a single fetch looks complete.
why3: 'The client API layer returns one page and leaves pagination to each caller — there is no ''fetch complete set'' helper — so every full-set consumer must independently remember to loop on has_more (same failure class as the per_page=1000 scope-navigation truncation, #844).'
why4: 'No test or contract forces list-consuming views to handle has_more: true; component test fixtures default to single-page responses (seedBoard hardcodes has_more: false), so the truncation path was never exercised.'
why5: The pagination contract exists only as convention — the type system does not distinguish 'one page' from 'complete set', so consuming a page as the full set compiles and renders cleanly and fails only at data volume.
prevention: 'Two-layer regression guard (automated-measure kanban-full-fetch-pagination-test): API-layer unit tests pin the fetch-all contract (multi-page merge, dedupe, response-driven advance, abort, empty-page guard, cap-preserves-has_more) and a component test runs the real loop inside the mounted board via an HTTP-client mock. Systemically: listAllEntities now exists as THE fetch-complete-set seam in the api layer, so future full-set consumers no longer hand-roll pagination (the why3/why5 root cause); its doc comment states the one-page-vs-complete-set distinction the type system doesn''t express, and the truncation state is user-visible by contract (has_more: true on the merged response) rather than silent.'
status: done
---

## Reproduce

1. Configure a kanban (`column_property: status`, N status columns).
2. Create > 25 entities of that type (the API's default `per_page`).
3. Open the kanban. The API returns `{data: [...25], meta: {total: >25, page: 1, has_more: true}}`; the board renders only those 25. Entities on page 2 are absent — no error, no indicator.

## Concrete case (reporter's instance)

- Entity type `taak`, 46 total, kanban `taken_bord` on `status` (4 columns: todo/bezig/wachten/gereed).
- TASK-RGC5 (status `wachten`) is on page 2 → missing from the "Wachten" column.
- Other `wachten` tasks that happen to sort into page 1 (TASK-7F8K, TASK-H9UM) do show up, so the user sees a partially-populated column and has no reason to suspect data is missing.

## Root cause (reporter's quick read)

- `dataentryconfig.Kanban` has no `page_size` field (only `List` does — see `internal/dataentryconfig/config.go:196`), so the limit can't be raised via config either.
- KanbanView (compiled asset `KanbanView-BG1UPc7-.js`) contains no `has_more` / pagination logic; it fetches once and stops.

## Impact

Correctness bug — the board is unreliable as soon as any board's underlying
entity list exceeds `per_page`. For an ISMS/ops board, "quietly loses a task" is
worse than "shows nothing." Filters (default: none) don't help since kanbans
typically show all statuses.

## Suggested fixes (from reporter)

1. **Client-side loop until `has_more: false` in KanbanView** — the board already loads the full type into memory to partition by column property, so paging through is the natural completion of what's already happening. (Cheaper.)
2. Column-scoped fetches per status value, one request per column (uses the column's status as a filter). Cleaner but N requests instead of 1.

Optionally add `page_size` to the Kanban config as an escape hatch, but even
that won't rescue a board that outgrows the ceiling — the real fix is completing
the fetch.

## Workaround

None from config. Users must fall back to the list view (e.g. `/list/all_taken`)
to see all tasks; the kanban is unusable for planning.
