---
description: Allow relations to have typed properties and markdown content, with automatic UI adaptation in data-entry
id: FEAT-jsjj
priority: medium
status: proposed
summary: Extend metamodel to support properties on relation types, reusing PropertyDef and validation logic from entities
title: Relation properties and content support
type: feature
---

# Relation Properties and Content Support

## Overview

Relations currently only store from/type/to identifiers. This feature extends the metamodel to support typed properties and markdown content on relations, with automatic UI adaptation in data-entry.

## Goals

1. Define relation properties in metamodel.yaml using existing `PropertyDef` structure
2. Reuse property validation logic via `PropertySchema` interface
3. Auto-detect "advanced mode" in data-entry when relations have properties/content
4. Display relation properties as cards with edit modal (vs simple select widget)

## Metamodel Schema

```yaml
relations:
  addresses:
    from: [decision]
    to: [requirement]
    properties:                    # NEW
      rationale:
        type: string
        required: true
      impact:
        type: enum
        values: [low, medium, high]
        default: medium
    content: true                  # NEW: allows markdown body
```

## Interface Design

```go
// PropertySchema abstracts property definitions for entities and relations
type PropertySchema interface {
    GetProperties() map[string]PropertyDef
    HasContent() bool
}

// Both EntityDef and RelationDef implement PropertySchema
```

## UI Behavior

| Relation Config | Widget |
|----------------|--------|
| No properties, no content | Select/multi-select dropdown |
| Has properties or content | Cards with Edit button → Modal |

## Implementation Reuse

- `PropertyDef` struct: no changes
- `validatePropertyValue()`: no changes  
- `CustomType` definitions: already shared
- New `ValidateProperties()` function shared between entities and relations
