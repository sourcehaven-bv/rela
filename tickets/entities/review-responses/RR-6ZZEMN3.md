---
id: RR-6ZZEMN3
type: review-response
title: Error.Range missing on planning errors about one range
finding: planReconcile errors such as an unknown type in range 3 were sent without the range member.
severity: minor
resolution: planReconcile returns a reconcileRangeError; the handler writes it with the range index. TestReconcile_PlanErrorNamesRange.
status: addressed
---
