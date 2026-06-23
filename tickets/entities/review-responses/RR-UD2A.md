---
id: RR-UD2A
type: review-response
title: RR-UD1H not actually addressed in the hot path
finding: |
  EntityDetail.cards/list and PropertyDisplay call synthDef(field) / defForProp(prop) / widgetFor(prop) inline in the template on every render. synthDef allocates a fresh object literal each call; defaultRegistry.resolve walks a Map and a console-warn guard. That's per-cell allocation + per-cell map lookup on every reactive tick. The ticket claimed "computed once per section (RR-UD1H)" -- but only mapFieldsToProperties got a pre-resolved propertyDef, and only for the entry section's PropertyDisplay. Cards and list still pay the per-cell cost.
severity: significant
status: addressed
resolution: |
  EntityDetail now precomputes a FieldRow array via fieldRowsFor(entity) which bundles { field, widget, hint } once per (entity, field) instead of recomputing widget+def inline per render. Cards and list templates iterate the precomputed rows and bind row.widget / row.hint.propertyName directly. PropertyDisplay similarly precomputes a `rows` array via a computed and iterates it. Single widget resolution per cell per render still happens, but no synthDef allocation on every reactive tick. The viewRouting.test.ts spy assertions confirm resolveFromHint is called with stable hint shapes (RR-UD2L).
---
