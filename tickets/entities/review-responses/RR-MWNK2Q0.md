---
id: RR-MWNK2Q0
type: review-response
title: Alert job ignored cancelled post-merge runs
finding: The alert only fired on failure, so a cancelled or timed-out run filed nothing.
severity: minor
resolution: The alert now treats any result other than success as a failed run.
status: addressed
---
