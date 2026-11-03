---
id: RR-J7R91Z
type: review-response
title: Relation audit record is written before the entity create record
finding: 'internal/entitymanager/linked.go:116-121 recordRelationAudit runs before finishCreate writes OpCreateEntity, so the audit log shows the edge before the entity. Fix: call finishCreate first, then record the relation.'
severity: minor
resolution: CreateLinked records the entity audit before the relation audit; TestCreateLinked_CommitsTogether asserts the order (2ce29a3fe)
status: addressed
---
