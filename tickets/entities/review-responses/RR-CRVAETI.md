---
id: RR-CRVAETI
type: review-response
title: Wire must never set SkipAutomation or WriteExternalRefs
finding: CreateOptions trusts these; the batch request type must not carry them.
severity: nit
resolution: v1.BatchEntity has no such fields and the decoder refuses unknown fields; TestBatch_RequestLimits sends skip_automation and gets 400.
status: addressed
---
