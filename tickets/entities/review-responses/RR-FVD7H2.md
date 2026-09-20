---
id: RR-FVD7H2
type: review-response
title: Window position must wrap the post-DISTINCT relation and handle empty OrderBy
finding: Ordinals would otherwise count candidate rows.
severity: minor
resolution: 'Plan: window over the finished graph select as a subquery; tests for world and empty OrderBy.'
status: addressed
---
