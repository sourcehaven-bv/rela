---
id: RR-KL2YAX
type: review-response
title: Concurrent rename makes Renew drop a valid lease
finding: 'Code review: Lookup returns the old id, Rekey moves it, mayUpdate misses and Release drops the moved lease.'
severity: minor
resolution: 'Fixed in f9bddab4 (Lease service: review fixes) with a test; see the commit.'
status: addressed
---
