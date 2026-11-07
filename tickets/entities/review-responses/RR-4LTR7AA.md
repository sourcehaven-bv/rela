---
id: RR-4LTR7AA
type: review-response
title: Close doc contradicts the stuck-handler paragraph
finding: The new opening sentence claimed Close stops every goroutine, while the next paragraph says the waiter outlives Close when ctx ends first.
severity: minor
resolution: 'Reworded to: once the handlers have returned, no goroutine the queue started remains.'
status: addressed
---
