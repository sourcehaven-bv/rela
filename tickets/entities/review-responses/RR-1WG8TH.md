---
id: RR-1WG8TH
type: review-response
title: FindOrphans holds every header with its property map
finding: storedHeaders keeps every header of the store, with properties, to feed a fold that needs id, type and face.
severity: minor
reason: 'The gate needs the property maps for when: predicates, so they cannot be dropped at the read. A streaming gate is a larger change to Resolver; the read count is already constant.'
status: deferred
---
