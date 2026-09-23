---
id: IMPL-5Q57ZZ
type: implementation-checklist
title: 'Implementation: Query-driven entity lists in sidebar navigation groups'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code (naventities_test.go: validation table, EffectiveNavSort, SortParam; SidebarEntityQuery.test.ts; useSidebarEmptyGroups.test.ts; useEvents refresh dispatch; firstNavTarget skip)
- [x] Integration tests written (sidebar_entities_test.go drives the served definition through the list endpoint for two principals and under a Declarative ACL; TestNavEntitiesDeriveTheListIndex; Sidebar.entities.test.ts mounts the real Sidebar; e2e sidebar-entities.spec.ts)
- [x] Happy path implemented
- [x] Edge cases from planning handled (top-level entry, label, combined kinds, stray query_scope/sort, unknown type/scope/sort property, default scope and default_sort fallback, overflow past 100, empty group hidden, load failure shown, world carried on links, config reload refetches the sidebar)
- [x] Error handling in place (a failed row fetch renders "Could not load" and is logged; it never reads as "nothing matched")

## Test Quality

- [x] Using fixture builders or factories for test data (newScopeTestApp, seedEntity, installNav, response()/rows() helpers)
- [x] No hardcoded values in assertions when object is in scope
- [x] Only specifying values that matter for the test
- [x] Interpolated values constructed from objects, not hardcoded
- [x] Property comparisons use original object, not hardcoded strings

## Manual Verification

- [x] Feature manually tested end-to-end
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified

**Verification Evidence:**

- Browser, demo project with a ticket scope `active` and an `entities:` group "In progress": the group lists the two in-progress tickets with the authored icon. Long titles wrapped to two lines; changed to a one-line ellipsis with the full title as tooltip, then re-checked.
- e2e sidebar-entities.spec.ts (real rela-server + Chromium): adding the group by editing data-entry.yaml shows it without reload (refresh event); a PATCH bringing FEAT-002 into scope adds its link live; moving both out hides the group; clicking a link opens the entity. Full e2e suite: 319 passed; one unrelated flake (markdown-editor-links) passed 15/15 on rerun.
- Performance, postgres build on the perf seed (19,117 entities; 11,000 tasks), scope `open` matching 8,237 tasks, sort=due, per_page=100: 76 ms wall / 51 ms db in 7 queries, versus 23 ms for the store-paged unscoped page. Acceptable for a background sidebar refetch; no limit knob needed.
- Principal independence: TestSidebarEntities_RowsAreACLGated asserts identical _sidebar payloads for two principals and different rows from the list endpoint.

## Quality

- [x] Code follows project patterns (list endpoint reuse, consumer-side wiring, placeholderData, scoped-style slot)
- [x] Checked for DRY opportunities (validateSortSpecs shared by lists and nav; EffectiveNavSort shared by wire and index planner; listShapes reuses the list index derivation)
- [x] No security issues introduced (no new read path; rows come from the ACL-gated list endpoint)
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind
