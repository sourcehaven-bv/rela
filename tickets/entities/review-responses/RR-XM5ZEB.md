---
id: RR-XM5ZEB
type: review-response
title: Outgoing card fields render raw ID for ACL-hidden targets (inherited)
finding: 'When `included` lacks a relation target the SPA falls back to rendering the raw ID (KanbanView.vue:289-292). For OUTGOING fields this is live now: outgoingRelations returns target IDs with no ACL filtering, while included is ACL-filtered, so an outgoing card field pointing at a target the viewer cannot read renders that target''s raw ID on the card. Inherited from EntityList relation columns, not introduced here, but this ticket newly surfaces it on kanban cards. Same root cause as the ODHV2D critical; the clean fix (omit IDs whose target isn''t visible) is server-side and belongs with the ODHV2D ACL fix. Confirm the product decision or fix server-side.'
severity: significant
resolution: 'Confirmed KanbanView''s included ? title : id fallback matches EntityList.vue exactly (both render the raw ID); kept consistent, no SPA divergence, documented in a comment. The actual leak is closed server-side by ODHV2D''s visibleRelationIDs gate (same list endpoint), which removes hidden neighbor IDs from the relations map before they can reach this fallback — noted in the comment. Commit 5a0f8e0a.'
status: addressed
---
