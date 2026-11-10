---
id: RR-WO1JSPZ
type: review-response
title: Subject addresses differ between endpoints
finding: RunList and Tasks take the subject from the _carries relation (a bare id) while Run uses run.Subject with its face.
severity: nit
reason: Only affects faced subject types, which no flow uses yet; recorded on TKT-FWQZGAM.
status: deferred
---
