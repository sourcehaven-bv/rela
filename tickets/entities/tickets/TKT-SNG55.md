---
id: TKT-SNG55
type: ticket
title: Replace lua.Services struct with minimal consumer interfaces per call site
kind: refactor
priority: medium
effort: m
status: in-progress
---

## Description

Today `lua.Services` is a single fat struct (Store, Manager, Tracer, Searcher,
Meta, ProjectRoot) that every call site must construct in full, even when it
only needs a subset. This couples call sites to capabilities they don't use,
forces hacks like `svc.Manager = nil` to disable writes in the validation path,
and led to the `interface{}` leak in `metamodel.ScriptContext.GetWorkspace()`
(see BUG-WQ7Y).

Follow Go best practice — "accept interfaces, return structs" and "interfaces
defined where consumed" — by replacing the single `Services` struct with small
role-specific interfaces (e.g. `EntityReader`, `EntityWriter`, `RelationWriter`,
`Tracer`, `Searcher`, `MetaProvider`) defined in the `lua` package and satisfied
by the `workspace`. Each call site wires only the capabilities it actually uses.

Secondary goal: eliminate the `GetWorkspace() interface{}` type-assertion
contract in `metamodel.ScriptContext` by giving the script package a typed
dependency.

## Acceptance Criteria

- `lua.Services` struct is gone (or replaced by an internal type) — public API is interface-based
- Each call site (`cli/script.go`, `cli/flow.go`, `mcp/tools_lua.go`, `script/executor.go`, `script/action.go`, `validation/lua.go`, `dataentry/actions.go`, `scheduler`) declares only the capabilities it needs
- Validation path no longer needs `svc.Manager = nil` — it uses a read-only capability set by construction
- `metamodel.ScriptContext.GetWorkspace() interface{}` no longer returns `interface{}`
- `go-arch-lint` still passes; no new package-cycle violations
- All existing tests pass; coverage ratchet holds
- Lua bindings (`rela.*`, `ai.*`) remain functionally identical from script perspective
