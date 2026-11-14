---
id: RR-KVRZ2SL
type: review-response
title: Relation-write refusals not tested under a real policy
finding: Only the update path was tested against a Declarative policy; relation create, delete and replace refusals were tested with the NopACL.
severity: minor
resolution: TestRelationRules_RelationWriteRefusalNamesOnlyTheRule runs create, delete and replace under a Declarative policy and checks the refusal carries only the rule message and no entity id.
status: addressed
---
