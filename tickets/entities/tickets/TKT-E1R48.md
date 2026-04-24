---
id: TKT-E1R48
type: ticket
title: MCP create_entity ignores id_type — allows custom ID on short/sequential types
kind: enhancement
priority: medium
effort: s
status: review
---

## Summary

The MCP `create_entity` tool accepts an `id` parameter for any entity type, even
when the metamodel declares `id_type: short` or `id_type: sequential` — in which
case IDs must be auto-generated. Today, supplying a custom ID on such a type
silently succeeds (modulo charset/duplicate checks in `entity.ValidateID`),
bypassing the schema contract.

## Reproduce

Call `create_entity` with `type: ticket` (ticket uses short IDs) and `id:
my-custom-id` — the entity is created with the manual ID.

## Expected

The MCP layer (and the underlying create path) must reject a caller-supplied ID
when the entity type's `id_type` is `short` or `sequential`. Only `id_type:
manual` should permit a caller-supplied ID.

## Scope

Fix at the convergence point (`workspace.createEntityCore` via
`entitymanager.CreateEntity`) so MCP, data-entry API, and Lua all benefit. Error
message must be actionable — name the entity type, its `id_type`, and the
offending id.

## Affected code

- `internal/mcp/tools_entity.go:139` — `handleCreateEntity` forwards `customID` without checking `id_type`
- `internal/workspace/workspace.go:905` — `createEntityCore` only calls `entity.ValidateID` (charset check), no `id_type` enforcement
- `internal/dataentry/api_v1.go:370` — same permissive path exists in data-entry API (fixing at the core benefits this too)
