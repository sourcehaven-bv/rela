---
id: RR-WISFA4
type: review-response
title: attach_render is never idempotent
finding: Each call stores a new file, so an on-update automation re-triggers itself up to the hop limit.
severity: minor
reason: 'The loop is bounded by the hop limit and documented with a when: guard. Skipping a write whose hash and name match changes the write semantics for max: N properties and needs its own design; it is not in the approved brief.'
status: deferred
---
