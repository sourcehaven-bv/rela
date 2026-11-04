---
id: RR-H6UMX4Q
type: review-response
title: leaveDecision depended on watcher timing
finding: leaveDecision only left while the composer's props still marked the type as a button type, which held only by watcher order.
severity: minor
resolution: 'Removed leaveDecision: dropping the record clears compose and the composer leaves a button type when compose stops naming it (Plan 2 and 3).'
status: addressed
---
