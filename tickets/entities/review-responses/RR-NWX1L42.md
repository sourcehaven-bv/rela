---
id: RR-NWX1L42
type: review-response
title: apply does not stop on cancellation between rows
finding: After the deadline every remaining write failed and flooded the report.
severity: minor
resolution: apply checks ctx.Err before every row and gone row and returns.
status: addressed
---
