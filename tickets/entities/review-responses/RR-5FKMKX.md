---
id: RR-5FKMKX
type: review-response
title: checkCardinality took relName beside the spec - the world seam leaked
finding: cardinalitySpec's godoc claims subject population, counting scope, and violation identity 'each have exactly one home', yet the relation name (the count query key) arrived as a separate parameter next to the spec while the spec only carried relationLabel (display identity). Two sources of relation truth the caller must keep in sync; a future world field would land on the spec while relName kept arriving by the side door.
severity: significant
resolution: Added relName to cardinalitySpec (set once per relation in CheckCardinality); checkCardinality's signature reduced to (ctx, spec, scope); countRelations reads spec.relName. The seam is now structural, not aspirational.
status: addressed
---
