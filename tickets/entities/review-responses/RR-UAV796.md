---
id: RR-UAV796
type: review-response
title: Label-copy precedence must mirror value precedence (custom-type vs inline)
finding: toV1PropertyDef resolves custom-type Values by type-name (if meta.Types[propDef.Type] exists) else inline propDef.Values. Label copy must replicate this exact branch so a property referencing a custom type gets the CUSTOM TYPE's labels, and an inline `labels` map on a custom-typed property is (intentionally) ignored, matching how inline `values` is ignored there. Also handleV1Schema custom-type loop is a second independent CustomType serialization site — consider a shared toV1CustomType(ct) helper so the two don't drift.
severity: significant
resolution: 'toV1PropertyDef copies Labels with the identical custom-type-vs-inline branch as Values. Extracted shared toV1CustomType(ct) used by both the _schema types loop and (implicitly) the property path. Verified by TestV1SchemaEnumLabels: custom-typed property inherits the custom type''s labels; inline labels on a custom-typed property are ignored.'
status: addressed
---
