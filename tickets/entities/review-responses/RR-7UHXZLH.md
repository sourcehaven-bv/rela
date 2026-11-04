---
id: RR-7UHXZLH
type: review-response
title: Both-ends-wrong edge gets different codes per surface
finding: checkRelationEdges checked the path entity first, so under an inverse name REST answered target while the manager answered source.
severity: minor
resolution: 'All per-edge checks now run after the peer read in the manager''s order: missing peer, source end, target end.'
status: addressed
---
