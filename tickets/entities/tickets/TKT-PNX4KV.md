---
id: TKT-PNX4KV
type: ticket
title: 'Computed properties: reject enum literals outside the enum at load'
kind: enhancement
priority: medium
effort: s
status: done
---

## Description

A computed enum property can store a value outside its enum when a selection
branch contains a mistyped literal:

```yaml
level:
  type: assurance # low, medium, high
  computed: entity.method == 'passkey' and 'hgh' or 'low'
```

This compiles, and it fails only when a write takes the `'hgh'` branch (write
validation rejects the value). The literal is known when the schema loads, so
the error can be reported there, naming the property and the literal. Deferred
from TKT-WQJGPS (RR-RWLE9X).

## Approach

- `internal/predicate`: `Program.ResultLiterals()` returns the constant values that can become the program's result: the root when it is a constant, and the value leaves of a selection (right of `and`, both sides of `or`), recursively. Anything else (attribute reads, concatenation, calls) contributes nothing.
- `internal/computed.compileProperty`: when the property has enum values (inline `values:` or a custom type), every string result literal must be one of them; otherwise a load error naming the property and the literal.

## Acceptance criteria

- A mistyped literal in any selection branch or as the whole expression is a schema-load error naming property and literal.
- Valid literals, attribute branches and non-enum properties are unaffected.
- Works for inline enums and custom enum types.
