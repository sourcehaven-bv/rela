---
id: RR-68KZPU
type: review-response
title: World-absent page ignores mentioned_by_panel false
finding: writeWorldAbsentView built its response without HideMentionedByPanel, so the panel the operator disabled appeared on a world-absent page.
severity: minor
resolution: The world-absent response applies the entity_views switch (not the backlinks step, since no sections render there). Pinned in TestFaceOrder_WorldAbsentPageUsesDeclarationOrder.
status: addressed
---
