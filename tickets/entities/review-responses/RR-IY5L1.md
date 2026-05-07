---
id: RR-IY5L1
type: review-response
title: Lock-ordering window between GetNode and WithTx allows stale-state writes
finding: |-
    Handler does `entity := a.State().Graph.GetNode(...)` (line 488) under writeMu only, then enters WithTx (line 559) which takes reloadMu. Between those, a concurrent file-watcher reload can fire and publish a new graph state. The captured entity pointer becomes stale; entity.Clone() clones stale state; staged commit overwrites the file watcher's just-loaded state.

    ETag check at line 498 doesn't save you — computeEntityETag(entity) computes against the same stale pointer.

    Fix: re-fetch INSIDE WithTx callback, move ETag check inside too:

    ```go
    txErr := a.ws.WithTx(func(tx *workspace.Tx) error {
        live, ok := tx.GetEntity(entityID)
        if !ok { return validationErrorf("entity not found") }
        if live.Type != typeName { return ... }
        // ETag check here
        staged := live.Clone()
        ...
    })
    ```
severity: significant
resolution: Entity lookup AND ETag check moved INSIDE the WithTx callback. The 404 fast-path stays outside (no race for a non-existent entity), but everything that reads/clones the entity for staging happens under reloadMu. The handler never reads graph state outside WithTx for the success path.
status: addressed
---
