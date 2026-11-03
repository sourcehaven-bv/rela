---
id: RR-UQL473
type: review-response
title: Failed NextExpiry stops expiry wakes until the next write
finding: 'Code review: arm leaves the timer stopped on error and nothing re-arms it while Version is unchanged.'
severity: significant
resolution: 'Fixed in f9bddab4 (Lease service: review fixes) with a test; see the commit.'
status: addressed
---
