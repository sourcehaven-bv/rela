---
id: TKT-sxcv
kind: enhancement
priority: high
status: done
title: Define YAML schema types for Query-as-Output-Structure views
type: ticket
---

# Define YAML schema types for Query-as-Output-Structure views

First phase of FEAT-qos: define the new Go types for the query-as-structure view format.

## Scope

Create new type definitions in `internal/views/` that represent the query-as-output-structure format. This is schema definition only - no engine implementation yet.

## Requirements

1. Define `QueryNode` type representing a node in the query tree
2. Support all traversal options: `via`, `via_incoming`, `type`, `recursive`
3. Support filtering: `where`, `require` (JSONPath-based scope filtering)
4. Support output control: `only`, `content`
5. Define `ViewDefV2` (or similar) to hold the new format
6. Add YAML unmarshaling with proper handling of the recursive tree structure
7. Coexist with existing types (don't break current views yet)

## Out of Scope

- Engine implementation (separate ticket)
- JSONPath evaluation (separate ticket)
- Migration of existing views
- CLI/MCP changes
