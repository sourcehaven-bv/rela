---
id: TKT-2IAC
type: ticket
title: Migrate scheduler to wire its own services (off Workspace)
kind: refactor
priority: medium
effort: s
status: done
---

## Summary

`internal/scheduler` already has a consumer-side `WorkspaceProvider` interface
(4 methods: Paths, Config, State, LuaWriteDeps). Today this is satisfied by
`Workspace`. Once focused services exist, the scheduler can be wired with a thin
struct that satisfies `WorkspaceProvider` from individually-constructed pieces.

## In scope

- `cmd/rela scheduler` (the entry point) and `cmd/rela-server`'s scheduler boot path build a small struct holding `Paths`, `Config.Loader`, `state.KV`, and a `LuaWriteDeps`-equivalent assembled from focused services. The struct satisfies `WorkspaceProvider`.
- `internal/cli/scheduler.go` no longer calls `workspace.Discover`; it constructs the focused services + provider directly.
- Existing scheduler tests still pass.

## Out of scope

- Changes to the `WorkspaceProvider` interface itself.
- Changes to scheduler scheduling logic, state persistence, or task execution.
- The scheduler in rela-server's main loop (coordinated with the rela-server migration ticket).

## Depends on

- `entitymanager.Manager` real implementation (separate ticket).
- automation.Runner extraction (separate ticket).

## Why

Same reasoning as MCP migration: scheduler's `WorkspaceProvider` is already
tight, so the migration is mostly wiring. Demonstrates the pattern on a service
with explicit Lua-deps assembly needs.

## Risks

- Scheduler's `LuaWriteDeps` assembly today happens inside Workspace; the wiring site has to reproduce it correctly with `triggered_by` ctx wrapping for audit (when audit lands). Verify by capturing scheduler-driven writes in a test.
