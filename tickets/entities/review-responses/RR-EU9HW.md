---
id: RR-EU9HW
type: review-response
title: No-op suppression runs BEFORE meta validation — invalid meta returns 200
finding: |-
    api_v1.go:670-682. Suppression check (line 670-674) runs before ValidateRelationProperties (line 679). Combined with C4 (propsValueEqual collapses across types), a request like `weight: "5"` (string) when existing has `weight: 5` (int) is wrongly classified as no-op and returns 200 with no validation. The schema-violating string is accepted but silently dropped — next reload shows the user's edit didn't persist.

    Fix: validate before suppressing. Build the candidate *model.Relation, validate, THEN check no-op:

    ```go
    rel := model.NewRelation(...)
    rel.Properties = finalProps; rel.Content = finalContent
    if errs := meta.ValidateRelationProperties(rel); len(errs) > 0 { ... }
    if existing, ok := currentByTarget[ref.ID]; ok {
      if relationsEqual(existing.Properties, finalProps) && existing.Content == finalContent {
        continue
      }
    }
    ```
severity: critical
resolution: computeDiff now calls meta.ValidateRelationProperties(rel) BEFORE the no-op suppression check. Test TestV1Patch_InvalidMetaTypeIsNotNoOp confirms invalid meta values return 422 even when the request would otherwise look like a no-op.
status: addressed
---
