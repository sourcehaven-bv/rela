---
id: IMPL-QOZ12U
type: implementation-checklist
title: 'Implementation: Relation-picker autosave aborts with "unknown types" when the linked entity is outside the 100-candidate window'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code
- [x] Integration tests written (test full flow, not just units)
- [x] Happy path implemented
- [x] Edge cases from planning handled
- [x] Error handling in place (errors surfaced, not swallowed)

Change is two files:

- `frontend/src/stores/entities.ts` — adds `fetchAllList`, which pages the
collection via the existing `listAllEntities`. `fetchList` and `fetchAllList`
share one `fetchListInternal` so row-caching, the loading flag and the
`try/finally` stay in one place rather than being duplicated per mode.
- `frontend/src/components/forms/RelationPicker.vue` — `loadCandidates` calls
`fetchAllList`. Its existing `isCancelledFetch` handling is unchanged, and
`listAllEntities` propagates the abort between pages.

Edge case handled deliberately: the paging **mode is part of the cache key**, so
a `fetchList` (page-1) entry can never be served to a `fetchAllList` caller.
That is the same bug in cache form and would have survived the fix otherwise.

## Test Quality

- [x] Using fixture builders or factories for test data
- [x] No hardcoded values in assertions when object is in scope
- [x] Only specifying values that matter for the test
- [x] Interpolated values constructed from objects, not hardcoded
- [x] Property comparisons use original object, not hardcoded strings

The e2e spec asserts against `SEED.*` constants and the `id` returned by the
seeding loop, never a literal `FEAT-104` — the off-page ID is whatever the
server minted.

Both new store tests were **mutation-verified**: reordering the cache key to
`<mode>:<type>:` (so `invalidateListCache`'s `<type>:` prefix stops matching)
fails `invalidates paged list entries on write`, and it also fails the
pre-existing `invalidates list cache when creating entity`. The e2e spec was
verified red before the fix and green after.

## Manual Verification

- [x] Feature manually tested end-to-end
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified

**Verification Evidence:**

Real browser (Playwright-driven Chromium) against `rela-server` on a project
seeded with 120 features, TASK-001 linked to FEAT-120 (page 2):

```text
chips before: ['Feature 120 (FEAT-120)']
chips after : ['Feature 007 (FEAT-007)', 'Feature 120 (FEAT-120)']
PATCHes sent: ['/api/v1/tasks/TASK-001']
unknown-types toast: false
server after: {'implements': ['FEAT-007', 'FEAT-120']}
```

Before the fix the same flow sent **zero** PATCHes and showed the toast
verbatim. The pre-existing off-page link survives the save rather than being
dropped, which was the second failure mode worth guarding.

Test runs: 2481/2481 frontend unit tests pass; full e2e suite 292 passed, 8
skipped, 0 failed; `npm run lint` and `npm run typecheck` clean; `just
arch-lint` OK.

## Quality

- [x] Code follows project patterns (check similar code)
- [x] Checked for DRY opportunities — repeated literals, expressions, or
patterns extracted to a helper / constant / type where it sharpens the contract
- [x] No security issues introduced
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind

Follows the precedent set by BUG-5OAQUG: `KanbanView` and `CalendarView` already
use `listAllEntities` for exactly this reason. This change routes the last
remaining single-page consumer through it — `grep "per_page: 100"` over
`frontend/src` now returns only the `listAllEntities` implementation itself.

The two fetch modes share `fetchListInternal` rather than copying the caching
block, and the cache-key comment states *why* the mode goes after the type so
the next person does not "tidy" it into a prefix.
