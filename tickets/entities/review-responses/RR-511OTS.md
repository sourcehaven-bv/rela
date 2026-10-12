---
id: RR-511OTS
type: review-response
title: pglease write paths lock rows in opposite orders
finding: 'Code review: Claim locks lease_counters then lease rows; Release/Drop/Renew/Rekey lock a lease row then the counter, so a release of a lapsed lease during a claim can deadlock (40P01); Rekey can hit an unhandled 23505.'
severity: significant
resolution: 'Fixed in 53753571 (Lease storage: review fixes); see the commit for the change and its conformance test.'
status: addressed
---
