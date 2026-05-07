---
id: RR-S4FXV
type: review-response
title: Relation Content (markdown body) cannot be expressed in the wire format
finding: |-
    metamodel/types.go:240: RelationDef.Content bool — relation types can have markdown body content. Existing per-relation endpoints accept content parameter. New wire format only has meta. So relation types with content: true CANNOT update body content via the new PATCH. Plan never mentions this. Will hit form auto-save the first time someone has a notes relation with markdown content per edge.

    Fix (recommend Option A): add optional Content *string field to V1ResourceIdentifier:
    ```
    type V1ResourceIdentifier struct {
        Type    string
        ID      string
        Meta    map[string]interface{}
        Content *string
    }
    ```
    Content == nil = leave alone. *Content == "" = empty body. Document the asymmetry with meta (which is replacement-only).

    Alternative (Option B): scope it out and 422 if Content:true relation type is touched via the new path. Recommend A — 'unified update path except for content' defeats the purpose.
severity: significant
resolution: 'Following user observation that meta could also use upsert: ALL edge-level data (meta, content) now uses upsert semantics matching entity properties/properties_unset/content. content: *string added to V1ResourceIdentifier. AC #8 covers upsert/clear/leave-alone variants. Decision #4 documents. Wire format more consistent with rela''s existing API conventions.'
status: addressed
---
