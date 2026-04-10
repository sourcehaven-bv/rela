---
id: TKT-910WC
type: ticket
title: Introduce workspace.Snapshot as the consumer read API
kind: refactor
priority: high
effort: l
status: done
---

## Problem

Consumer packages (`cli`, `dataentry`, `mcp`) reach through
`workspace.Workspace` to access `Graph()`, `Meta()`, and `Search()` directly.
This leaks internal types (`graph.Graph`, `metamodel.Metamodel`) into consumers
and means each call site independently loads from the atomic pointer — multiple
calls within a handler can observe different snapshots if a reload lands between
them.

See `.ignored/database-lessons.md` proposal #1 ("Snapshot as the Consumer API")
for full context.

## Current state

- **dataentry**: 151+ calls to `a.Graph()` / `a.Meta()` across 12 files (heaviest consumer)
- **mcp**: 45+ calls to `s.ws.Graph()` / `s.ws.Meta()` across 12 files
- **cli**: 7 calls across 6 files (lightest consumer)
- **lua**: indirect via workspace interface

`dataentry` already has an `AppState` snapshot struct, but consumers still call
`a.Graph()` and `a.Meta()` which load from the atomic pointer on each call.

## Approach

Introduce `workspace.Snapshot` that wraps `workspaceState` and provides the
consumer-facing read API. Consumers call `ws.Snapshot()` once at the top of
their operation and use the snapshot for all reads within that scope.

### Phase 1: Introduce the type and migrate MCP (this ticket)

MCP is the cleanest target — each tool handler is a standalone function that
calls `s.ws.Graph()` and `s.ws.Meta()` at the top and uses them throughout.
Migration is mechanical:

```go
// Before
func (s *Server) handleListEntities(...) {
    g := s.ws.Graph()
    meta := s.ws.Meta()
    ...
}

// After
func (s *Server) handleListEntities(...) {
    snap := s.ws.Snapshot()
    g := snap.Graph()
    meta := snap.Meta()
    ...
}
```

### Phase 2: Migrate dataentry (separate ticket)

dataentry's `AppState` already acts as a snapshot. The migration there is more
about ensuring handlers capture state once.

### Phase 3: Migrate CLI (separate ticket)

CLI is trivial — 7 call sites.

## Scope (this ticket — Phase 1 only)

**In scope:**
1. Create `workspace.Snapshot` type wrapping `*workspaceState`
2. Add methods: `Graph()`, `Meta()`, `Search()`, `GetEntity()`, `GetRelation()`, `AllEntities()`, `EntitiesByType()`, `AllRelations()`
3. Add `Workspace.Snapshot() *Snapshot`
4. Migrate all MCP tool/prompt/resource handlers to use `snap := s.ws.Snapshot()`
5. Verify `internal/mcp` still works with all existing tests

**Out of scope:**
- Migrating dataentry (Phase 2)
- Migrating CLI (Phase 3)
- Removing `ws.Graph()` / `ws.Meta()` (can't until all consumers migrated)
- Hiding `graph.Graph` behind the snapshot (future — snapshot methods still return `*graph.Graph` for now)

## Acceptance Criteria

1. `workspace.Snapshot` type exists with read-only methods
2. `Workspace.Snapshot()` returns a consistent point-in-time view
3. All MCP handlers use snapshot instead of direct `ws.Graph()` / `ws.Meta()`
4. All existing MCP tests pass
5. `go test -race ./...`, `just lint`, `go-arch-lint check` all pass

## Completion

- Phase 1 (MCP): PR #368, merged
- Phase 3 (CLI): PR #372, merged
- Phase 2 (dataentry): deferred — separate follow-up
