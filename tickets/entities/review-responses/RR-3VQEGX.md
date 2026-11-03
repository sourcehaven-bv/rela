---
id: RR-3VQEGX
type: review-response
title: Datetime min, max and latest_by order by raw text
finding: Datetimes with an offset or a fraction sort out of time order as text; every backend agrees on the wrong order.
severity: significant
resolution: 'rollup.Compile refuses datetime for min, max and latest_by (isoDate). Backlog ticket filed for chronological datetime ordering. Tests: TestCompile_Rejects latest_by a datetime.'
status: addressed
---
