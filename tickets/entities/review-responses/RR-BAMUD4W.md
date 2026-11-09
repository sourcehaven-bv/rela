---
id: RR-BAMUD4W
type: review-response
title: Concurrent first requests each open a full tenant
finding: Without single-flight N parallel cold requests opened N pools and ran migrations N times.
severity: significant
resolution: 'Acquire single-flights opens per org through a pending map; waiters share the result or error. Test: TestRegistry_ConcurrentFirstRequestsOpenOnce.'
status: addressed
---
