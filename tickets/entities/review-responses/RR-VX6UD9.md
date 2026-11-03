---
id: RR-VX6UD9
type: review-response
title: Budget test guard passes when only one row resolves and has no incoming column
finding: The vacuity guard only checked that Feature 1 and Person 1 appear somewhere. That passes if only the first row resolves and Feature 1 also matches Feature 10. The list had no incoming column so a later per-row incoming read would not move the count.
severity: minor
resolution: The guard now requires every rendered row to resolve its Feature and Assignee cells and at least 10 rows. The test adds an incoming blocks column and checks it resolves on TKT-0001.
status: addressed
---
