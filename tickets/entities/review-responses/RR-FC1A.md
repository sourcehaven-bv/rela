---
id: RR-FC1A
type: review-response
title: 'C1: buildSectionEditFields can be parameterized; buildRowEditFields is dead-weight'
finding: |
  The existing `buildSectionEditFields(fields, ent, getPropertyDef)` reads only `ent.type` and `ent._fields?.[f.property]`. Both fields exist on the post-IHC7D ViewEntity. Forking into `buildRowEditFields` is architectural duplication that the IHC7B reviewer would have flagged on its own merits — same for `sectionShouldRouteToInlineEdit`. Parameterize over a `{ type: string; _fields?: Record<string, FieldAffordance> }` shape (or a new `FieldVerdictSource` type) and the same helper serves both call sites.
severity: critical
status: addressed
resolution: |
  PLAN AC 10 + Technical Approach amended: the existing `buildSectionEditFields` and `sectionShouldRouteToInlineEdit` helpers get their second parameter loosened to `FieldVerdictSource = { type: string; _fields?: Record<string, FieldAffordance> }`. Both Entity (entry section) and ViewEntity (row) satisfy the shape. No new helpers required. Tests extend the existing suite with a row-shaped fixture. Removes ~60 lines of proposed duplication.

  The `applyPropertyToRow` helper IS new (Entity has `properties`, ViewEntity has `_props` — different storage shapes), kept as planned.
---
