---
id: RR-K31DAOU
type: review-response
title: lead creates a verify ticker for a lock without a check
finding: The verify ticker fires every takeover interval even when check is nil (lock file, memory).
severity: nit
resolution: lead only creates the ticker when there is a check; a nil channel never fires.
status: addressed
---
