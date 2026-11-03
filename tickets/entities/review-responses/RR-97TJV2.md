---
id: RR-97TJV2
type: review-response
title: Binding skips parts of the upload route's preflight
finding: Upload slot, the configured size cap, and the rejection audit are outside attachmentWritePreflight; the binding receives only an address.
severity: significant
resolution: 'Plan: extract slot+cap+preflight+write+rejection audit into one transport-free core that the PUT route and the binding share; resolve the stored type through the gated reader first.'
status: addressed
---
