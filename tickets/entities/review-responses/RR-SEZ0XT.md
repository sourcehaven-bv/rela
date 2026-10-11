---
id: RR-SEZ0XT
type: review-response
title: Store fault renders as an empty pile
finding: ResolveHeaders logs and returns nil on a read error, so a fault would show 0 items.
severity: minor
resolution: 'Plan: readableItems uses the error-returning ResolveIDsErr and the handler returns 500.'
status: addressed
---
