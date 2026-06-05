---
id: RR-UD2D
type: review-response
title: MultiSelectWidget display drops propertyName when synthDef routes scalars
finding: |
  synthDef returns list:true for any propType, so 'status' = 'open' goes through MultiSelectWidget. The widget loops one Badge with :property="propertyName" -- fine from EntityDetail (field.propType) and PropertyDisplay (prop.propType ?? prop.name). But propertyName is OPTIONAL in WidgetProps, so Badge gets :property="undefined" whenever a caller forgets it, which silently fires the cross-property fallback scan in Badge (iterates every property's styles looking for a matching value). For ambiguous values like 'open' styled differently under status and pr_state, the scan picks whichever key Object.values yields first -- non-deterministic across JS engines and schema reloads.
severity: significant
status: addressed
resolution: |
  Two-part fix. (1) Made propertyName REQUIRED in WidgetProps (string, no default). Passing '' is acceptable for fields with no semantic binding; the type system forces callers to acknowledge the absence. (2) Removed Badge's cross-property fallback scan after auditing every Badge usage in the codebase (audit found all 8 call sites pass :property= explicitly). Badge now returns 'badge--gray' when :property= is absent or doesn't resolve a style -- deterministic. Two new edit-mode test mounts assert propertyName: ''; existing tests updated to pass an explicit :property= where they were testing the cross-property scan.
---
