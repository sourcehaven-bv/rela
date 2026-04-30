---
id: IMPL-HCWVA
type: implementation-checklist
title: 'Implementation: Add search interface to data-entry list views'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code
- [x] Integration tests written (test the full flow, not just units)
- [x] Feature implemented
- [x] Edge cases from planning handled

**Tests added:**
- `internal/dataentry/api_v1_test.go::TestV1ListEntitiesSearchQuery` (4 sub-tests: empty q no-op, q intersects + preserves list sort, q with no matches, q AND-combines with property filter).
- `frontend/src/components/lists/SearchBox.test.ts` (6 tests: debounce, Enter flush, clear button, Esc, external prop sync, in-progress text not clobbered).
- `frontend/src/components/lists/AdHocFilterMenu.test.ts` (5 tests: locked properties hidden, enum apply, free-text apply, Escape cascading close, type option).

**Implementation:**

Backend:
- `internal/dataentry/helpers.go`: added `freeTextIDsForType` — runs Bleve search constrained to one type, returns id set.
- `internal/dataentry/api_v1.go::handleV1ListEntities`: reads `?q=`, intersects entities by id BEFORE filter/sort/paginate so list sort wins over Bleve relevance ranking.

Frontend:
- New `frontend/src/components/lists/SearchBox.vue` — 250ms-debounced text input with clear button and Esc handling.
- New `frontend/src/components/lists/AdHocFilterMenu.vue` — lifted from SearchView; two-step property+value picker; supports both list mode (one type) and search mode (synthetic `Entity Type` option) via `includeTypeOption`.
- `frontend/src/composables/useUrlFilterSync.ts`: extended to round-trip `q` alongside `filter[*]`.
- `frontend/src/composables/useListKeyboard.ts`: added `onFocusSearch` callback, bound to `/`.
- `frontend/src/composables/useKeyboardShortcuts.ts` and `frontend/src/components/common/Sidebar.vue`: defer the global `/` shortcut to the list's in-place search box when present, so users don't lose list context.
- `frontend/src/views/SearchView.vue`: refactored to use the new `AdHocFilterMenu` (no duplicated dropdown code remains).
- `frontend/src/components/lists/EntityList.vue`: renders SearchBox + AdHocFilterMenu above FilterBar; ad-hoc filters render as removable chips; empty-state shows "No matches" with a Clear-search button when q is active; forwards `q` in `navigateToEntity` query.

## Manual Verification

- [x] Tested feature end-to-end manually (puppeteer + just dev)
- [x] Verified each acceptance criterion
- [x] Documented verification evidence

**Verification evidence:**

| AC | Method | Result |
|----|--------|--------|
| AC1: SearchBox above every list | Visited `/list/all_tickets` | SearchBox present ✓ |
| AC2: typing filters with debounce | Typed "API" → 3 rows (was 5) | URL became `?q=API`, list narrowed ✓ |
| AC3: `q=` in URL hydrates input | Direct nav `/list/all_tickets?q=API` | input value=`API`, 3 rows ✓ |
| AC4: clear restores list | Clicked clear-search button | URL stripped `q`, 5 rows ✓ |
| AC5: + Filter dropdown lists props | Opened menu | Properties shown, FilterBar-managed ones excluded ✓ |
| AC6: AND-combines with FilterBar | Backend test + smoke API call | filter[status]=open AND q=API yielded subset ✓ |
| AC7: static-pinned props hidden | Visual: Status/Priority/Assignee not in menu | ✓ |
| AC8: `/` focuses; Esc clears | Dispatched `/` keydown | Input focused; Esc cleared input + URL ✓ |
| AC9: empty state shows clear-search | Typed "noooooope" | "No matches for ..." + Clear-search button ✓ |
| AC10: backend intersects + sort wins | `TestV1ListEntitiesSearchQuery` | Pass — TKT-001 before TKT-003 in list sort ✓ |

## Quality

- [x] Code follows project patterns (CLAUDE.md, idioms)
- [x] No silent failures (errors surface, not just logged)
- [x] No dead code or commented-out blocks
- [x] No magic numbers or hardcoded paths
- [x] Type checks pass (`npm run typecheck`)
- [x] Lint passes (`just lint`, `npm run lint`)

**Quality notes:**

- All Go tests pass (`go test -race ./internal/dataentry/...`).
- All 558 frontend tests pass (`npm run test:run`).
- `just lint`, `just arch-lint`, `npm run typecheck`, `npm run lint` all clean.
- `just coverage-check` PASS (74.2% total).
- SearchView refactored — no duplicated dropdown code remains (per user's "no tech debt" instruction).
