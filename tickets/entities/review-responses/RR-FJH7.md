---
id: RR-FJH7
type: review-response
title: rebuildSearchIndex publishes index built from stale graph snapshot
finding: 'rebuildSearchIndex reads w.graph.AllNodes() then publishes via searchIdx.Swap. If a CreateEntity/UpdateEntity lands between the snapshot and the Swap, the new entity is in the graph but absent from the freshly-published index. Same root cause as the critical graph-torn issue: Reload is not serialized against writers.'
severity: critical
resolution: 'Same root cause as RR-PA4Y: the graph is mutated in place and not under the workspaceState snapshot. In TKT-252Y we documented this as a known limitation and demonstrated in the new TestConcurrentReloadStateSnapshot that the bug is avoided if writers and reloaders share an external mutex (mirroring App.writeMu in production). The proper fix is scoped as TKT-PNPI ''Introduce Workspace transactions (Tx)'' — under the new model, all writers go through ws.WithTx which owns writeMu and a repo.Transaction for the commit, and repo.Sync returns a fresh graph rather than mutating one in place. Follow-up tickets TKT-CXHM (stage automation cascades on the outer Tx) and TKT-1LFQ (clone-and-publish commit semantics) complete the transactional model in stages.'
reason: 'Deferred: pre-existing issue, production code uses App.writeMu to enforce the required serialization discipline; making the workspace intrinsically safe without an external mutex is out of scope for TKT-252Y.'
status: deferred
---
