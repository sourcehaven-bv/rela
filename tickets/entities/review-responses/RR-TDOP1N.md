---
id: RR-TDOP1N
type: review-response
title: Relation checks must run before the entity write
finding: writeRelationCreate checks capacity and owning after createCore, so a full bound leaves an entity on fs/mem; prepareRelationCreate cannot resolve a not-yet-existing endpoint and writeRelationCreate does no ACL or template work.
severity: significant
resolution: 'Plan: before the Tx authorize RelationCreateRequest from known types and run ValidateRelation and the relation template; in the Tx mint the id, run unique, capacity and owning checks on the view, then write entity and relation. fs/mem test with a full capacity bound asserts no entity remains.'
status: addressed
---
