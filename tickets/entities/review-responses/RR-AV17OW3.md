---
id: RR-AV17OW3
type: review-response
title: One findByRef graph query per new row
finding: An initial pull of N rows runs N ref lookups.
severity: minor
reason: Bounded by max_fetch per run and not on a read path; batching the ref lookup per page is a performance follow-up for TKT-6GRZHXD.
status: deferred
---
