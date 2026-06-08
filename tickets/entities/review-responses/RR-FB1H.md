---
id: RR-FB1H
type: review-response
title: 'S4: fields prop should be a discriminated union, not two-optionals'
finding: |
  `propertyDef?: PropertyDef` AND `routingHint?: WidgetRoutingHint` both optional creates an ambiguity (which wins? what if neither?). Bang-cast in the impl covers a real contract gap.
severity: significant
status: addressed
resolution: |
  PLAN updated to use a discriminated union on the `fields` prop:

  ```ts
  type SectionEditField = {
    property: string
    label: string
    writable: boolean
    verdict?: FieldAffordance  // forwards options + writable; see RR-FB1I
  } & ({ kind: 'schema'; propertyDef: PropertyDef } | { kind: 'hint'; routingHint: WidgetRoutingHint })
  ```

  Implementation: `field.kind === 'schema' ? defaultRegistry.resolve(undefined, field.propertyDef) : defaultRegistry.resolveFromHint(field.routingHint)`. No bang-casts. Mirrors PropertyDisplay's clean if/else.
---
