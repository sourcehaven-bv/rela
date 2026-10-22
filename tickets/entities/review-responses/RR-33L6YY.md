---
id: RR-33L6YY
type: review-response
title: New test builds its PATCH request by hand
finding: The dataentry test called handleV1UpdateEntity directly to attach a principal.
severity: nit
resolution: Added a patchEntityBy helper; patchEntity delegates to it.
status: addressed
---
