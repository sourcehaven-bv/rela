---
id: RR-OXR6M0
type: review-response
title: 'Design: capture-now has no schema projection'
finding: Only the sweep holds the ProjectionProvider; insertVersion needs a schema hash.
severity: significant
resolution: Provider injected into both version stores; shared capture function extracted from captureOne (R4).
status: addressed
---
