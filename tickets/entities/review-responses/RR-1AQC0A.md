---
id: RR-1AQC0A
type: review-response
title: guarded() only recognises plain NopAnchorGuard
finding: newChannelGuard silently returns Nop on a nil store, and guarded() type-checks only the plain value, so a wrapped Nop runs the pre-pass.
severity: nit
status: open
---
