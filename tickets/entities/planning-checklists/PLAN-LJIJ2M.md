---
id: PLAN-LJIJ2M
type: planning-checklist
title: 'Planning: Collapsible kanban columns'
started: "2026-10-09"
completed: "2026-10-09"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:** In: a collapse control on every kanban column heading (plain and
swimlane boards); KanbanView marks sections `collapsed` and handles
expand/collapse; the reader's choice is kept in localStorage per board;
`columns[].collapsed: true` in `data-entry.yaml` sets the default; docs in
`docs/data-entry.md`. Out: dropping a card onto a collapsed column (the library
already excludes collapsed columns as drop targets, and the rail has no card
list); collapse on inferred columns via config (no place to declare it; the
reader can still collapse them); server-side per-user persistence.

**Acceptance Criteria:**
1. A reader collapses a column with the heading's collapse button and expands it by clicking the rail. Test: e2e on a plain board and a swimlane board.
2. A collapsed column shows its title and card count. Test: e2e asserts rail text and count.
3. The choice survives a reload, per board. Test: e2e reload; unit test of the storage composable (corrupt entry, other board unaffected).
4. `collapsed: true` on a declared column starts it collapsed and the reader can expand it; the expansion is remembered. Test: Go config test (field parsed and served in JSON) + e2e on a fixture board.

## Research

- [x] ~~For larger features: run `/research`~~ (N/A: small UI change, library already renders the collapsed state)
- [x] ~~Searched for existing libraries that solve this problem~~ (N/A: rela-components already has RlBoardColumnCollapsed)
- [x] Checked codebase for similar patterns or reusable code
- [x] ~~Looked for reference implementations in other projects~~ (N/A: mockup in Atlas TASK-8997C defines the look)
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** N/A

**Existing Solutions:**
- `RlBoardColumnCollapsed.vue` and the rail in `RlSwimlaneBoard.vue` already render a collapsed column and emit `expandSection`; `RlBoard`/`RlSwimlaneBoard` already skip collapsed columns as drop and keyboard targets.
- `useListGrouping.ts` (`collapsedStorageKey`, `readCollapsed`) persists closed list sections in localStorage; same pattern for board columns.
- KanbanView already folds swimlanes (`collapsedLanes`), view-state only.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:**
1. rela-components: `RlSectionHeading` gets an optional collapse button (prop `collapsible`, emit `collapse`), message `collapseSection({title})`. `RlBoardColumn` and the swimlane header pass it through; `RlBoard` / `RlSwimlaneBoard` gain prop `collapsible` and emit `collapseSection`.
2. Go config: `KanbanColumn.Collapsed bool` (`yaml:"collapsed" json:"collapsed,omitempty"`). No validation beyond YAML typing.
3. SPA: composable `useKanbanCollapse(kanbanId, defaults)` stores an override map `{columnValue: boolean}` under `rela:kanban-columns:<id>`; effective state = override ?? config default. Storing overrides (not a set) lets a reader expand a default-collapsed column and keep it expanded.
4. KanbanView sets `collapsed` on `boardSections`, passes `collapsible`, handles `expandSection`/`collapseSection`.
5. Docs: column table gets `collapsed`; a short "Collapsing columns" subsection.

**Alternatives rejected:** view-state only (like lanes) fails the reload
criterion in the Atlas task; server-side user preference is heavier and no other
UI state uses it yet.

**Files to modify:**
- internal/dataentryconfig/config.go (+ test)
- frontend/packages/rela-components/src/components/{common/RlSectionHeading.vue, board/RlBoard.vue, board/RlBoardColumn.vue, board/RlSwimlaneBoard.vue}, composables/useMessages.ts
- frontend/src/types/config.ts, frontend/src/composables/useKanbanCollapse.ts (+ test), frontend/src/views/KanbanView.vue
- docs/data-entry.md, e2e test + fixture board

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] ~~Security-sensitive operations identified~~ (N/A: client-side view state only)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:**
- localStorage entry: parsed defensively; non-object or non-boolean values ignored, corrupt JSON falls back to config defaults.
- `collapsed:` in config: YAML bool, typed by the decoder.

**Security-Sensitive Operations:** None. Counts on the rail come from the
already gated board read.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:** see acceptance criteria; vitest for the composable and the
heading button; Go test for config JSON; Playwright e2e for plain board,
swimlane board, reload, config default.

**Edge Cases:**
- Stored override for a column value that no longer exists: ignored.
- All columns collapsed: board shows only rails.
- Collapsed column with 0 cards: rail shows 0.
- Card selected in a column that gets collapsed: selection remains in URL; no crash.

**Negative Tests:** corrupt localStorage JSON; stored non-boolean values.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:**
- Heading button crowding on narrow columns: icon-only button with aria-label.
- Overlap with piles branch (TKT-K3RJLH): it does not touch board files.

Effort: s

## Documentation Planning

- [x] User-facing docs identified (skip if internal refactor)
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:**
- [x] docs/data-entry.md - Kanban columns: `collapsed`, collapsing columns

## Design Review

- [x] ~~Run `/design-review` before starting implementation~~ (N/A: small UI change on an existing library capability; plan reviewed with the user)
- [x] ~~All critical/significant findings addressed in plan~~ (N/A: no design review run)

**Design Review Findings:** N/A
