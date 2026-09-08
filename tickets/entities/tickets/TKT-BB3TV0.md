---
id: TKT-BB3TV0
type: ticket
title: Make list rows and kanban cards behave as real links
kind: enhancement
priority: medium
effort: m
status: done
---

## Description

List rows and kanban cards navigate via a JavaScript `@click` handler rather
than a real hyperlink, so the browser's own navigation affordances do not work:

- right-click shows no "Open Link in New Tab" / "Copy Link"
- Cmd/Ctrl+click does nothing
- middle-click does nothing

That breaks a common workflow — opening several entities in tabs to compare them
side by side.

GitHub issue #1172.

## Current state

`EntityList.vue` renders `<tr class="entity-row"
@click="navigateToEntity(...)">` and `KanbanView.vue` renders cards with
`@click="openCard(entity)"`. Neither emits an `<a href>`.

A helper already exists — `entityDetailHref` in `utils/entityRoute.ts` — and its
call site in `EntityList` even carries the comment *"Centralised so right-click
/ middle-click open through a real `<a href>` on the row markup elsewhere."* The
helper landed for the command palette; the row markup never followed.

## Constraints

The row contains interactive controls — selection checkboxes, action buttons,
drag-and-drop on kanban. Those must keep working, which is the reason the naive
"wrap the row in an anchor" answer does not apply to a `<tr>`.
