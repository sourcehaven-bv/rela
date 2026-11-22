---
id: RR-V5EI3XG
type: review-response
title: Quadratic owner dedup
finding: slices.Contains in the row loop is quadratic over up to 1000 rows.
severity: nit
resolution: Deduplicated with a seen map.
status: addressed
---
