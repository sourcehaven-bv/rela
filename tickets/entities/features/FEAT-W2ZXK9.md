---
id: FEAT-W2ZXK9
type: feature
title: 'Roll-up properties: read-time aggregates over related entities'
summary: Schema-declared read-only properties whose value aggregates related entities (count, exists, min, max, latest_by). Computed per viewer at read time and never stored.
description: 'Computed properties are entity-local. GRC and similar domains need values derived from related entities on lists and entity pages: the number of valid evidence items of a control, the result of the latest control test, the number of open treatment plans of a risk. A rollup: property aggregates the related rows the viewer may read, at read time, so the value is per-viewer and never persisted. Realises IDEA-001 (Rollup Properties).'
priority: high
status: proposed
---
