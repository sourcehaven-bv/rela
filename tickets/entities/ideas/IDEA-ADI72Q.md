---
id: IDEA-ADI72Q
type: idea
title: Live entities reference schema_hash (which entities predate migration X + faithful live rendering)
description: 'Follow-up to TKT-9INY0Y / RES-4ILUJZ (option C2). The pgstore content-versioning feature introduces a content-addressed schema_versions(hash, snapshot) table that historical entity_versions rows reference. This idea extends that face to LIVE entities: the entities table also carries schema_hash into the same shared schema_versions table. Unlocks: (1) ''which entities predate metamodel migration X / are stale'' queries, (2) faithful rendering everywhere (not just history), (3) a natural join key. Cost: touches the hot entities table and every live write path, so it was deferred out of the versioning v1. RES-4ILUJZ recommends designing schema_versions so this can be added without a migration rewrite.'
category: architecture
effort: medium
value: valuable
notes: Depends on TKT-9INY0Y landing schema_versions first. Not scheduled.
status: captured
---
