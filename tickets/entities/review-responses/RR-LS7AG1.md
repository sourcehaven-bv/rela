---
id: RR-LS7AG1
type: review-response
title: 'direction: incoming on a faced type reports once per face for one logical defect'
finding: 'entity.Relation has FromFace and deliberately NO ToFace (entity.go:263-265), so an incoming edge cannot be attributed to a face of the entity it arrives at. analysis.go:504-517 already establishes the asymmetry: the tail filter applies only on the outgoing direction. Because loadCandidates validates every face as its own row (AllStates: true), an incoming constraint on a 3-face entity evaluates identically three times and emits three violations for one logical defect. The plan''s ''face behaviour unchanged'' criterion is inapplicable — incoming does not exist today, so there is no behaviour to preserve.'
severity: significant
resolution: 'Accepted as a real gap in the plan. DECISION: allow it and document that incoming counts are entity-level, not per-face; do NOT make it a load error, because the atlas motivating case is precisely an incoming constraint on a faced type (procedure with concept/vastgesteld), so rejecting it would block the use case the ticket exists for. The duplicate reporting is accepted for now; per-entity deduplication belongs to the deferred world ticket. Added acceptance criteria: a test asserting the count is face-independent, and documentation of the semantics in the godoc and docs/metamodel.md.'
status: addressed
---
