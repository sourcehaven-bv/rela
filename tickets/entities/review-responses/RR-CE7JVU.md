---
id: RR-CE7JVU
type: review-response
title: 'Flat []Target return loses edge multiplicity: a triple can carry N edges'
finding: 'The plan''s central structural claim was that returning resolved far entities ([]Target) is strictly better than returning edges, because it makes the wrong-endpoint mistake unavailable. The data model contradicts it: relation identity is (From, FromFace, Type, To), not the triple (internal/entity/entity.go:302-308 — ''two edges on the same triple with different tails are two relations''). A RelationQuery with FromFace == nil leaves the tail unfiltered, so one subject can have N edges to the same target, and today''s loop counts EDGES (validation.go:577-582), so N face-tailed edges count N. A flat []Target forces an unstated choice: dedupe (silently 3 -> 1, breaking the ''gates byte-identical'' criterion) or don''t (repeated ids, a contract no reader expects and one a future batched implementation would naturally collapse).'
severity: critical
resolution: 'Interface changed to one element per EDGE carrying a resolution tri-state, which satisfies this and the C3 shape requirement together. The ''ONE ELEMENT PER EDGE; ids may repeat'' invariant is now stated in the plan and must go in the interface godoc. Added an acceptance criterion pinning it: two face-tailed edges to one target must count as 2 — specifically to stop the deferred batching work collapsing it silently.'
status: addressed
---

Verified independently: `entity.Relation.Key` at
`internal/entity/entity.go:302-308` confirms the face is part of relation
identity.
