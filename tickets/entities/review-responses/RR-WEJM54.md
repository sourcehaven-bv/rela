---
id: RR-WEJM54
type: review-response
title: Kanban/RelationCards drag asymmetry correct but undocumented; RR-NPDW9A's test never written
severity: significant
resolution: 'Added a comment at the Kanban card explaining why it deliberately has NO draggable=false (the card is both the link and the drag source; the attribute is for an anchor nested inside one, and setting it here would disable reordering, while onDragStart sets dataTransfer unconditionally). Verified both drag surfaces still work: kanban.spec.ts 15/15 including ''drag updates the entity status'', relation-cards.spec.ts 12/12.'
status: addressed
---

**Finding (S3, significant).** The Kanban/RelationCards drag asymmetry is
CORRECT but undocumented, and the test RR-NPDW9A asked for was never written.

Why Kanban legitimately differs: RR-NPDW9A's `draggable="false"` prescription
targets an anchor *child* of a drag source. In Kanban the card IS both the
anchor and the drag source, so `draggable="false"` would disable dragging
entirely; the `:draggable="canUpdate(entity)"` binding is the right control, and
`onDragStart` (`:446-452`) unconditionally sets `dataTransfer`, overriding the
native link-drag in both branches. RelationCards genuinely needed the attribute
because its anchors are children of the draggable card AND its `onDragStart`
early-returns when `!isOrderable`.

Two gaps:

1. That reasoning exists nowhere in the code. The next person reads RR-NPDW9A,
sees no `draggable="false"` on the Kanban card, and "fixes" it — breaking drag.
It belongs in a comment at the `:draggable` binding.
2. RR-NPDW9A was closed as addressed with "add a test that reorder-by-dragging
still works". No such test exists. Kanban has e2e coverage
(`kanban.spec.ts:83-100`, passing); **RelationCards drag-reorder has none at any
level** — and it is the component where the anchor/drag interaction is subtlest.
