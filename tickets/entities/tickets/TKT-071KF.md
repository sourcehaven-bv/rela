---
id: TKT-071KF
type: ticket
title: Entity and relation help system with metamodel-driven documentation
kind: enhancement
priority: medium
status: backlog
---

<!-- github-issue:162 -->

Imported from [#162](https://github.com/sourcehaven-bv/rela/issues/162)

## Summary

Add automatic, contextual help for entities and relations throughout rela. Users
should be able to understand what each entity type is for, what properties mean,
and how relations connect things—without leaving the interface.

## Current State

The metamodel already supports `description` fields for:
- Properties (`PropertyDef.Description`)
- Relations (`RelationDef.Description`)

The CLI schema command already displays these. However:
- Entity types themselves have no description field
- TUI doesn't display property/relation descriptions
- Data entry forms don't pull descriptions from metamodel
- No contextual help (tooltips, help panels, `?` key)

## Proposed Features

### 1. Entity Type Descriptions in Metamodel

Add a `description` field to entity type definitions:

```yaml
entities:
  ticket:
    label: Ticket
    description: |
      A unit of work to be completed. Tickets track bugs, features,
      enhancements, and other actionable items. Each ticket should
      be linked to the concepts it affects and features it implements.
    properties:
      # ...
```

### 2. Contextual Help in TUI

- **Metamodel view**: Show entity/relation descriptions alongside type listings
- **Detail view**: Press `?` to toggle a help panel showing:
  - Entity type description
  - Property descriptions for visible fields
  - Relation descriptions for connected entities
- **Browser view**: Press `?` to show help for currently selected entity type
- **Form fields**: Show property descriptions inline or on focus

### 3. CLI Help Integration

- `rela schema <type>` already shows descriptions—keep this
- `rela help <type>` as alias for schema with enhanced formatting
- `rela help <type>.<property>` for property-specific help
- `rela help relations <type>` for relation help

### 4. Data Entry Web UI

- Pull `description` from metamodel as default help text (fallback)
- Show descriptions in form field tooltips
- Add help icons that expand to show full descriptions
- "What is this?" links that explain entity purpose

### 5. MCP Help Resources

- `rela://help/entity/{type}` resource with full documentation
- `rela://help/relation/{type}` resource
- `rela://help/property/{entity}/{property}` resource
- AI assistants can query these for contextual understanding

## Brainstorm: Additional Ideas

### Auto-generated Documentation

- `rela docs generate` command that outputs markdown documentation
- Could generate a "project guide" explaining all entity types
- Mermaid diagrams showing relation types between entities

### Validation Message Improvements

- When validation fails, show relevant descriptions
- "This property (status) must be one of: draft, ready, done. Status indicates the workflow state of the ticket."

### Inline Examples

Add `example` field to property definitions:

```yaml
properties:
  why1:
    type: string
    description: "5-Whys level 1: What was the immediate cause?"
    example: "The test didn't check for null values"
```

### Relation Guidance

Show "suggested relations" when creating entities:

```
Creating a ticket...
💡 Tickets typically need:
  - affects → concept (what area this touches)
  - implements → feature (what feature this delivers)
```

### Quick Reference Card

`rela help --quick` or `?` in TUI shows a condensed reference:

```
ENTITY TYPES
  ticket     Work item (bug, feature, task)
  feature    Product capability
  concept    Architectural area

COMMON RELATIONS
  affects     ticket → concept
  implements  ticket → feature
  requires    feature → concept
```

### Interactive Tutorial

`rela tutorial` or first-run guidance that walks through:
1. What entities are available
2. How they relate
3. Creating first entity with relation

### Metamodel Validation for Help

Warn when descriptions are missing:

```
⚠️  Missing descriptions:
  - Entity type 'component' has no description
  - Property 'ticket.assignee' has no description
  - Relation 'depends-on' has no description
```

## Implementation Notes

### Schema Changes (internal/metamodel/types.go)

```go
type EntityDef struct {
    Label       string   `yaml:"label"`
    Description string   `yaml:"description"`  // NEW
    // ...
}
```

### TUI Changes

- Add help panel component
- `?` key binding in relevant views
- Description rendering in metamodel view

### Priority

1. Add `description` to EntityDef (schema change)
2. Display in CLI schema command
3. TUI metamodel view descriptions
4. TUI contextual help (`?` key)
5. Data entry tooltip integration
6. MCP help resources

## Related

- Property descriptions already work in CLI
- Data entry forms support help text (manually configured)
- MCP already has schema introspection tools

## Questions

- Should descriptions support markdown formatting?
- Should we enforce descriptions in strict mode?
- How verbose should inline help be vs linking to full docs?
