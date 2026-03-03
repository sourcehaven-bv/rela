---
description: Add support for relation-based filter controls in data-entry list views, allowing users to filter entities by outgoing relation targets (e.g., filter controls by owner role).
id: FEAT-024
priority: medium
status: implemented
title: Relation-based filter controls in list views
type: feature
---

## Changes

### `internal/dataentryconfig/config.go`

Extended `FilterControl` with two new fields:

- `Relation string` — when set, the filter dropdown is populated from outgoing relation targets rather than a property value
- `Label string` — optional display label override for both property and relation filter controls

### `internal/dataentry/helpers.go`

Added two new methods on `App`:

- `filterByRelation(entities, relationType, value)` — keeps only entities that have an outgoing relation of the given type whose target title matches `value`
- `resolveRelationFilterValues(entities, relationType)` — collects and sorts the distinct target titles across all entities for use as dropdown options

Updated `resolveScope` to apply relation-based filters when reconstructing the ordered entity list for scope navigation (prev/next), keeping it in sync with the filter logic in `handleList`.

### `internal/dataentry/handlers.go`

Updated `handleList` in three places:

1. **Filter application loop** — added `fc.Relation != ""` branch calling `filterByRelation`
2. **filterParams builder** — uses `fc.Relation` as the URL param key when set
3. **Filter control renderer** — relation controls get their dropdown values from `resolveRelationFilterValues` on all (unfiltered) entities; respects the new `Label` field for both relation and property controls

### `business/data-entry.yaml` (example usage)

```yaml
filter_controls:
  - property: implementation_status
    widget: select
  - relation: controlOwnedByRole
    label: Owner
```
