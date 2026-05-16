---
id: TKT-DS43
type: ticket
title: Migrate CLI production code off workspace.Workspace to appbuild.Services
kind: refactor
priority: high
effort: m
status: backlog
---

Replace `*workspace.Workspace` usage in `internal/cli` production code with
`*appbuild.Services`. `cliServices`'s accessor methods already match
`appbuild.Services`'s surface 1:1, so the migration is largely mechanical.

**Changes:**

1. `internal/cli/cli_wiring.go`:
   - `cliServices.ws *workspace.Workspace` → `svc *appbuild.Services`
   - All `s.ws.X()` → `s.svc.X()`
   - `newCLIServices` → `appbuild.Discover` (not `workspace.Discover`)
   - Rename `newCLIServicesFromWorkspace` → `newCLIServicesFromAppbuild`
   - Drop `cliWrite.LuaCache()` from the bundle interface entirely. The prior LuaWriteDeps refactor already plumbed cache through WriteDeps; only `flow.go` consumes `LuaCache()`.
   - Update the renametype panic message reference from `workspace.WithFS` → `appbuild.WithFS`

2. `internal/cli/flow.go`:
   - `workspace.Discover` → `appbuild.Discover`
   - `flowWs.LuaCache()` / `lua.WithCache(...)` → dropped. **AUDIT FIRST**: confirm `script.NewWriterRuntime` reads the cache from `WriteDeps`; if not, that's a real gap to surface
   - `flowWs.LuaWriteDeps()` → `svc.LuaWriteDeps()` (same shape)
   - No `defer svc.Close()` — short-lived CLI subcommand

3. `internal/cli/validate.go`:
   - `workspace.Discover(startDir, workspace.NopScriptExecutor)` → `appbuild.Discover(startDir, script.NewEngine())`
   - Engine init is cheap; no NopScriptExecutor equivalent on appbuild
   - `*workspace.Workspace` parameter on `runValidationChecks`/`runPropertiesCheck`/etc. → `*appbuild.Services`
   - No `defer checkWs.Close()`

4. `.go-arch-lint.yml`: `cli.mayDependOn`: add `appbuild` (keep `workspace` until TKT-NEW-C completes).

Does NOT touch CLI test files (those use `workspace.NewForTest`; covered by
follow-up TKT-NEW-C).

**Scope:** ~150 LOC modified.

See `.ignored/cli-off-workspace-plan.md` PR 2 for full detail.
