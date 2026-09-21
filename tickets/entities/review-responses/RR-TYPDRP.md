---
id: RR-TYPDRP
type: review-response
title: Peers with no type are dropped silently and the picker count disagrees with what carries
finding: duplicatePrefill.ts filters `edges.filter((e) => !!e.type)`. The server can emit an empty type when entityReader.entityType misses, which is reachable on a reader/gate divergence. Everything else in this module reports what it drops — that is its stated discipline — and this drop is both silent and more consequential (an edge, not a property). Worse, relationChoices does NOT filter on type, so the picker shows a count of 3, the user checks the box, and 2 edges are carried.
severity: minor
resolution: An untyped peer is reported as an omitted entry rather than dropped silently, matching the module's stated discipline. Pinned by a test.
status: addressed
---

## Suggested resolution

Report the drop as an omitted reason, and make the count agree with what will actually be carried.
