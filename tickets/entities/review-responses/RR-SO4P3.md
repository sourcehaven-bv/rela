---
id: RR-SO4P3
type: review-response
title: splitNamespaced emits identical namespace_hash for bare keys (round 2)
finding: If a future caller slips a bare key past the namespacing layer, logFields emits namespace_hash=sha256('')[:16]='e3b0c44298fc1c14' — a constant that looks plausible but collides across every improperly-namespaced entry, misleading operators trying to debug.
severity: minor
resolution: 'Added unnamespacedMarker constant ''<unnamespaced>'' emitted in logFields when the namespace part is empty. Operators grep-ing logs for namespace collisions see the literal sentinel instead of a plausible hash. New test: TestLogFieldsBareKeyEmitsUnnamespacedMarker.'
status: addressed
---
