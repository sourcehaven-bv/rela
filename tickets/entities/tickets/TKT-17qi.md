---
effort: l
id: TKT-17qi
kind: enhancement
priority: medium
status: done
title: Add Lua scripting support via gopher-lua
type: ticket
---

# Add Lua Scripting Support

Integrate the [gopher-lua](https://github.com/yuin/gopher-lua) library to provide Lua scripting capability as an alternative to declarative views.

## Background

The current view system is YAML-based and declarative. For complex data extraction scenarios, a programmable approach offers more flexibility. Lua scripts can:

- Query entities and relations via rela bindings
- Apply arbitrary filtering and transformation logic
- Produce structured output (JSON, multiple files, etc.)

## Scope

1. **Lua Runtime Integration**: Add gopher-lua dependency and create `internal/lua` package
2. **Rela Bindings**: Expose core rela functions to Lua scripts:
   - `rela.get_entity(id)` - Get entity by ID
   - `rela.list_entities(type, filter?)` - List entities with optional filter
   - `rela.get_relations(from?, type?, to?)` - Query relations
   - `rela.trace_from(id, depth?)` - Trace dependencies
3. **Output API** (imperative):
   - `rela.output(data)` - Write JSON to stdout
   - `rela.write_file(path, content)` - Write to file
4. **CLI Command**: `rela script <file.lua> [params...]` to execute scripts
5. **Script Location**: Project `scripts/` directory

## Design Decisions

- **Imperative Output API**: More flexible - supports multiple outputs, file generation, streaming
- **Full Lua stdlib**: Security not a concern (local tool, trusted data, rela already executes commands)
- **Scripts in `scripts/` folder**: Convention over configuration
