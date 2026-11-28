---
id: RR-8Q3CQ7P
type: review-response
title: Hop CTEs read before the cap
finding: pg hop CTEs were materialized before the LIMIT; sqlite had no timeout and an unindexed json_each seen anti-join.
severity: significant
resolution: HopCap per pg hop level; sqlite 5 s timeout and a keyed temp seen table; stale comment fixed.
status: addressed
---
