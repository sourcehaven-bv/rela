---
id: RR-DUBI8Y7
type: review-response
title: An epoch timestamp diverges between backends
finding: micros() maps the epoch to 0, which SQL reads as unset.
severity: nit
resolution: flowrun rejects a create or update time, and a due time, at or before the epoch; unit tests added.
status: addressed
---
