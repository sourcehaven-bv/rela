---
id: AM-recreated-id-lineage-fenced
type: automated-measure
title: 'storetest: a recreated id''s timeline excludes its deleted predecessor'
description: A versioning storetest case deletes an entity, recreates the same id, and asserts ListVersions of the new entity does not return the deleted one's snapshots.
kind: test
location: internal/store/storetest
status: proposed
---
