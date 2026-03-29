---
description: Views define how to traverse the entity graph and shape output for document publishing, context generation, and analysis. Defined in views.yaml.
id: views
layer: core
package: internal/views
status: stable
summary: Declarative graph traversals for context generation
title: Views
type: concept
---

# Views

Views are declarative definitions that specify:

1. **Entry point**: Starting entity type and parameter
2. **Traversal rules**: How to follow relations through the graph
3. **Filters**: Which entities to include/exclude
4. **Output structure**: How to shape the result

## Current Implementation

The current view system uses flat collections with implicit type filtering based on collection names. This requires post-processing scripts to:
- Filter entities to scope (e.g., only components belonging to document's bouwblokken)
- Build hierarchical structures for document rendering
- Deduplicate entities that appear via multiple paths

## Key Files

- `internal/views/types.go` - Type definitions
- `internal/views/engine.go` - Execution engine
- `internal/views/formatter.go` - Output formatting
- `internal/views/loader.go` - YAML loading
