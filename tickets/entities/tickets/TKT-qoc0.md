---
id: TKT-qoc0
kind: enhancement
priority: high
status: review
title: Implement v2 view engine with two-pass execution
type: ticket
---

# Implement v2 view engine with two-pass execution

Phase 2 of FEAT-qos: implement the execution engine for v2 view definitions.

## Scope

Create the engine that executes v2 view definitions using a two-pass approach:
1. **Pass 1 (Collect)**: Traverse graph following relations, collecting entities into the hierarchical structure
2. **Pass 2 (Filter)**: Apply `require` clauses using JSONPath to filter collected entities

## Requirements

1. Implement `EngineV2` type in `internal/views/`
2. Execute v2 view definitions (QueryNode tree)
3. Traverse relations following `via` and `via_incoming`
4. Filter by entity types (`types: [...]`)
5. Support recursive traversal with depth limit
6. Apply property filters (`where: "..."`)
7. Integrate JSONPath library (ohler55/ojg) for `require` clause evaluation
8. Build hierarchical output that mirrors input structure
9. Generate flat entity map for lookups
10. Handle cycles in graph traversal

## Out of Scope

- CLI/MCP integration (separate ticket)
- Migration tooling for v1 -> v2
- Performance optimization

## Acceptance Criteria

- [x] Engine executes basic v2 view (entry + single relation)
- [x] Type filtering works (`types: [function]`)
- [x] Recursive traversal respects depth limit
- [x] Property filter (`where`) works
- [x] JSONPath `require` filters entities correctly
- [x] Two-pass execution handles cross-references
- [x] Output structure mirrors input query structure
- [x] Entity deduplication works correctly

## Implementation Notes

### Pass 1 (Collect) - Completed

The collect pass is fully implemented in `internal/views/engine_v2.go`:

- `EngineV2` struct with `graph` and `meta` fields
- `Execute()` method validates entry and kicks off collection
- `collectEntity()` builds hierarchical output for each entity
- `collectChildren()` follows relations (via/via_incoming), applies filters
- `collectRecursive()` handles recursive traversal with depth tracking
- `followOutgoing()`/`followIncoming()` traverse edges
- `filterByTypes()` filters by entity type
- `filterByWhere()` filters by property expression
- `buildRelationRefs()` builds relation references for output

### Pass 2 (Filter) - Completed

JSONPath filtering (`require` clauses) implemented in `internal/views/filter_v2.go`:

- `buildJSONData()` / `entityToMap()` convert result to JSON-like structure
- `evaluateJSONPath()` evaluates JSONPath expressions using ohler55/ojg
- `extractIDs()` extracts IDs from JSONPath results
- `filterByRequire()` filters entities based on relation constraints
- `applyRequireFiltersRecursive()` applies filters throughout the tree
- `hasRequireClauses()` detects if filtering is needed

### Output Structure

- `ResultV2`: Contains `Entry` (root entity) and `EntityMap` (flat map of all entities)
- `EntityResultV2`: Hierarchical entity with `Children` map for nested relations
- Structure mirrors the query structure exactly

### CLI Integration - Completed

- `LoadViewsAuto()` detects v1 vs v2 format automatically
- `IsV2Format()` checks for `entry_type` field (v2) vs `entry.type` (v1)
- `view` command calls appropriate engine based on format
- `FormatV2()` formats results as JSON or YAML

### Files Created/Modified

- `internal/views/engine_v2.go` - Core v2 engine
- `internal/views/engine_v2_test.go` - Comprehensive tests
- `internal/views/filter_v2.go` - JSONPath filtering
- `internal/views/formatter.go` - Added FormatV2()
- `internal/repository/repository.go` - Added LoadViewsV2, LoadViewsAuto
- `internal/workspace/workspace.go` - Added v2 workspace methods
- `internal/cli/view.go` - CLI auto-detection for v1/v2
