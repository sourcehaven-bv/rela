---
id: RR-0LIIT7
type: review-response
title: Generic error log could carry the lease id
finding: 'Security design review: in renew/release the only id before Lookup is the lease id.'
severity: minor
resolution: 'Plan: log the entity id only after Lookup; never the lease id; log-capture test on error paths.'
status: addressed
---
