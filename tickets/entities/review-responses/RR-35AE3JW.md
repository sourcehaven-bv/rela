---
id: RR-35AE3JW
type: review-response
title: Purge marker written for every purged entity
finding: A marker per purged entity grows state.KV without bound.
severity: significant
resolution: The generation is written only when the schema has on.deleted triggers.
status: addressed
---
