---
id: RR-VFX8P
type: review-response
title: Propagated counterparty edges are never validated
finding: |-
    api_v1.go:611-683 validates ValidateRelation, target existence, target type, and ValidateRelationProperties for primary edges only. computeRelationPropagation appends propagated.adds directly into diff.adds without re-validation. Failure modes: (1) Inverse: {ID: 'ghost-relation'} where ghost doesn't exist in metamodel — back-edge written with no def. (2) Inverse type's From/To allowlist isn't checked against the swapped endpoints. (3) Inverse's Properties schema isn't validated against the inherited primary meta. PLAN-CAK3L line 451 explicitly said 'staged counterparty changes are validated (target type, meta) just like primary changes' — not implemented.

    Fix: extract validation into a helper, run it on every staged edge in diff.adds AFTER propagation:

    ```go
    for _, rel := range diff.adds {
      fromEnt, _ := tx.GetEntity(rel.From)
      toEnt, _ := tx.GetEntity(rel.To)
      if fromEnt == nil || toEnt == nil { return validationErrorf(...) }
      if err := meta.ValidateRelation(rel.Type, fromEnt.Type, toEnt.Type); err != nil { ... }
      if errs := meta.ValidateRelationProperties(rel); len(errs) > 0 { ... }
    }
    ```
severity: critical
resolution: 'validateStagedEdges helper validates every entry in diff.adds (primary AND propagated). Checks: target existence (tx.GetEntity), target type allowlist (meta.ValidateRelation against the swapped endpoints), meta property type (meta.ValidateRelationProperties). Test TestV1Patch_InverseToUnknownRelationTypeRejected confirms that an Inverse pointing to a non-existent relation type now returns 422.'
status: addressed
---
