---
id: RR-M7MC2HY
type: review-response
title: Registration errors before Run go to the app error handler
finding: Registration errors before Run go to the app error handler
severity: minor
resolution: Sync runs only after the app has started (project load happens after Run); a failed Register is logged.
status: addressed
---
