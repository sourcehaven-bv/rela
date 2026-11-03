---
id: RR-LYTQZL
type: review-response
title: Waiter goroutine outlives a timed-out Close
finding: When ctx ends first, the goroutine waiting on the WaitGroup lives until the handlers return.
severity: minor
resolution: Documented on Close.
status: addressed
---
