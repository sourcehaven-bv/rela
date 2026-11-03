---
id: RR-3XN6R5
type: review-response
title: Integer axis comparator was not a total order
finding: Mixing numeric and string comparison is not a valid ordering when a value is non-numeric.
severity: nit
resolution: 'Numbers sort first, then non-numeric values by text. Test: TestAxisValues_IntegerOrderIsTotal.'
status: addressed
---
