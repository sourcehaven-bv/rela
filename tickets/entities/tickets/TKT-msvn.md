---
effort: m
id: TKT-msvn
kind: enhancement
priority: medium
status: review
title: Add metamodel cleanup/trim command
type: ticket
---

Metamodels can grow quickly with entity types and relation types that may no longer be needed. Add a command that helps identify and clean up unused or underused schema elements.

## Requirements

1. Show entity types with zero instances
2. Show relation types with zero instances  
3. Show entity/relation types with only a few instances (configurable threshold)
4. Allow cleanup of unused entity types and relation types from metamodel
5. When cleaning up, also update:
   - data-entry.yaml (remove forms/views referencing deleted types)
   - views.yaml (remove views referencing deleted types)
6. Expose via both CLI (`rela analyze schema` or similar) and MCP tools
