---
id: RR-RVPDJ0L
type: review-response
title: Face on a local tail end is silently ignored
finding: checkBatchEnds refuses only a face on the head.
severity: minor
resolution: checkBatchEnds refuses a face on a local From; the handler refuses a face on any ref end. Shape test case 'local tail with a face'.
status: addressed
---
