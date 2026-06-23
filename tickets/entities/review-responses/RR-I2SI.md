---
id: RR-I2SI
type: review-response
title: filterVisibleIncludes uses candidates[:0] alias — brittle if caller retains slice
finding: 'out := candidates[:0] aliases the input slice''s storage. Safe under current call site (caller drops candidates immediately) but a non-obvious aliasing pattern with no comment. Next refactor that retains candidates introduces silent corruption. Fix: out := make([]*entityPkg.Entity, 0, len(candidates)). One slice header per request, invisible vs the GraphQuery roundtrip.'
severity: minor
resolution: filterVisibleIncludes now uses make([]*entityPkg.Entity, 0, len(candidates)) instead of aliasing candidates[:0].
status: addressed
---
