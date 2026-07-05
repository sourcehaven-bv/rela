---
id: FEAT-BL2E2X
type: feature
title: Relation fields on kanban cards
summary: Allow kanban card fields to render relation targets (incoming or outgoing), not just properties
description: Kanban card fields currently support only entity properties (KanbanCard.Fields is []ViewSectionField, which carries only Property). Add support for rendering a relation target on a card, in either direction, so boards can show e.g. the responsible person per task.
priority: medium
status: proposed
---

## Use case

A task kanban board wants each card to show the responsible person, where the
edge runs `persoon --verantwoordelijk_voor--> taak` (i.e. incoming to the taak).
Today card fields can only show scalar properties of the taak, so there is no
way to surface the assignee on the card at all — incoming or outgoing.

## Solution

Introduce a dedicated `KanbanCardField` type (`Property`, `Relation`,
`Direction`, `Label`) instead of widening the shared `ViewSectionField` (which
is reused by form relations, side panels, and view sections). Render
`Relation`-typed fields via the same direction-aware resolution used by
list/section relation columns, and plumb the values to the SPA card renderer
(`KanbanView.vue` `getCardFieldValue`), including the `?include=*` fetch the
board does not currently request.

Backward compatible: existing `- property: X` card fields parse and render
unchanged.

## Relation to existing work

Depends on the mechanism chosen for restoring incoming relation columns in lists
(the ID+`included` client-resolution path) so the two read surfaces share one
approach rather than inventing a card-specific title map.
