---
id: RR-FHYMGH
type: review-response
title: Purge record of a face batch does not say what was erased
finding: channels.PurgeAudit carries only face, author and time. Face batches file no trail, so the purged trail holds no type, name or count of the erased channels.
severity: nit
resolution: Purge writes one record per erased channel with its type, name and tomb stamp.
status: addressed
---
