---
id: RR-GARBRT
type: review-response
title: Watch wakes every subscriber on a plain renew
finding: 'Code review: Store.Renew always moves Version and the expiry timer wakes when the earliest lease was extended, so live views recompute on every renew.'
severity: significant
resolution: 'Fixed in f9bddab4 (Lease service: review fixes) with a test; see the commit.'
status: addressed
---
