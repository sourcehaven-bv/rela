---
id: TKT-U2FQ
type: ticket
title: Add Lua validation rules to metamodel
kind: enhancement
priority: medium
effort: m
status: done
---

# Add Lua validation rules to metamodel

## Problem

Current validation rules use declarative `when`/`then` filter expressions which
are limited:
- Cannot do date arithmetic (e.g., "created_at must be within 30 days")
- Cannot access related entities (e.g., "parent must have status=approved")
- Cannot use regex captures or complex string logic
- Cannot implement business rules requiring computation

## Proposed Solution

Add `lua:` field to ValidationRule that executes Lua code returning true/false:

```yaml
validations:
  - name: deadline-within-sprint
    description: Deadline must be within current sprint
    entity_type: ticket
    lua: |
      local deadline = entity:prop("deadline")
      local sprint_end = entity:prop("sprint_end")
      return deadline and sprint_end and deadline <= sprint_end
    severity: error
```

## Scope

- Add `lua` field to ValidationRule type
- Execute Lua in sandboxed runtime with entity context
- Integrate with existing validation service
- Support both inline `lua:` and `lua_file:` for script files
