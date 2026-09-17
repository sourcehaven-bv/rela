---
id: RR-CNQJVM
type: review-response
title: Test fixture was larger and slower than the straddle required
finding: The breadth fixture built ~8,300 entities and added ~4s to the package. Review noted the size was driven by the (wrongly fitted) flat limit, and that higher fan-out with fewer parents achieves the same straddle far more cheaply, since most of the cost is CreateRelation calls.
severity: minor
resolution: 'Fixture restructured to 85 parents x 75 children (fan-out 3x nestedChildPreview) instead of 276 x 30. An intermediate attempt at 8x fan-out was measured and rejected for being slower, not faster: 17,085 entities against the 6,460 that suffice. The TestQueryBudget suite now runs in ~2.9s, down from ~10s.'
status: addressed
---
