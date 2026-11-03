---
id: RR-01UDEW
type: review-response
title: Timeout return time depends on hidden writes
finding: 'Security review: a pass started by any store event, hidden ones included, could run up to 15 s past the timeout, so how late a timeout returned showed write activity.'
severity: minor
resolution: Each pass now runs under a deadline of min(pass budget, call deadline), and slot waits use that ctx, so the answer comes by the timeout. Documented in docs/acl-security.md. TestWatch_PassBoundedByTimeout pins it.
status: addressed
---
