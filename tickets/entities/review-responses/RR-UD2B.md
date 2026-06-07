---
id: RR-UD2B
type: review-response
title: synthDef is a typed lie
finding: |
  synthDef returns { type: 'enum', values: field.values ?? [], list: true } whenever field.propType is non-empty -- but values here is the current entity's values, not the enum's allowed values. That's not what PropertyDef.values means anywhere else in the codebase. It happens to work because the only consumer downstream is defaultWidgetFor (which only checks values.length > 0) and MultiSelectWidget.options (which only matters in edit mode, never entered here). The next person who reads this and writes "widget X also uses propertyDef.values to render labels" will introduce a real bug.
severity: significant
status: addressed
resolution: |
  Introduced a typed WidgetRoutingHint discriminated union ({ kind: 'text' | 'enum-list' | 'enum' | ..., propertyName }) and a new defaultRegistry.resolveFromHint(hint) method. synthDef is gone -- replaced by viewFieldRoutingHint() in frontend/src/widgets/viewRouting.ts which returns honest hint values. Forms still resolve via the real-schema-driven resolve(propertyDef); views resolve via resolveFromHint. No more fake PropertyDefs. The contract is documented in types.ts: "propertyDef is typically present in edit mode and may be absent in display mode."
---
