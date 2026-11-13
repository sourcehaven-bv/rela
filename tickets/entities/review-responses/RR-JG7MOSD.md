---
id: RR-JG7MOSD
type: review-response
title: Question cap doubles as a read-side damage check
finding: CheckAsked refused a stored set over MaxAsked on read; lowering the cap would make channels unreadable.
severity: minor
resolution: CheckAsked checks structure only; PlanBatch holds writes to the cap.
status: addressed
---
