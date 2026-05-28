---
id: TKT-7I3P
type: ticket
title: 'Slice 1: mcp.Deps replaces mcp.Services interface'
kind: refactor
priority: medium
effort: s
status: done
---

## Description

Slice 1 of the service-layering plan (`.ignored/service-layering-plan.md`,
crit-approved).

Replaces the `mcp.Services` interface (11 producer-shaped accessor methods) with
a `mcp.Deps` struct of domain types. The MCP `Server` holds a `Deps` value built
by the cli wiring site; it has no reference to any composition-root aggregate.

## Changes

- `internal/mcp/server.go`: `Services` interface → `Deps` struct; `Server.ws` → `Server.deps`; `NewServer(Deps, ...)`.
- All `s.ws.X()` accessor calls → `s.deps.X` field reads across the mcp tool/resource/prompt files.
- `Paths() *project.Context` narrowed to `ProjectRoot string` — the only piece mcp consumed. Eliminates the `internal/project` import from mcp's non-test code and from its test stubs.
- `internal/cli/mcp_wiring.go`: `mcpServices` keeps lifecycle (Close) but exposes `Deps() mcp.Deps` instead of satisfying `mcp.Services`.
- Test stubs become struct literals: `newTestServices` (+ the `testServices` adapter type) replaced by `newTestDeps` returning a `mcp.Deps`.
- `.go-arch-lint.yml`: removed stale `project` from mcp's `mayDependOn`; documented that appbuild is deliberately absent (whitelist model).

## Verification

- `just build` (default + `-tags memorybackend`), `just test`, `just lint`, `just arch-lint` all clean.
- `internal/mcp` imports neither `internal/appbuild` nor `internal/project`.
