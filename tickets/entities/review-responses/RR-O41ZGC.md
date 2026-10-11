---
id: RR-O41ZGC
type: review-response
title: rela-server reload does not share the lease backend
finding: 'Code review: each Discover generation gets its own backend and hub; the memory fallback drops leases on Configure save.'
severity: minor
resolution: 'Fixed in f9bddab4 (Lease service: review fixes) with a test; see the commit.'
status: addressed
---
