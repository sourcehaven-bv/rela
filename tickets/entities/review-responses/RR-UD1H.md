---
id: RR-UD1H
type: review-response
title: Per-cell schemaStore lookup cost is asserted, not measured
finding: |
  Ticket says "schemaStore is reactive and cached." It is -- entityTypes is a Map on a ref(). But the lookup site is inside a template render function, so accessing schemaStore.getEntityType(field.propType) registers a reactive dependency on entityTypes.value for every cell. Cards view 200 entries x 5 fields = 1000 dependencies. SSE schema reload triggers a full re-render. Waving this away as "measure if visible at 1000 cards" is the moment regressions land in production.
severity: significant
resolution: |
  Plan revised. Each display-mode-rendering section computes a Map<fieldKey, PropertyDef> once via a Vue computed -- one schema lookup per (section x field), not per (section x entity x field). Cells receive the resolved PropertyDef directly. For mixed-type cards sections, the Map keys on (entityType, property). One reactive subscription per section instead of per cell; bounded by section config, not by entity count. A spy test asserts schemaStore.getPropertyDef is called once per (section x field) as a verification gate.
status: addressed
---
