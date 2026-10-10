---
id: RR-IWVFO2
type: review-response
title: Upgrade leaves every row with a NULL hash, a one-time backlog
finding: After migration every row is a candidate; at Batch per Interval a large database drains slowly and new edits wait behind it.
severity: minor
resolution: A tick that fills its batch and makes progress now triggers the next tick at once (sweep.more), so a backlog drains in one run instead of one batch per Interval.
status: addressed
---
