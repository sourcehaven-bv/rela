---
id: RR-TRMD4O
type: review-response
title: Center label resolution on Badge, not the form widgets
finding: 'The plan threads labels through SelectWidget/MultiSelectWidget display mode, but Badge is the shared render surface used directly in EntityList (rows+cards), EntityDetail (enum cells), KanbanView (card fields), and SidePanel. Wiring only the form widgets leaves every list/detail/kanban/side-panel enum showing the raw snake_case value — the majority of the UI the reach decision claims to cover. Correct design: make Badge resolve the label itself via useSchemaStore (which it already imports for color lookup), keyed on (property, value). Form widgets then need no display-mode label plumbing.'
severity: critical
resolution: Label resolution centered on Badge via a new schema-store getter getEnumLabel(value, property, entityType?). Badge resolves and displays the label while keeping :value (and color lookup) on the raw value. This covers list rows, entity detail, kanban cards, and side panel in one place. Verified by Badge.test.ts.
status: addressed
---
