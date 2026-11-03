---
id: RR-98FO5N
type: review-response
title: Shape-cap eviction could hide the hot statement
finding: Once 128 distinct statements were recorded, new ones were dropped, so a hot statement starting late was never named.
severity: minor
resolution: 'Space-Saving replacement: past the cap a new text replaces the least-counted one and inherits its count; cap raised to 256. TestQueryStats_ShapeCapIsBounded asserts a late hot statement is reported.'
status: addressed
---
