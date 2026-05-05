---
id: TKT-6HTTX
type: ticket
title: MCP update_entity should support deleting properties
kind: enhancement
priority: medium
effort: s
status: review
---

## Problem

When a property is removed from `metamodel.yaml`, existing entities still carry
the stale property in their frontmatter. The MCP `update_entity` tool only
adds/overwrites known properties — it has no way to remove a property. Same
problem when an author wants to clear a single value on one entity.

GitHub issue: https://github.com/sourcehaven/rela-9/issues/643

## Scope

IN scope:

- A way for MCP clients (AI assistants) to remove a property from an entity via `update_entity` (no separate tool, no schema-evolution autocleanup).
- The exact mechanism is a design call (the issue suggests `null`, an `unset` array, or auto-pruning). The minimum is: an MCP client CAN remove a property without editing files directly.
- Test coverage for the chosen mechanism.
- Update MCP tool description so AI assistants discover the capability.

OUT of scope:

- Auto-pruning on every entity write (most invasive option from the issue) — defer unless we explicitly want it.
- A new top-level `delete_property` MCP tool (overlapping with `update_entity`).
- Bulk schema-migration tooling.

## Acceptance criteria

1. Given an entity with property `foo=bar`, an MCP client can call `update_entity` in a way that removes `foo` from the entity's frontmatter on disk.
2. The tool description / schema makes the capability discoverable to AI assistants.
3. Existing `update_entity` semantics are preserved — non-destructive set/overwrite still works.
4. Removing a property that doesn't exist on the entity is a no-op (not an error).
5. Property-name validation behavior is documented (decide: must the property exist in the metamodel to be unset, or can stale properties be unset).
6. Unit tests cover: remove existing property, no-op for absent property, mixed set+unset in one call, validation behavior for unknown property names.
