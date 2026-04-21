---
id: RR-KNXFF
type: review-response
title: Partial-failure rollback on multi-op writes
finding: 'If reconcileOutgoingRelations errors mid-way after CreateEntity/UpdateEntity succeeded, the entity persists with partial edges and the handler returns 422. No rollback. POST case is worse: retry collides on ID.'
severity: critical
reason: 'Owner explicit sign-off: ''partial-failure => don''t care; fix other findings'' (in-session decision). Rationale accepted for this PR: (1) the common failure modes (unknown relation type, unknown target, source/target-type mismatch) are now caught up-front by metamodel pre-validation before any writes happen, which eliminates the most likely cause of a partial write in practice; (2) remaining failure modes (automation veto, store IO error) are rare and already have the same ''partial write, 422 returned'' shape on the per-edge POST/DELETE endpoints that have been live for months, so the new PATCH/POST path is consistent with existing behaviour rather than introducing a new contract. A proper fix requires a transactional batch primitive on entitymanager.EntityManager (architect C2); that is a cross-cutting refactor and will be tracked as a separate ticket. The retry-collides-on-ID edge case for POST remains a known sharp edge but is observable to the client (422 + existing entity) rather than silent data loss.'
status: wont-fix
---
