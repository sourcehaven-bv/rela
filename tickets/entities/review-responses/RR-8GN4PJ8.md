---
id: RR-8GN4PJ8
type: review-response
title: Threshold should use the longest array, not the sum
finding: A generic plan filters each array separately, so the sum of all ids overstates the risk and makes small batches one-shot.
severity: minor
resolution: idBatchPage takes the longest array for the threshold and the total for the page size.
status: addressed
---
