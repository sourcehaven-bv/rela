---
id: RR-Y8X93JV
type: review-response
title: 'Design: size bounds and step 5 splitting'
finding: The cap must be below MaxRangeList; the cap bounds rows not bytes; mixed ranges list in full; Scope compile has no budget.
severity: minor
resolution: WithMaxRows accepts 1..MaxRangeList-1. Bodies are bounded by the row cap only; documented. Splitting at type windows is noted for step 5. The budget test covers the whole call including the compile.
status: addressed
---
