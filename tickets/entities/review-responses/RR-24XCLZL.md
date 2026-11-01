---
id: RR-24XCLZL
type: review-response
title: Missing unit tests for card derivations
finding: Ordering rules were tested only through the mounted view.
severity: minor
resolution: utils/dashboardCards.test.ts covers ordering, tie-break, unset, redaction, mixed types, groupKey, non-enum list breakdown and link cells.
status: addressed
---
