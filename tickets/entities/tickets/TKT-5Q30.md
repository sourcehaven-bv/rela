---
id: TKT-5Q30
type: ticket
title: Add Lua action type to automation engine
kind: enhancement
priority: medium
effort: m
status: review
---

# Add Lua Action Type to Automation Engine

Extend the automation engine to support Lua actions that execute scripts when triggers fire. This enables complex automation scenarios:

- **Status workflows**: Auto-transition related entities when parent changes state
- **Cascade operations**: Handle dependent entities on delete/archive  
- **Auto-assignment**: Assign based on type, tags, or workload balancing

## Two Action Types

### `lua:` - Inline Code

For simple logic directly in metamodel.yaml:

```yaml
automations:
  - name: set-completed-date
    on:
      entity: [ticket]
      property: status
      becomes: done
    do:
      - lua: |
          rela.update_entity(entity.id, {properties = {completed_at = "{{today}}"}})
```

### `lua_file:` - Script Reference

For complex logic in reusable script files:

```yaml
automations:
  - name: cascade-status-to-children
    on:
      entity: [feature]
      property: status
      becomes: done
    do:
      - lua_file: automations/cascade-status.lua
```

## Context Available in Lua

- `entity` - The triggering entity (id, type, properties, content)
- `old_entity` - Previous state for update events
- All standard `rela.*` bindings (get_entity, update_entity, create_relation, etc.)

## Follow-up

- `rela automation test` command for dry-run testing of automations
