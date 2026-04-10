---
id: TKT-7WKTZ
type: ticket
title: Separate metamodel reload from data reload in watcher
kind: refactor
priority: high
effort: m
status: ready
---

## Problem

The file watcher calls `Reload()` on every file change, which re-reads the
metamodel AND re-syncs all entities/relations from disk. This is wasteful:
entity/relation changes (the common case) don't need a metamodel reload, and
metamodel changes don't need the full entity parse if nothing else changed.

More importantly, the metamodel's `includes:` files (e.g., `types.yaml`,
`entities.yaml`) are not in the watcher's file list, so changes to include
files don't trigger any reload at all.

See `.ignored/database-lessons.md` proposal #2 ("Catalog vs. Data Lifecycle").

## Approach

1. **In the watcher callback**, inspect event paths to determine what changed:
   - If metamodel.yaml (or an include file) changed → full `Reload()` (meta + graph)
   - If only entities/relations changed → `Sync()` (graph only, keep current meta)
2. **Add include files to the watch list** so changes to `types.yaml` etc. trigger reloads
3. The metamodel's `Includes` field (post-parse) lists the include paths. After initial
   load, pass these to the watcher as extra files.

## Scope

**In scope:**
1. Modify `StartWatching` callback to inspect event paths
2. Add metamodel include files to the watcher's file list
3. Call `Sync()` for data-only changes, `Reload()` for metamodel changes
4. Test that both paths work correctly

**Out of scope:**
- Removing metamodel from `workspaceState` (catalog lifecycle proposal #2 full version)
- Making metamodel restart-only (the user explicitly chose option 3 — smart reload)
- Snapshot API Phase 2 (dataentry migration)

## Acceptance Criteria

1. Entity/relation file changes trigger `Sync()`, not `Reload()`
2. `metamodel.yaml` changes still trigger full `Reload()`
3. Metamodel include file changes trigger full `Reload()`
4. `views.yaml` changes trigger full `Reload()` (it affects view definitions)
5. Mixed changes (metamodel + entities in same batch) trigger `Reload()`
6. All existing tests pass
7. `go test -race ./...`, `just lint`, `go-arch-lint check` all pass
