---
id: RR-TD45CX4
type: review-response
title: Elevation scope read before the reload check
finding: The reload check ran only for available_on actions, so one request could run on a narrowed scope.
severity: nit
resolution: actionOps.run returns 409 config_reloaded for an elevated action when the config changed.
status: addressed
---
