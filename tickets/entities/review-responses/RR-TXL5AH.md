---
id: RR-TXL5AH
type: review-response
title: Script-error detail built separately from the envelope
finding: scriptErrorDetail re-derived the degraded detail and dropped the entity id.
severity: minor
resolution: scriptErrorDetail now renders buildScriptErrorEnvelope(se, false, ...) and includes the entity id.
status: addressed
---
