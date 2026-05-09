---
id: RR-4P4U
type: review-response
title: Watcher leaks stale property values into propCache on cleartext→encrypted transition
finding: 'internal/store/fsstore/watcher.go:212-217 reconcileEntityPath, when an entity transitions from cleartext to encrypted on disk, calls s.loadEntity(existing.ID, existing.Type) to compute the cache delta. loadEntity re-reads the file — which is now encrypted — and returns an entity with empty Properties. removeEntityFromCache therefore no-ops, and the prior cleartext property values stay in propCache. They surface in distinct-value lookups, ad-hoc filter dropdowns, and any propcache consumer — the exact information disclosure encryption was meant to prevent. Fix: snapshot the prior entity from in-memory representation BEFORE the disk re-read, OR store enough cleartext property data on entityMeta to compute the cache delta without a re-read. Add a regression test that flips a file cleartext → encrypted via the watcher and asserts propCache is clean.'
severity: critical
status: open
---
