---
id: TKT-8T78Z
type: ticket
title: Configurable list actions with keyboard shortcuts for bulk property updates
kind: enhancement
priority: medium
effort: m
status: done
---

## Description

Add keyboard-driven list actions for efficient bulk property updates. Extends
the existing `actions` config with `label`, `key`, `confirm`, and `set` fields.
Lists reference actions by ID. Both `set` (declarative) and `script` (Lua)
actions work with multi-select by invoking once per selected entity.

## Design

### Unified action model

Extends the existing `actions` map in `data-entry.yaml`:

```yaml
actions:
  mark-done:
    label: "Done"
    key: "d"
    set:
      status: done
      completed_at: "{{today}}"
  archive:
    label: "Archive"
    key: "x"
    confirm: true
    set:
      status: archived
  run-check:
    label: "Run Check"
    key: "c"
    confirm: true
    script: scripts/check.lua
    params:
      verbose: "true"

lists:
  tickets:
    entity_type: ticket
    columns: [title, status, priority]
    actions: [mark-done, archive, run-check]
  my-tickets:
    entity_type: ticket
    columns: [title, status]
    actions: [mark-done]
```

### Interaction model

1. Navigate rows with `j`/`k` or arrows (existing)
2. `Space` toggles row selection (multi-select)
3. Press action key (e.g. `d`) → applies action to all selected rows
4. `confirm: true` shows yes/no before applying
5. `confirm: false` (default) applies immediately
6. Action keys do nothing when no rows are selected
7. `Escape` clears selection
8. Action bar at bottom shows selection count and available actions with key hints

### Execution

Both `set` and `script` actions iterate selected entities, invoking once per
entity:
- `set` → PATCH /api/v1/{plural}/{id} with properties
- `script` → POST /api/v1/_action/{id} with entity context
- Errors on individual entities don't stop the rest; toast per failure
- `{{today}}` interpolated to current date (YYYY-MM-DD) in set values

### Action fields

- `label`: Display name shown in action bar
- `key`: Single-key shortcut (a-z, 0-9)
- `confirm`: Whether to show confirmation before applying (default: false)
- `set`: Map of property → value (declarative mutation)
- `script`: Lua script path (existing field)
- `params`: Static params for script (existing field)
- `description`: Description (existing field)

An action has either `set` or `script` (not both).
