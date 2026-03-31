---
id: RR-FYP2
type: review-response
title: Non-deterministic output order
finding: Map iteration in outputValidationViolations is non-deterministic. For consistent output, rule names should be sorted.
severity: minor
resolution: Added sort.Strings() to sort rule names before iterating, ensuring deterministic output order.
status: addressed
---
