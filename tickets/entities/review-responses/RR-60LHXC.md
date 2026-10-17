---
id: RR-60LHXC
type: review-response
title: Jitter delays a single visible tab
finding: A lone tab that returns waits up to the jitter maximum before refreshing.
severity: nit
reason: The delay is bounded and short; detecting sibling tabs would need cross-tab coordination for little gain.
status: wont-fix
---
