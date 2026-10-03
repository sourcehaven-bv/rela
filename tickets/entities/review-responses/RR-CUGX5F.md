---
id: RR-CUGX5F
type: review-response
title: '[security] Relation history ?record_id= reaches a sibling tail''s lineage'
finding: 'Security review: authorizeRelationHistoryRead authorizes the named tail, then resolveLineageIDs validated the client record_id with the face-blind recordIDIsHeadOfKey, so a caller who can read only FROM@published could read the @draft tail''s relation history.'
severity: significant
resolution: recordIDIsHeadOfKey now filters from_face and resolveLineageIDs passes q.FromFace on both backends. Pinned by storetest RelationRecordIDIsBoundToItsTail.
status: addressed
---
