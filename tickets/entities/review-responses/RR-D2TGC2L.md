---
id: RR-D2TGC2L
type: review-response
title: Forwarding decorator could widen its query and stay exempt
finding: A decorator assigning q.IDs = nil before forwarding stayed exempt.
severity: nit
resolution: 'A parameter counts as forwarded only when the function never assigns it, a field of it, or takes its address. Test: decorator widening its query.'
status: addressed
---
