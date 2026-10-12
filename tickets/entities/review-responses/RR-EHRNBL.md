---
id: RR-EHRNBL
type: review-response
title: Change between fetch and subscribe is never signalled
finding: The baseline is taken when the PUT arrives, after the client fetched; a write in that gap is folded into the baseline and no frame follows. Same on reconnect for views.
severity: critical
resolution: 'New AC8: after a 204 the SPA refetches each newly added key once; new keys take their baseline without a frame.'
status: addressed
---
