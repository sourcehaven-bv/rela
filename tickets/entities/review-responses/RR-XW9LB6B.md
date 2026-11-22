---
id: RR-XW9LB6B
type: review-response
title: Paging check in assertHiddenFieldPages was inverted
finding: The Limit/Offset check ran only when nothing matched, so paging through a guarded Where was never tested.
severity: significant
resolution: The check now runs when two or more rows match and asserts the second row and the full total.
status: addressed
---
