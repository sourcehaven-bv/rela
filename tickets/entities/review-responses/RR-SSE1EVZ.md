---
id: RR-SSE1EVZ
type: review-response
title: Display rounds away over-scale digits
finding: format.ts uses scale for both min and max fraction digits so 12.345 on scale 2 shows as 12.35.
severity: minor
reason: Deferred to a follow-up decimal ticket; display only; analyze properties and the save warning still report the value.
status: deferred
---
