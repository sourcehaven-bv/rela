---
id: TKT-NOSM
type: ticket
title: Add Lua scripting support for custom validation rules
kind: enhancement
priority: medium
effort: m
status: done
---

Add `lua:` and `lua_file:` fields to validation rules for complex business
logic.

## Changes

- Add read-only workspace wrapper to prevent mutations in validation scripts
- Add 5-second execution timeout to prevent infinite loops
- Add path traversal protection for external script files
- Update architecture boundaries to allow validation→lua dependency
- Add comprehensive documentation for Lua validation
