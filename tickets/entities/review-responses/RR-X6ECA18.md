---
id: RR-X6ECA18
type: review-response
title: 'Code review: re-check after open uses request context'
finding: A cancelled first request would discard a successfully opened tenant.
severity: minor
resolution: Registry.recheck runs on context.WithoutCancel with a 10s timeout.
status: addressed
---
