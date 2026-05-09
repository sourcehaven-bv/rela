---
id: RR-5VND
type: review-response
title: buildPluralToTypeMap rebuilt per watcher event — cache it once at construction
finding: watcher.go:279 entityIdentityFromPath calls buildPluralToTypeMap on every encrypted-entity event. Schemas are immutable after fsstore.New. Same code path is also in index.go:172 at startup. Cache the map on FSStore at construction. Negligible production cost today but it's the kind of regression you'd flag elsewhere.
severity: minor
status: open
---
