---
id: RR-33RYC30
type: review-response
title: Placeholders after ? were path-escaped
finding: '{run}, {type} and {id} used url.PathEscape even in the query part of a navigate link.'
severity: nit
resolution: fillLink escapes path segments before the first ? and query values after it. TestFlowTasks_LinkEscaping.
status: addressed
---
