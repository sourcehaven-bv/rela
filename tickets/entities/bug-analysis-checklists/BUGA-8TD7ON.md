---
id: BUGA-8TD7ON
type: bug-analysis-checklist
title: 'Analysis: Edit form drops the world and hides relations to faced peers'
started: "2026-10-06"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Reproduction

- [x] Bug reproduced locally
- [x] Minimal reproduction steps documented
- [x] Environment/conditions noted

Reproduced in the faced e2e project. `CTL-3 --mitigates--> POL-2`, where
POL-2 has only a draft face. The default world (`published`) does not serve
POL-2; `editorial` does. `GET /api/v1/controls/CTL-3` returns no `mitigates`
entry, and the same request with `?world=editorial` returns `["POL-2"]`. In
the SPA: open CTL-3 with `?world=editorial`, the detail page shows POL-2.
Click Edit: the URL loses `?world=` and the "Mitigates" picker is empty.

`e2e/tests/faces-edit-relations.spec.ts` fails on develop at the picker tile
assertion. With that assertion made soft, the rest passed: adding POL-1 kept
the hidden POL-2 edge, because since #1753 the picker saves a delta. Before
#1753 (up to v26.10.1) the picker sent a full `{data: [...]}` set, which
replaces, so the same save deleted the hidden edge.

## Root Cause

- [x] Immediate cause identified (why1)
- [x] Contributing factors found (why2-3)
- [x] Systemic cause explored (why4-5)

Recorded on the bug. In short: every edit entry point dropped `?world=`, and
`DynamicForm.loadEntity` fetched without a world. The entity GET resolves
relations per world, so the form loaded the default world's relation set.

## Fix Planning

- [x] Fix approach determined
- [x] Regression test planned
- [x] Related areas checked for similar issues

**Approach.** One helper, `editFormRoute(formId, address, world, query)` in
`frontend/src/utils/entityRoute.ts`, used by every edit entry point:
EntityDetail (Edit link, `e` shortcut, section row edit), EntityList,
KanbanView, SidePanel, EntityPreviewModal and DocumentView.
`DynamicForm.loadEntity` passes the form's world to `fetchEntity`. The world
is the route's for a page form and the host's for an embedded one (renamed
`createWorld` to `formWorld`). The picker already searches in the route's
world, so it now matches the loaded set.

Not chosen: making the entity GET return every stored edge regardless of
world. That changes world semantics for every reader and the ACL-filtered
read path. Sending only a diff on save is already in place since #1753.

**Regression test.** E2E `faces-edit-relations.spec.ts`: open a control in the
editorial world, click Edit, see the draft-only link, add another, and check
that both links are stored. Unit tests: `editFormRoute`, DynamicForm fetches
in the route's world, and the updated route assertions in the EntityDetail,
EntityList and Kanban suites.

**Related areas.** The cards widget (`RelationCards`) loads its edges from
`/relations/{rel}`, which serves the default world only (422 for any other).
It therefore has the same display gap for a faced peer outside the default
world. Its saves are deltas, so nothing is lost. Left out of this fix because
it needs a server change to the relations route; noted on the bug.
