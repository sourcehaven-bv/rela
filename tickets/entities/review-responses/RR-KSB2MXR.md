---
id: RR-KSB2MXR
type: review-response
title: No case for manage without a manual start
finding: managePermissions with manage set and no manual start is untested.
severity: minor
reason: managePermissions returns the manage permission before it reads the starts, so the start list cannot affect the result; the load test covers parsing.
status: wont-fix
---
