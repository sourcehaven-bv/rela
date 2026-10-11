---
id: RR-78FY7T
type: review-response
title: Scratch schema DROP has no deadline
finding: A hung DROP blocks Close forever.
severity: nit
resolution: Cleanup uses context.WithTimeout(context.WithoutCancel(ctx), scratchDropTimeout).
status: addressed
---
