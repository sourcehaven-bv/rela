---
id: RR-8BP7R4
type: review-response
title: Panics are recorded as golden content
finding: serveRecovering records any panic with the recorder's default 200; an -update would bake a regression in.
severity: significant
resolution: A handler panic fails the case with its stack; no status is recorded for it.
status: addressed
---
