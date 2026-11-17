---
id: RR-K8RT5G1
type: review-response
title: Ordered nil on dates became silently false
finding: Under the lowering option an unset date or decimal in an ordered comparison read as false instead of erroring, so a view condition over a missing due date dropped rows without a warning.
severity: significant
resolution: NilOrderedIsFalse now applies only when the other operand is an integer or a string. Dates, decimals, numbers and nil against nil still error. Tests pin each case and the guides say so.
status: addressed
---
