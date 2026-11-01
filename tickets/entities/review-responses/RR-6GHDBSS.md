---
id: RR-6GHDBSS
type: review-response
title: Gated CTE runs visibility checks twice
finding: With a query scope each matching face runs the EXISTS checks in the CTE and again in the inner query.
severity: minor
resolution: 'Fixed: noted in the comment. It costs time and not correctness.'
status: addressed
---
