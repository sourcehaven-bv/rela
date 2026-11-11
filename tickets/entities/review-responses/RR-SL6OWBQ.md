---
id: RR-SL6OWBQ
type: review-response
title: Close must stop River before closing its pool
finding: Closing the pool while River is still stopping makes the stop fail and can leak goroutines.
severity: minor
resolution: 'Addressed: Close runs StopAndCancel and only then closes the pool in the same goroutine.'
status: addressed
---
