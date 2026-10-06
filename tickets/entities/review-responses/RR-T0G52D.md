---
id: RR-T0G52D
type: review-response
title: Extra header query per traversal level without a budget test
finding: ownedEdges adds one header batch per traversal level for content relations; no storetest.Counting test pins it.
severity: minor
reason: The cost is one batch per level and does not grow with rows, so TKT-1U8XYN's rule holds. This is Stage 0 using existing helpers; the Stage 1 resolver (TKT-2528AB) replaces this per-surface resolution and is where a budget test belongs.
status: deferred
---
