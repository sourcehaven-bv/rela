---
description: Lua scripting engine using gopher-lua for flexible data extraction and transformation
id: lua-scripting
layer: core
status: draft
summary: Embedded Lua runtime for programmable extensions
title: Lua Scripting
type: concept
---

# Lua Scripting

Embedded Lua 5.1 runtime (via gopher-lua) providing programmable extensions for rela.

## Use Cases

1. **Programmable Views**: Complex data extraction with arbitrary logic
2. **Future**: Custom validations, automations, computed properties

## Key Components

- `internal/lua/` - Lua runtime and rela bindings
- Script files in project `scripts/` directory
