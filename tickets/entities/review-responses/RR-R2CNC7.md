---
id: RR-R2CNC7
type: review-response
title: Engine.WithOptions mutated a shared receiver
severity: minor
status: addressed
finding: 'Engine.WithOptions mutated its receiver and returned it, a builder on a value documented as safe for concurrent use -- and the handler doc invites caching an engine, which would have made it a live data race.'
resolution: 'Replaced with a construction-time functional option, nextaction.WithOptions(fn) passed to New. Chosen over returning a copy because the tests revealed the builder shape is a footgun either way: a copy-returning builder is silently a no-op when the caller discards the result, which three tests did. Neither mistake is available now.'
---
