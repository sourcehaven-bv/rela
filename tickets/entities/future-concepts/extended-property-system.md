---
challenges: Circular dependency detection, performance with many rollups, formula syntax design, caching computed values
description: 'Extend the metamodel to support: (1) rollup properties that aggregate data from relations (count, sum, avg, min, max), (2) computed properties using formulas (property arithmetic, date math, conditionals), (3) property inheritance from related entities. Would require changes to metamodel parsing, entity loading, and property resolution.'
effort: large
id: extended-property-system
prerequisites: None - can build on current property system
rationale: Eliminates data duplication, enables dynamic dashboards, brings Notion-like power to the schema system.
status: validated
summary: Rollups, computed properties, and formula support
title: Extended Property System
type: future-concept
value: critical
---
