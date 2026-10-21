---
id: RR-ATK6IN
type: review-response
title: Unique conflict status
finding: Edge cases said 409 but the create core returns a 422 that names no colliding id.
severity: minor
resolution: 'Plan: pass the create core error through as 422 validation_failed.'
status: addressed
---
