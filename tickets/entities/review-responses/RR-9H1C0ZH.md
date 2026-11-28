---
id: RR-9H1C0ZH
type: review-response
title: Face and value from different arrivals
finding: pgstore took min(face) and min(val) separately and judged hit per arrival, unlike the reference.
severity: minor
resolution: Face, value and hit come from the earliest arrival on every backend; differential test covers mixed gates, faces, cap and extreme ints.
status: addressed
---
