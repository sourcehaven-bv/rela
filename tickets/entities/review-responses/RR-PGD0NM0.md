---
id: RR-PGD0NM0
type: review-response
title: Allowlist blocks reordering and unchanged meta sent back
finding: The closed universe included the order properties, so a restricted role lost drag-reorder. The unified PATCH checked every meta key of an existing edge, and the UI sends the whole meta map back, so editing one granted key returned 403.
severity: significant
resolution: Order properties stay writable unless a grant names them. The unified PATCH and the manager update check only keys whose value differs from the stored edge (entitymanager.ChangedRelationMeta). Pinned in TestV1RelationMeta_FieldsGrantIsAnAllowlist.
status: addressed
---
