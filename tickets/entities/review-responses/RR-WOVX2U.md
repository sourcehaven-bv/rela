---
id: RR-WOVX2U
type: review-response
title: Outlier on an unbounded integer axis hides the real values
finding: Filling lowest..highest and cutting at 50 lets one outlier push every real value off the axis; desc also cut from the low end.
severity: significant
resolution: When an undeclared span is wider than 50 the axis lists only the values that occur, capped at 50 from the end shown first (desc keeps the highest). Tests cover outlier, cap and desc.
status: addressed
---
