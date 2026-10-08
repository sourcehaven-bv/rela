---
id: RR-0FETEE
type: review-response
title: Read-only cards dropped in another column failed silently
finding: On a reorderable board every card can be picked up; a cross-column drop of a card the reader may not update returned without feedback.
severity: minor
resolution: The drop shows an error toast. KanbanView.reorder.test.ts.
status: addressed
---
