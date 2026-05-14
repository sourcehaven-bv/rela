---
id: TKT-KWAX
type: ticket
title: Migrate MCP server to wire its own services (off Workspace)
kind: refactor
priority: medium
effort: s
status: planning
---

## Summary

`internal/mcp/server.go` already declares a consumer-side `Services` interface
(CLAUDE.md cites it as the positive example). Today the wiring still goes
through `Workspace` which satisfies `Services`. Once `entitymanager.Manager`
exists as a standalone implementation, MCP can wire its services directly from
focused components and stop depending on Workspace.

## In scope

- `cmd/rela mcp` (the entry point) constructs Store, Meta, Tracer, Searcher, EntityManager, Validator individually and supplies them to `mcp.Server` via something that satisfies `mcp.Services`.
- A small `mcpServices` struct in `cmd/rela/mcp.go` (or a new wiring helper) that just holds the focused services and satisfies `mcp.Services`.
- `internal/cli/mcp.go` no longer calls `workspace.Discover`; it calls the new wiring path.
- All existing MCP tests still pass; if they instantiate via Workspace today, they migrate to instantiating via the new wiring (or via a test-side `mcp.Services` stub).

## Out of scope

- Changes to the `mcp.Services` interface itself (it's already correct).
- Changes to MCP tool implementations.
- Watcher wiring strategy for MCP — preserve current behavior.

## Depends on

- `entitymanager.Manager` real implementation (separate ticket).

## Why

MCP is the easiest command to migrate first because (a) `Services` is already a
tight consumer-side interface, and (b) MCP doesn't have the multi-service
post-construction wiring that the data-entry server has. Proves the
decomposition pattern on a small, contained surface before applying to
dataentry.

## Risks

- MCP currently relies on Workspace for `LuaWriteDeps()` and `LuaCache()`. Reproducing the exact Lua-deps assembly without going through Workspace needs care — assembly logic moves to the wiring site.
