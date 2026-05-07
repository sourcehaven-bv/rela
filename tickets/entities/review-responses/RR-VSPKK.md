---
id: RR-VSPKK
type: review-response
title: Symmetric/inverse back-edges share the primary's Properties map by reference
finding: |-
    api_v1.go:860-872: `back.Properties = rel.Properties` aliases the same map. Any future code path that does `rel.Properties[k] = v` silently mutates the back-edge too. Also contradicts the plan's stated future intent of giving inverses their own meta — you've made it impossible without breaking the primary.

    Fix: shallow-copy the map (YAML scalar values are immutable in practice):

    ```go
    backProps := make(map[string]interface{}, len(rel.Properties))
    for k, v := range rel.Properties { backProps[k] = v }
    back.Properties = backProps
    ```
severity: critical
resolution: Added cloneProps helper. propagateRelations now calls cloneProps(rel.Properties) when building the back-edge so the primary and back-edge have independent maps. Test TestV1Patch_SymmetricMetaIsDeepCopied verifies the maps have different pointer addresses.
status: addressed
---
