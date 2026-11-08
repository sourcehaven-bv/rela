---
id: RR-K8872GP
type: review-response
title: 'Code: current world link reset pagination'
finding: The current world link went through worldQuery, which drops page.
severity: minor
resolution: The current world links to route.fullPath. Covered by WorldSwitcher.test.ts.
status: addressed
---
