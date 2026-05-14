---
id: RR-31DE
type: review-response
title: 'Cranky #12: storeUpsertEntity duplication between workspace and entitymanager'
finding: workspace.storeUpsertEntity / storeUpsertRelation duplicate entitymanager's upsertEntity / upsertRelation. Two implementations of the same pattern can drift.
severity: minor
reason: workspace's versions only serve SeedEntityForTest / SeedRelationForTest test fixtures. Replacing them with entitymanager helpers would either need exporting them (API surface increase) or routing test seeds through Manager (which doesn't expose raw upsert). TKT-64R3 deletes workspace and the duplication along with it.
status: deferred
---
