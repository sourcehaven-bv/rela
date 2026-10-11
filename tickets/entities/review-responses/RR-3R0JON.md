---
id: RR-3R0JON
type: review-response
title: R2 locking on invalidate, set and delete untested
finding: Removing the lock passes all tests; persist retry untested.
severity: minor
resolution: 'Broker tests: Invalidate, Set and Delete time out while the lock is held; Put failing once still stores, twice errors.'
status: addressed
---
