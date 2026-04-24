---
id: RR-EXNL6
type: review-response
title: Tests coupled to generation behavior via hardcoded IDs
finding: 'Dropping explicit ID: ''REQ-001'' seeding in favor of auto-generation left downstream assertions coupled to the current sequential allocator (that it produces REQ-001, DEC-001). If allocator prefix casing / padding / start value ever changes, every affected test breaks confusingly.'
severity: minor
resolution: mustCreate now returns the created entity. TestDeleteEntity_NoCascade_NoRelations, TestDeleteEntity_CascadeRelations, TestCreateRelation, TestCreateRelation_Duplicate, TestDeleteRelation capture the returned entity and reference req.ID / dec.ID downstream. TestCreateEntity and TestGenerateID_Sequential keep hardcoded IDs because those tests are about sequential generation itself.
status: addressed
---
