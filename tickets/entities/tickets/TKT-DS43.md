---
id: TKT-DS43
type: ticket
title: Migrate CLI production code off workspace.Workspace to appbuild.Services
kind: refactor
priority: high
effort: m
status: ready
---

Replace `*workspace.Workspace` usage in `internal/cli` (production + tests) with
`*appbuild.Services`. `cliServices`'s accessor methods already match
`appbuild.Services`'s surface 1:1, so the migration is largely mechanical.

**Note:** TKT-UG3C (test fixture) was folded into this ticket — the production
swap can't compile while CLI tests still use `workspace.NewForTest` +
`newCLIServicesFromWorkspace` (they share the `cliServices.svc` field). Doing
them together produces a single coherent diff.

**Changes:**

1. `internal/appbuild/appbuild.go`:
   - Add `Services.ScriptEngine() *script.Engine` accessor so consumers can reach `script.Engine.LuaCache()` (audit confirmed: `script.NewWriterRuntime` reads cache from `lua.Option` not `WriteDeps`, so `flow.go` still needs the explicit `WithCache`)

2. `internal/appbuild/testfixture.go` (new):
   - `NewForTest(meta *metamodel.Metamodel, opts ...TestOption) *Services`
   - `WithTestStore(store.Store)` — pre-built store for seeded fixtures
   - `WithFS(fs storage.FS, paths *project.Context)` — for paths-aware code
   - No `WithScript` (CLI tests don't drive automation)
   - Takes `*Metamodel` directly (bypasses loader → works with pre-migration test metamodels)

3. `internal/cli/cli_wiring.go`:
   - `cliServices.ws *workspace.Workspace` → `svc *appbuild.Services`
   - All `s.ws.X()` → `s.svc.X()`
   - `LuaCache()` delegates via `s.svc.ScriptEngine().LuaCache()`
   - `newCLIServices` → `appbuild.Discover`
   - `newCLIServicesFromWorkspace` → `newCLIServicesFromAppbuild`
   - Renametype panic message references `appbuild.WithFS` not `workspace.WithFS`

4. `internal/cli/flow.go`:
   - `workspace.Discover` → `appbuild.Discover`
   - `lua.WithCache(flowSvc.ScriptEngine().LuaCache())` (cache plumbing audit found `script.NewWriterRuntime` doesn't read cache from WriteDeps)

5. `internal/cli/validate.go`:
   - `workspace.Discover(startDir, workspace.NopScriptExecutor)` → `appbuild.Discover(startDir, script.NewEngine())`
   - `*workspace.Workspace` parameter on `runValidationChecks`/`runPropertiesCheck` → `*appbuild.Services`

6. CLI test files migrated: `test_helpers_test.go`, `export_test.go`, `validate_test.go`, `rename_test.go`. `seedWorkspace` → `seedServices`.

7. `.go-arch-lint.yml`: `cli.mayDependOn` gains `appbuild`; `appbuild.mayDependOn` gains `memstore` (for `NewForTest`).

**Audit finding (worth noting for future):** `script.NewWriterRuntime` does NOT
plumb the lua.Cache through `WriteDeps` — cache flows through `lua.Option`. The
original plan claimed otherwise. This means dropping `LuaCache()` from
`cliWrite` would have broken `flow.go`. Kept it; delegates via
`ScriptEngine().LuaCache()`.

**Scope:** ~400 LOC modified across production + tests; appbuild.NewForTest is
~150 LOC new.

Closes TKT-UG3C.
