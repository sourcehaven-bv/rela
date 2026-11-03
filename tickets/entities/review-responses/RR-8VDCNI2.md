---
id: RR-8VDCNI2
type: review-response
title: Early return hides other data-entry errors
finding: CheckDataEntry returns on deprecated syntax before structural validation, so other errors appear only after migrating.
severity: nit
resolution: Deliberate, matching the server; documented in the CheckDataEntry comment.
status: addressed
---
