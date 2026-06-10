---
id: TKT-EL7RP1
type: ticket
title: Route MCP wiring through appbuild (dedupe second composition root)
kind: refactor
priority: medium
effort: s
status: done
---

## Summary

`internal/cli/mcp_wiring.go` (`newMCPServices`) re-implemented the entire write
stack — automation engine + `autocascade.New`, `audit.NewFilesystem`,
`entitymanager.New`, `validator.New`, `script.NewLuaScriptRunner`, lua read deps
— a near-line-for-line duplicate of `appbuild.prepare` + `assemble`. The three
`mcp_wiring_{fs,memory,postgres}.go` seams duplicated
`appbuild_{fs,memory,postgres}.go` (`openMCPBackend` ≈ `openBackend`, down to a
parallel `mcpNoopCloser`/`noopCloser`).

This is the follow-up to [[TKT-KWAX]] (migrate MCP off Workspace), which created
the standalone MCP wiring in the first place. Now that `appbuild` is the shared
composition root for rela-server and rela-desktop, MCP joins it.

## Changes

- `newMCPServices` calls `appbuild.Discover(startDir, script.NewEngine(), appbuild.WithACL(acl.NopACL{}))` and builds `mcp.Deps` from the `*appbuild.Services` accessors. `mcpServices` becomes a thin holder of `(*appbuild.Services, mcpWatcher)`; `svc.Deps()` / `svc.Close()` keep their signatures so `mcp.go` is untouched.
- Deleted `mcp_wiring_fs.go`, `mcp_wiring_memory.go`, `mcp_wiring_postgres.go`, `mcp_wiring_search_fs_test.go`, and the `openMCPBackend` / `backfillMCPBackend` / `mcpNoopCloser` symbols. appbuild's per-build recipes supply the store/searcher/closer.
- Kept the `mcpWatcher` adapter + `storeStartStopper` capability interface (the one genuinely MCP-specific piece; appbuild has no watcher story by design).

Net −286 lines, 4 files removed. No behavior change.

## ACL decision (deliberate NopACL)

MCP is a **local stdio** transport. Anyone who can launch `rela mcp` already has
filesystem write access to the entity markdown and can edit it directly,
bypassing every gate — so policy enforcement on the MCP tool surface defends
nothing; the filesystem is the trust boundary. `appbuild.WithACL(acl.NopACL{})`
makes this an explicit, justified opt-out rather than the previous silent `ACL:
acl.NopACL{}` default. Access control that matters belongs on the deployed HTTP
API, which serves callers without direct file access (see [[TKT-G3PPD]] for the
MCP-transport/ACL intersection follow-up).

## Verification

`go test ./...` (default, plus `-tags memorybackend` and `-tags postgres` builds
compile), `golangci-lint`, and `just arch-lint` all pass. The existing
`mcp_wiring_test.go` behavior tests (no-project, bad-metamodel, succeeds,
writes-reach-index, close-idempotent, watcher) are unchanged and green.
