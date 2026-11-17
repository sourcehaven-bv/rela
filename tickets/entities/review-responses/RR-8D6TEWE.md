---
id: RR-8D6TEWE
type: review-response
title: fs store compared a parsed date edge value as a timestamp
finding: '[security] fsstore keeps a YAML date as time.Time; the naive matcher compared it as RFC3339 text; so ''<= 2026-12-31'' rejected the last day while SQL accepted it.'
severity: minor
resolution: The naive matcher compares a date-shaped time.Time as YYYY-MM-DD; matchEdge in aclstore now uses the same matcher. TestGraph_EdgeCondition covers it.
status: addressed
---
