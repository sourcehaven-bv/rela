---
id: RR-5HFYZMJ
type: review-response
title: Quick-add cards vanish behind board filters
finding: A card that does not match the board filters or query_scope is created but never shown. The facilitator types it again.
severity: significant
resolution: After the refetch the board checks whether the new card is visible and shows an info message when the filters hide it. Tested in KanbanView.quickAdd.test.ts and documented.
status: addressed
---
