---
id: FEAT-i5ji
type: feature
title: Lua scripting support for programmable views
description: Add Lua scripting capability using gopher-lua to enable programmable views as an alternative to declarative YAML views. Scripts can invoke rela functions to query entities and relations, then produce structured output as JSON.
status: implemented
---

# Lua Scripting Support

Enable Lua scripts as a flexible alternative to declarative views for complex data extraction scenarios.

## Motivation

The current view system is declarative (YAML-based), which works well for straightforward traversals but becomes limiting for complex transformations. Lua scripting provides:

- **Flexibility**: Arbitrary logic for filtering, transforming, and aggregating data
- **Reusability**: Scripts can be shared and parameterized
- **Extensibility**: Foundation for future scripting use cases (automations, validations, etc.)

## Core Capabilities

1. **Entity Access**: Query entities by type, ID, or filter expressions
2. **Relation Traversal**: Navigate the graph programmatically
3. **Parameter Input**: Accept parameters when invoking scripts
4. **Structured Output**: Produce JSON-serializable data structures

## Open Questions

- **Output API**: Should scripts return a value (rela serializes) or call an output function (imperative)?
- **Script Location**: Where do scripts live? (`scripts/` directory? inline in views.yaml?)
- **Error Handling**: How to surface Lua errors to users?
- **Sandboxing**: Which Lua standard libraries to expose?
