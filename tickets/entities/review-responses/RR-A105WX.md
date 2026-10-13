---
id: RR-A105WX
type: review-response
title: Promoted.Type not validated or capped
finding: PrepareAction checks Promoted.ID but not Promoted.Type; any length is copied into audit records.
severity: nit
resolution: Promoted.Type must be 1 to 64 bytes of UTF-8 without control characters.
status: addressed
---
