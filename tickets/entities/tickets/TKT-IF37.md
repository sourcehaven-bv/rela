---
id: TKT-IF37
type: ticket
title: Narrow lua.WriteDeps.EntityManager and autocascade.Mutator to the 5 methods lua actually calls
kind: refactor
priority: low
effort: s
status: done
---

## Summary

Hygiene follow-up to TKT-Z9MR. Today `lua.WriteDeps.EntityManager` is typed as
the wide `entitymanager.EntityManager` (7 methods); to satisfy structural
assignment in `LuaScriptRunner.Run`, `autocascade.Mutator` had to grow to 7
methods too — even though only 5 are invoked by Lua bindings (CreateEntity /
UpdateEntity / DeleteEntity / CreateRelation / DeleteRelation; `RenameEntity`
and `UpdateRelation` are unused).

Narrow both to the 5-method actual surface.

## In scope

- Define a 5-method consumer-side interface in `internal/lua` (e.g., `lua.Mutator` or rename `lua.WriteDeps.EntityManager` field to be a narrower type).
- `lua.WriteDeps.EntityManager` field type narrows to the new interface.
- `autocascade.Mutator` narrows to the same 5 methods.
- `entitymanager.Manager` continues to satisfy both structurally.
- `internal/lua` may drop its `internal/entitymanager` import entirely once the field type narrows.
- Update tests that stub `entitymanager.EntityManager` for Lua callers — narrow the stubs.

## Out of scope

- `entitymanager.EntityManager` interface itself stays 7-method (still serves non-script callers like CLI/MCP/dataentry HTTP handlers that need rename / update-relation).

## Why

After TKT-Z9MR, `autocascade.Mutator` reads "mirrors entitymanager.EntityManager
for shape symmetry" — that's a smell. The consumer (Lua scripts) only needs 5
methods; the interface should reflect that. Cleans up the godoc claim, removes
lua's transitive dependency on entitymanager, and is a small CLAUDE.md hygiene
win ("consumer-side interfaces at the call site").
