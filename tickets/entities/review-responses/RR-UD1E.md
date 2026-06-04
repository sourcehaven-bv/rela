---
id: RR-UD1E
type: review-response
title: Badge's 'property' prop is ambiguous and the ticket does not pin it
finding: |
  Badge.vue looks up schemaStore.styles[property][valueKey] and falls through to scanning all properties. Today EntityDetail cards/list pass field.propType; PropertyDisplay passes propType || field.name. These can disagree -- propType is the property's name in the metamodel, name is the local field id. After the refactor, SelectWidget display mode has only propertyDef (which carries the property's metamodel name), not the field's property name from the view config. If a view aliases a property, the badge style lookup degrades.
severity: significant
resolution: |
  Plan revised. Added propertyName?: string to WidgetProps -- the field's wire-level binding (separate from propertyDef, which is the schema entry). Display-mode callers pass field.propType into propertyName. SelectWidget hands it to Badge's property prop in display mode. Falls back to propertyDef.name if propertyName is absent. Pins the alias case behind an explicit prop. The same prop pre-positions TKT-IHCY7's inline-edit which needs the field's wire binding to PATCH by property name.
status: addressed
---
