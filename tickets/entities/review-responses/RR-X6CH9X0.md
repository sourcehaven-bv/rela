---
id: RR-X6CH9X0
type: review-response
title: 'Design: differential test must isolate gate and fold logic'
finding: Comparing against elementnaive over a separately seeded memstore mixes gate differences with cross-backend value representation.
severity: significant
resolution: 'Plan changed: the differential runs elementnaive over the same backend store, so only the SQL gate and fold differ. Stored hash equals canonical read-back stays pinned by RunElementTests; the gate''s MatchingFaces is pinned against graphquerynaive by RunGraphDifferential.'
status: addressed
---
