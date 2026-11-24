---
id: RR-LEBY5K4
type: review-response
title: Stale optimistic row after a non-latest failed move
finding: A failed move that is not the latest keeps its optimistic position until the queue refetch.
severity: nit
reason: Same behaviour as useListReorder. The refetch after the queue drains corrects the screen.
status: wont-fix
---
