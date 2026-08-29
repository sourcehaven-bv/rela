---
id: RR-NPDW9A
type: review-response
title: Anchors inside RelationCards/Kanban drag sources break drag-reorder
severity: critical
status: addressed
---

**Finding (C3, critical).** Making `RelationCards.vue:562/565` spans into
anchors breaks drag-reorder. The plan's Risks table only considered Kanban drag.

These spans sit inside the card at `:545-556`, which is
`:draggable="isOrderable"` with a full dragstart/dragover/drop set. Two
problems:

1. Anchors are natively draggable. Unlike Kanban (`KanbanView.vue:446` unconditionally sets `text/plain`), `RelationCards.onDragStart` (`:458-459`) **early-returns when `!isOrderable`** — verified. So on a non-orderable list, dragging the new anchor gets the browser's default link-drag (dragging a URL), a behaviour that does not exist today.
2. The spans have no `.stop`, so clicks already bubble to the card. An anchor with a navigating default action inside a drag source is where "was that a drag or a click?" bugs live.

**Resolution:** `draggable="false"` on any anchor placed inside a drag source
(applies to Kanban cards too, where `:draggable="false"` on the card does not
stop an anchor child from being independently draggable). Add a test that
reorder-by-dragging-the-title still works.
