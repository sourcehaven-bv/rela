---
id: RR-FJH7
type: review-response
title: rebuildSearchIndex publishes index built from stale graph snapshot
finding: 'rebuildSearchIndex reads w.graph.AllNodes() then publishes via searchIdx.Swap. If a CreateEntity/UpdateEntity lands between the snapshot and the Swap, the new entity is in the graph but absent from the freshly-published index. Same root cause as the critical graph-torn issue: Reload is not serialized against writers.'
severity: critical
resolution: Fixed in TKT-Z7HL alongside RR-PA4Y. The new Reload path in workspace.go builds the search index against the freshly-synced graph and publishes graph + searchIdx atomically as part of the same workspaceState. The new buildReloadSearchIndex helper also keeps the old index in place on any failure (it never publishes a partial or broken index over a working one). The migration-error branch (reloadKeepingOldMetamodel) now also rebuilds the search index against the new graph so the published state is internally consistent — addresses the cranky review's S4/finding-#4 about the migration branch.
reason: 'Deferred: pre-existing issue, production code uses App.writeMu to enforce the required serialization discipline; making the workspace intrinsically safe without an external mutex is out of scope for TKT-252Y.'
status: addressed
---
