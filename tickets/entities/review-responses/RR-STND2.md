---
id: RR-STND2
type: review-response
title: Concurrent-PATCH locking boundary not specified — diff staleness window plausible
finding: |-
    Plan: 'serialized by App.writeMu; one client may see the other's state when computing the diff.' But writeMu and reloadMu are SEPARATE locks. Inside WithTx, reloadMu is held — graph won't move under the diff. But the diff is computed in the handler BEFORE entering WithTx (per plan: steps 1-7 are pre-Tx, step 8 is Tx). Between step 5 (compute diff) and step 8 (commit), the watcher reload can fire. writeMu doesn't block the watcher.

    Fix: make lock boundary explicit. Move 'Compute the relation diff' (step 5) INSIDE the WithTx block. Read tx.OutgoingEdges(entityID), compute diff against req.Relations, validate, stage — all under reloadMu. The handler never reads graph.OutgoingEdges outside WithTx. Diff is now atomic with validation and writes.

    Restructure Approach steps so 5-8 are all inside WithTx.
severity: significant
resolution: 'Approach restructured: handler enters WithTx at step 3 (after lookup + ETag check). Diff computation, validation, and staging all run inside WithTx under reloadMu. The handler never reads graph.OutgoingEdges outside WithTx. Eliminates diff-staleness window.'
status: addressed
---
