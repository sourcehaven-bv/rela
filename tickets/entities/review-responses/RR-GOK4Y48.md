---
id: RR-GOK4Y48
type: review-response
title: River deadline check before a retry uses the base delay
finding: The pre-retry deadline check used the 15s base, while the next attempt waits retryDelay(n). A job whose deadline fell in between was woken only to be cancelled.
severity: minor
resolution: 'Addressed: the check uses retryDelayFloor(n), the soonest the next attempt can run.'
status: addressed
---
