---
id: RR-1VVZGLS
type: review-response
title: 'Design: every region state must be able to finish'
finding: A cycle in a region with no way to the join or a final state never arrives, so join all waits forever.
severity: minor
resolution: 'Plan updated: the load refuses a region state from which neither the join nor a final state is reachable.'
status: addressed
---
