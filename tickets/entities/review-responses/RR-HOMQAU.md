---
id: RR-HOMQAU
type: review-response
title: customTypeNameForProperty duplicated labelsForProperty's scan scaffold
finding: The entityType-first-then-entity-then-relation scan loop was copy-pasted; two copies would drift.
severity: nit
resolution: Extracted shared scanPropertyDefs<T>(property, entityType, fromDef) helper; both labelsForProperty and customTypeNameForProperty now use it, making the first-wins tie-break identical by construction.
status: addressed
---
