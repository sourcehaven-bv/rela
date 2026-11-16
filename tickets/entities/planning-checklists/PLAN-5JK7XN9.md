---
id: PLAN-5JK7XN9
type: planning-checklist
title: 'Planning: One step at a time over a set of entities'
started: "2026-11-15"
completed: "2026-11-15"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:**

In scope: a `walk: true` option on a list tab of a `mode: steps` page. The step
shows the list's rows one at a time in the entity's own detail view, with "3 of
8" and previous/next row controls. The current row is in the URL as
`?item=<id>`. In present mode PageDown and PageUp move through the rows first
and then to the next or previous step. Works on entity pages too (the tab's page
scope narrows the rows). Docs, Go, Vitest and e2e tests.

Not in scope: choosing a form instead of the detail view (the detail view edits
in place, so a form option can follow if a workshop needs one), a `walk` on
kanban tabs, remembering the last row of a step when stepping back, shared state
between screens.

**Config shape (decision):** `walk: true` on a list tab. Alternatives: a new tab
destination `walk: <list>` (a seventh destination key would duplicate `list:`
and every check written for it), or a mapping `walk: {form: ...}` (nothing to
configure yet; a boolean can grow into a mapping later without breaking configs,
as `scope:` did).

**Acceptance Criteria:**

1. `walk: true` on a list tab of a steps page loads; on a tabs page or a
non-list tab it fails to load with a message naming page and tab. Test:
`TestValidatePages` cases.
2. The sidebar payload carries `walk` on the tab. Test:
`TestPages_SidebarCarriesStepsModeAndPrompt`.
3. Opening a walk step without `?item=` replaces the URL with the first row
the list shows; with no rows an empty state shows. Test: PageView unit test.
4. The step shows "N of M", the entity's detail view, and previous/next row
links, disabled at the ends. Test: PageView unit test; e2e.
5. In present mode PageDown moves to the next row, and from the last row to
the next step; PageUp moves back the same way. Test: PageView unit test; e2e.
6. A row the principal cannot see, or that the list does not show, is never
counted or shown: position comes from `/_position`, which runs the list pipeline
under the row gate and now also applies the list's `condition:` (`list_id` in
the scope). An `?item=` outside the set shows a "not in this step" state. Test:
Go test on `/_position` with `list_id`; PageView unit test for the 404.

## Research

- [x] For larger features: run `/research` to create a structured research doc
- [x] Searched for existing libraries that solve this problem
- [x] Checked codebase for similar patterns or reusable code
- [x] Looked for reference implementations in other projects
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** N/A: extends Pages steps mode (TKT-35BY3PE) with existing
building blocks.

**Existing Solutions:**

- `useScopeNavigation` and `GET /api/v1/_position`
(`internal/dataentry/scope.go`): prev/next/current/total over a list scope,
gated, store-paged when possible. Reused as the walk's position source.
- `listBaseParams` (`frontend/src/utils/listParams.ts`): the one builder of a
list's base read. The walk builds both its first-row read and its position scope
from it, so it cannot disagree with the list.
- `EntityDetail` embedded with `followOwner=false`, as `EntityDetailPanel`
does.
- No library needed.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:**

Backend:
- `PageTab.Walk bool`; `validateStepListOptions` refuses it outside a steps
page and on a non-list tab. `SidebarPageTab.Walk` on the wire.
- `ScopeDescriptor.ListID` (`list_id`): only with source list; `toQuery`
passes it on, `resolveScope` applies the list's condition after the gated
pipeline (`scopeListCondition`, primed). `storePosition` declines to the Go path
because `resolveListNarrowing` now sees the condition. This also fixes the walk
counting rows the list's condition hides.

Frontend:
- `useStepWalk` composable: first row by `listEntities(per_page=1)` and the
position by `getEntityPosition`, both from `listBaseParams` plus the page scope
and the world. Recomputed after `entity:changed` (debounced).
- `PageWalk.vue`: "N of M", previous/next links, `EntityDetail` keyed by row.
- `PageView`: `?item=` read from the route; step links drop it; PageDown and
PageUp consult the walk first.

Alternatives rejected: loading every row id in the SPA (truncates at the page
cap, the #844 defect); a path segment for the row (clashes with entity page
routes `/p/<page>/<entity>/<tab>`).

**Files to modify:**

- internal/dataentryconfig/page.go, page_test.go
- internal/apiwire/v1/responses.go
- internal/dataentry/views_handler.go, scope.go (+ tests)
- frontend/src/types/config.ts, api/entities.ts
- frontend/src/composables/useStepWalk.ts (new) + test
- frontend/src/components/pages/PageWalk.vue (new)
- frontend/src/views/PageView.vue, PageView.test.ts
- frontend/src/i18n/locales/en.json, nl.json
- e2e/tests/pages.spec.ts, e2e fixture config
- docs-project/entities/guides/GUIDE-data-entry.md

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:**

- `walk` comes from operator config, validated at load.
- `?item=` is untrusted. It is only ever passed to `/_position` and to the
entity read, both gated. A hidden id answers the same 404 as a missing one.
- `list_id` in the scope is an id, never an expression; it selects a
condition the operator declared and can only narrow the set.

**Security-Sensitive Operations:**

Read-side ACL. Rows, the total and prev/next all come from the gated list
pipeline, so a hidden row is never counted or linked.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:** see the test named under each acceptance criterion.

**Edge Cases:**

- One row: previous and next disabled; PageDown goes to the next step.
- No rows: empty state; PageDown goes to the next step.
- `?item=` of a hidden or filtered-out row: "not in this step" with a link to
the first row.
- A row edited so it leaves the list: the next recount shows the "not in this
step" state.
- Walk step on an entity page: rows narrowed to the anchor.

**Negative Tests:** `walk` on a tabs page; `walk` on a kanban tab; `list_id`
with a search or pile scope (400).

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:**

- `EntityDetail` is large and route-aware; embedded under a page it might
react to the page's query. Mitigation: embed it as the detail panel does
(`followOwner=false`) and verify in e2e.
- A list condition on `/_position` turns off the store-paged path for that
scope. Mitigation: only when `list_id` names a list with a condition, the same
trade the list page already makes.

Effort: m.

## Documentation Planning

For enhancements: identify what documentation needs updating.

- [x] User-facing docs identified (skip if internal refactor)
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:**

- [x] docs/data-entry.md (via docs-project GUIDE-data-entry): steps section

## Design Review

- [x] Run `/design-review` before starting implementation
- [x] All critical/significant findings addressed in plan

**Design Review Findings:** self-review, recorded as review-responses on the
ticket: the position endpoint ignored the list's condition (addressed with
`list_id`), and the walk must read its first row and its scope from one builder
(addressed with `listBaseParams`).
