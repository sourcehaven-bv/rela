---
id: RR-ECF20IP
type: review-response
title: Busy, full and conflict errors carry no context
finding: Errors did not name flow or subject, and rekey iterated a map.
severity: nit
resolution: Busy and full errors name the flow and subject id in both backends; rekey checks flows in sorted order.
status: addressed
---
