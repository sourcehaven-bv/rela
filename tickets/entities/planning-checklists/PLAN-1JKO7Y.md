---
id: PLAN-1JKO7Y
type: planning-checklist
title: 'Planning: Create menu puts the current page''s type first'
started: "2026-10-15"
completed: "2026-10-15"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:**

In scope: the order of the rows in the space Create menu
(`SpaceCreateMenu.vue`). On a list, kanban, calendar or gantt page, and on a
page tab that shows one of those, the offered types the view shows come first.
The rest keep config order. A separator follows the leading group when other
rows follow.

Out of scope: which types are offered (unchanged, incl. the entity-page filter
from TKT-XPFXMG); a config key; the entity view `/entity/<type>/<id>` and
documents (they show one entity, and a new entity of the same type is not the
likely intent); dashboards; kanban card code (PR #1893).

**Acceptance Criteria:**
1. List page: the list's type is first; rest in config order. Test: Vitest + e2e `/s/support/list/bugs` with `create: [task, bug]` gives Bug, Task.
2. Kanban page: the board's type is first. Test: Vitest + e2e `/s/support/kanban/bug-board` gives Bug, Task.
3. Calendar/gantt with several source types: those offered types first, in config order. Test: Vitest.
4. Page tab: the open tab's view decides. Test: Vitest for `/p/<page>/<tab>` and the entity-page form.
5. Page without view types: config order. Test: e2e `/s/support/dashboard` gives Task, Bug; Vitest.
6. Separator only between a non-empty leading group and other rows. Test: Vitest.

Decision (user-approved plan): with several view types, the view types go
first in config order. A gantt's `sources` is a map, so its key order carries
no meaning; using the config order makes one rule for every view.

## Research

- [x] ~~For larger features: run `/research` to create a structured research doc~~ (N/A: small frontend change)
- [x] ~~Searched for existing libraries that solve this problem~~ (N/A)
- [x] Checked codebase for similar patterns or reusable code
- [x] ~~Looked for reference implementations in other projects~~ (N/A)
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** N/A

**Existing Solutions:** `usePageCreateLink().offered` (usePageTabScope.ts)
filters the rows; PageView resolves the open tab from the route the same way
this change does; `RlMenuSeparator` is the rela-components divider used in
EntityActionsMenu, AccountMenu and SpaceSwitcher.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:**
- New `frontend/src/composables/useViewEntityTypes.ts`: pure `viewEntityTypes(view, id, schema)` for list/kanban/calendar/gantt; `pageTabView(route params, pages)` resolves the open tab of a page like PageView; `useViewEntityTypes()` reads the current route.
- Pure `leadWith(rows, types)` splits rows into `{ lead, rest }`, both in config order.
- `SpaceCreateMenu.vue` renders lead, a `RlMenuSeparator` when both groups are non-empty, then rest.

Alternatives: views publishing their types to a store (rejected: every view
must set and clear it, like `pageStore.current`; the route already says which
view is open); the view's own type order (rejected: gantt sources have none).

**Files to modify:** frontend/src/components/common/SpaceCreateMenu.vue,
frontend/src/composables/useViewEntityTypes.ts (+ test),
SpaceCreateMenu.test.ts, e2e/tests/spaces.spec.ts,
docs-project/entities/guides/GUIDE-data-entry.md, docs/data-entry.md.

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:** route params and the served config. An
unknown view id yields no types, so the order stays the config order.

**Security-Sensitive Operations:** none. Only order changes; the offered set
still comes from the server and the entity-page filter.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:** see acceptance criteria.

**Edge Cases:** view type not among `create:`; view type offered but already
first; unknown list id; schema not loaded yet; page tab without target; entity
page two-segment URL; duplicate source types in a calendar.

**Negative Tests:** unknown view id or page gives config order, no separator.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:** a test or doc that relies on the fixed order (checked: only
spaces.spec, which still holds and is extended). Effort: s.

## Documentation Planning

- [x] User-facing docs identified (skip if internal refactor)
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:**
- [x] docs/data-entry.md - UI changes (§ Spaces, `create:`), via GUIDE-data-entry.md

## Design Review

- [x] ~~Run `/design-review` before starting implementation~~ (N/A: plan approved by the user)
- [x] ~~All critical/significant findings addressed in plan~~ (N/A)

**Design Review Findings:** none
