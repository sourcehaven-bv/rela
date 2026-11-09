---
id: RR-1VE2UX3
type: review-response
title: Relation meta from the source is not schema-validated
finding: writeRelation wrote remote relation meta through CreateRelation/UpdateRelation, which check endpoints only; required, min/max and length rules on relation properties were not enforced.
severity: significant
resolution: peersync.checkRelationMeta validates the meta the write leaves with metamodel ValidateRelationProperties before writing; a failure rejects the row. TestRun_RelationMetaIsValidated.
status: addressed
---
