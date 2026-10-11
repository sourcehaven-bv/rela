---
id: RR-PRMRFX
type: review-response
title: Gate or store fault reads as no backlinks
finding: applyViewTraverse has no error path; dropping MentionedBy errors would make a fault look like an empty backlink list while the built-in panel is hidden.
severity: significant
resolution: A resolver fault returns gateFaultError and the view fails through writeGateError. TestViewMentions_FaultFailsTheView pins the non-200 and that the raw error stays off the wire.
status: addressed
---
