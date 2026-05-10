---
id: TKT-Y0JU
type: ticket
title: Narrow lua.WriteDeps.EntityManager to an EntityMutator interface
kind: refactor
priority: low
effort: xs
status: ready
---

## Summary

`internal/lua/deps.go` holds the full `entitymanager.EntityManager` interface (7
methods) in `WriteDeps.EntityManager`. Lua bindings call only a subset
(CreateEntity, UpdateEntity, DeleteEntity, CreateRelation, UpdateRelation,
DeleteRelation). Define a smaller consumer-side interface in `internal/lua` that
names exactly what bindings need.

## In scope

- New `lua.EntityMutator` interface in `internal/lua/deps.go` (or a new file in the same package), with the 3–6 methods Lua bindings actually call.
- `WriteDeps.EntityManager` field changes type from `entitymanager.EntityManager` to `lua.EntityMutator`.
- `entitymanager.EntityManager` (the full interface) still satisfies `lua.EntityMutator` structurally — no changes there.
- Update tests / mocks that supplied stub EntityManagers; they only need to implement the smaller interface now.

## Out of scope

- Changes to `entitymanager.EntityManager` itself.
- Changes to Lua bindings beyond what's needed to compile against the new interface.
- Renaming `WriteDeps.EntityManager` field; keep the field name (it's clearer than `Mutator` or `Writer` for callers).

## Why

Pure idiomacy / decoupling improvement. No cycle today; no test pain to speak
of. Worth doing eventually so future EntityManager surface changes don't ripple
through Lua bindings.

## Risks

- None significant. Pure narrowing refactor.
- If the EntityMutator interface grows to ~6 methods (close to EntityManager's full surface) the value diminishes; if so, document the choice in the package doc and don't fight it.
