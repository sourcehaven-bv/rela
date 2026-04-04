---
id: FEAT-qos
type: feature
title: Query-as-Output-Structure View System
summary: Redesign view system where query structure mirrors output structure, with JSONPath-based scope filtering
priority: high
status: removed
---

# Query-as-Output-Structure View System

Redesign the view system so the YAML query structure directly mirrors the output structure. Uses JSONPath for scope filtering.

## Problem

The current view system has several issues:

1. **No type filtering on collect** - must manually filter by entity type in post-processing
2. **No scope filtering** - can't express "only include components belonging to document's bouwblokken"
3. **No relation filters** - can't filter based on relation properties
4. **Flat collections** - output doesn't match document rendering needs
5. **Output structure unclear** - must trace all traverse rules to understand what gets collected

## Solution

Query-as-Output-Structure: the YAML query tree IS the output structure.

```yaml
views:
  document_publish:
    query:
      $:  # Root (entry)
        type: document
        
        bouwbloks:
          via: describesBouwblok
          
          functions:
            via_incoming: partOfBouwblok
            type: function
            
            components:
              via_incoming: realizes
              type: component
              
              dependencies:
                via: dependsOn
                recursive: 5
                type: component
                # JSONPath scope filter
                require:
                  partOfBouwblok: $.bouwbloks[*].id
```

Output mirrors query:
```yaml
entry:
  id: DOC-001
  bouwbloks:
    - id: BB-001
      functions:
        - id: FUNC-001
          components:
            - id: COMP-001
              dependencies:
                - id: COMP-002
```

## Key Features

| Feature | Syntax |
|---------|--------|
| Follow outgoing relation | `via: relationName` |
| Follow incoming relation | `via_incoming: relationName` |
| Type filter | `type: entityType` |
| Property filter | `where: "status=active"` |
| Recursive traversal | `recursive: 5` (max depth) |
| Select properties | `only: [id, title]` |
| Scope constraint | `require: { partOfBouwblok: $.bouwbloks[*].id }` |

## Defaults

- All properties included by default (use `only:` to limit)
- Content included by default (use `content: false` to exclude)
- Relations included by default

## JSONPath for Scope Filtering

Use standard JSONPath (RFC 9535) syntax:

| Syntax | Meaning |
|--------|---------|
| `$` | Root |
| `$.bouwbloks` | Bouwbloks collection at root |
| `$.bouwbloks[*].id` | All bouwblok IDs |
| `$..components` | All components anywhere in tree |
| `[?@.status=='active']` | Filter expression |

Benefits:
- Well-documented standard
- Use existing Go library (ohler55/ojg)
- No custom parser to maintain

## Two-Pass Execution

1. **Pass 1 (Collect)**: Traverse graph, build output tree without filtering
2. **Pass 2 (Filter)**: Apply `require` clauses using JSONPath, top-down until stable

This allows siblings to reference each other regardless of YAML order.

Validation:
- Error if JSONPath references non-existent collection
- Warn if JSONPath matches nothing at runtime

## Entity Map

Auto-generated flat map of all collected entities for lookup:

```yaml
entry:
  # ... hierarchical structure

entities:
  COMP-001: { id: COMP-001, type: component, ... }
  COMP-002: { id: COMP-002, type: component, ... }
```

## Migration

This replaces the current traverse/filter model. Existing views.yaml files will need rewriting, but the new format is more intuitive and self-documenting.

## Implementation Steps

1. Define new YAML schema types
2. Implement two-pass engine (collect + filter)
3. Integrate JSONPath library (ohler55/ojg)
4. Add validation (JSONPath references, type existence)
5. Update formatter for hierarchical output
6. Update MCP tools and CLI
7. Migrate example views
8. Update documentation
