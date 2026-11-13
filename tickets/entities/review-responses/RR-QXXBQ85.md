---
id: RR-QXXBQ85
type: review-response
title: Subject membership check was quadratic
finding: slices.Contains per row made usersBySubject O(holders squared).
severity: minor
resolution: Use slices.BinarySearch on the sorted subjects.
status: addressed
---
