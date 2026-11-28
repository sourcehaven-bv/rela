---
id: RR-CDZUU4T
type: review-response
title: Ungated hop reads affect latency
finding: Hop and edge reads are not gated; hidden volume affects latency and can reach the pg 5 s timeout.
severity: minor
reason: 'Accepted residual: both outcomes fail closed; recorded in the threat model (design section 11 and GUIDE-acl-security.md).'
status: wont-fix
---
