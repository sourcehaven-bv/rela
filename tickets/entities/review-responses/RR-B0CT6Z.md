---
id: RR-B0CT6Z
type: review-response
title: Conflict list and detail endpoints are ungated
finding: GET /_conflicts and /_conflicts/{path} return ids, values and content without the read gate. Pre-existing.
severity: minor
reason: 'Pre-existing and outside #1774; filed as BUG-Q3Z15V (high).'
status: deferred
---
