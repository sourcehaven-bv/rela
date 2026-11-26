---
id: RR-WN7BXI5
type: review-response
title: A done branch silently wins over a kept event
finding: settleStep took a done branch and discarded a kept change without trace.
severity: minor
resolution: settleStep logs at Info when a done branch takes precedence; the guide documents the order.
status: addressed
---
