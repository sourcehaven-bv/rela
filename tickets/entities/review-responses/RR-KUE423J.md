---
id: RR-KUE423J
type: review-response
title: First row and position from one builder
finding: If the walk built its first-row read and its position scope separately, they could disagree on sort or filters, and the first row would not be row 1.
severity: minor
resolution: Both are built from listBaseParams plus the page scope.
status: addressed
---
