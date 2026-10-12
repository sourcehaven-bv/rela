---
id: RR-NNMMIO
type: review-response
title: Cancelled request ctx leaves a lease on a deleted entity
finding: 'Code review: alias hooks run on the request ctx; a disconnect makes the lease store call fail and the lease outlives the delete or rename; log text is CalDAV-specific.'
severity: significant
resolution: 'Fixed in f9bddab4 (Lease service: review fixes) with a test; see the commit.'
status: addressed
---
