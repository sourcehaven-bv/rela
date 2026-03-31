---
effort: m
id: TKT-ps58
kind: enhancement
priority: medium
status: done
title: Add test fixture builders with randomized data
type: ticket
---

## Problem

~283 instances of verbose `Properties: map[string]interface{}{...}` patterns across 110 test files. Each package re-implements its own entity creation helpers. Test intent is obscured by boilerplate - hard to see what properties are actually being tested.

## Solution

Add builder pattern helpers to `internal/testutil` with two variants:

### Simple Builder (no metamodel)
```go
entity := testutil.Entity("ticket").
    ID("TKT-001").
    With("status", "open").
    WithList("tags", "bug", "urgent").
    Build()
```

### Metamodel-Aware Builder (auto-fills required fields)
```go
entity := testutil.EntityFor(meta, "ticket").
    With("status", "open").  // override random value
    Build()
// Auto-generates: random ID, random title, random kind (from enum values)
```

### Random Value Generation by Type
- `string` → random word like "alpha-7x3f"
- `enum` → random pick from allowed values
- `integer` → random int in reasonable range
- `date` → random recent date
- `boolean` → random true/false

### Relation Builder
```go
relation := testutil.Relation("implements").
    From("TKT-001").
    To("FEAT-001").
    Build()
```

## Benefits

- Reduces test code by ~70% for entity creation
- Makes test-specific properties stand out
- Centralizes test data patterns in one place
- Metamodel-aware builder ensures valid entities with minimal code
