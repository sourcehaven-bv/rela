---
id: PLAN-W21Z
type: planning-checklist
title: 'Planning: Migrate MCP server to wire its own services (off Workspace)'
status: done
---

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Problem.** `internal/mcp/server.go` declares a consumer-side `Services`
interface (CLAUDE.md cites it as the positive example) but
`*workspace.Workspace` is the only implementation. The interface itself imports
`internal/workspace` for `WatchOptions` and `ChangeEvent` types and asserts
compile-time satisfaction by `*Workspace`. That means `internal/mcp` cannot
exist without `internal/workspace` even though nothing in MCP needs
workspace-specific behavior beyond what `Services` names.

After this ticket lands, `cmd/rela mcp` constructs Store, Meta, Tracer,
Searcher, Validator, EntityManager, Config, Paths, LuaWriteDeps, LuaCache
directly, supplies them to MCP via a wiring helper, and `internal/mcp` drops its
`internal/workspace` import.

**Scope (in):**

- New wiring file `internal/cli/mcp_wiring.go` constructs the focused dependencies and exposes a struct that satisfies `mcp.Services`.
- `internal/mcp/server.go::Services` interface keeps its current shape on the read side, but watcher methods (`StartWatching` / `StopWatching` / `PauseWatching` / `ResumeWatching` taking `workspace.WatchOptions`) are replaced with a single `Watcher() mcp.Watcher` capability declared inline in `internal/mcp` (4 methods, narrow, consumer-side).
- `internal/cli/mcp.go` stops calling `workspace.Discover`; calls the new wiring path.
- `internal/mcp/tools_test.go` (one call site to `workspace.NewForTest`) migrates to a stub `mcp.Services` for the test surface it actually needs — see Step 6 below.
- `internal/mcp/watcher_test.go` relocates to `internal/workspace/` or gets deleted (storage watcher tests already cover the underlying behavior — see Risks).

**Scope (out):**

- Changes to MCP tool implementations (tools_entity, tools_relation, etc.) other than the `PauseWatching` / `ResumeWatching` call-site rename to `Watcher().Pause()` / `Watcher().Resume()`.
- Changes to the Services interface read-side methods (Store, Meta, Tracer, Searcher, Validator, EntityManager, Config, Paths, LuaWriteDeps, LuaCache).
- Watcher behavioral changes — preserve current semantics (200ms debounce, fsstore feature-test).
- Removing `Workspace.StartWatching` / etc. methods — they stay on Workspace for now; only their typed contract with MCP changes. Other consumers (data-entry, scheduler) still call these methods.
- Building dataentry/CLI/scheduler off Workspace (separate tickets).
- Migrating the search-backend wiring to MCP's helper (TKT-Q1JT just landed the Observer pattern; MCP's helper uses it directly — ~5 LOC instead of the ~30 LOC the original plan worried about).

**Acceptance Criteria:**

1. `grep -r 'internal/workspace' internal/mcp/` (excluding `_test.go`) returns zero matches.
2. `cmd/rela mcp` still starts a working MCP server: file watcher fires on entity file changes; all MCP tools work end-to-end. Verified by existing MCP integration tests passing (tools_test, tools_lua_test, convert_test, watcher_test or its replacement).
3. The wiring helper is small: target ≤ 200 LOC (revised up from the original 100 — see "Wiring complexity" in Approach for why).
4. `mcp.Services` interface stays narrow: ≤ 11 methods, no growth. `mcp.Watcher` is 4 methods. No service-locator god-struct, no late-binding setters.
5. `just lint`, `just arch-lint`, `just test -race`, `just ci` all pass.

## Research

- [x] Searched for existing libraries that solve this problem
- [x] Checked codebase for similar patterns or reusable code
- [x] Looked for reference implementations in other projects
- [x] Reviewed relevant rela concepts for prior art

**Reference implementations in repo:**

- `internal/scheduler/scheduler.go` — 4-method consumer-side `WorkspaceProvider` interface. Wiring lives in CLI.
- `internal/lua/deps.go` — `ReadDeps` / `WriteDeps` capability bundles, constructed at wiring sites.
- TKT-LCTG (PR #718, merged) — workspace migrated to `bleveindex.NewMem()` + `search.New(store, backend)`; the same primitives the MCP helper will call.
- TKT-Q1JT (PR #720, in review) — `app.FSFactory.AddObserver(observer)` so the search backend is wired as a synchronous observer on the store before OpenStore. MCP's helper uses the same pattern.
- TKT-IU2S (PR #711, merged) — `entitymanager.Manager` is the production write path. MCP constructs its own Manager instance directly.

**The ScriptRunner cycle.** `entitymanager.Manager` requires
`autocascade.ScriptRunner` if the metamodel declares automations. The
ScriptRunner is the adapter that translates a cascade action into a Lua
execution. The runner needs `lua.WriteDeps`, which holds a reference back to the
EntityManager so Lua scripts can call into CRUD. This is the cycle that
`wsScriptRunner` resolves via per-call `w.LuaWriteDeps()` — see
`internal/workspace/wsscriptrunner.go`.

MCP's helper must reproduce this pattern: a small adapter struct in
`internal/cli/mcp_wiring.go` that resolves `lua.WriteDeps` per cascade dispatch,
using the helper's own Manager reference.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

### Step 1 — Replace watcher methods on `mcp.Services` with a narrow capability

Today:

```go
// internal/mcp/server.go
type Services interface {
    // ... read services (Store, Meta, Tracer, ...)
    PauseWatching()
    ResumeWatching()
    StartWatching(workspace.WatchOptions) error
    StopWatching()
}
```

The `workspace.WatchOptions` parameter is the load-bearing import. The MCP body
only does `s.ws.StartWatching(workspace.WatchOptions{ OnChange: func(_
[]workspace.ChangeEvent) { ... }})` and ignores the event slice. So MCP only
needs "start watching, call this when anything changes."

After this ticket:

```go
// internal/mcp/server.go
type Services interface {
    // ... read services unchanged ...
    Watcher() Watcher
}

// Watcher is the narrow file-watching capability MCP requires from
// its wiring site. The implementation is supplied by the helper at
// construction time.
type Watcher interface {
    Start(onChange func()) error
    Stop()
    Pause()
    Resume()
}
```

`internal/mcp` no longer imports `workspace.WatchOptions` or
`workspace.ChangeEvent`. The wiring site supplies a `mcp.Watcher` adapter that
wraps `storage.Watcher`.

### Step 2 — Update MCP call sites

- `server.go::Serve` — `s.ws.StartWatching(workspace.WatchOptions{OnChange: ...})` becomes `s.ws.Watcher().Start(func() { ... })`; `s.ws.StopWatching()` becomes `s.ws.Watcher().Stop()`.
- `tools_entity.go:331-332` — `PauseWatching` / `ResumeWatching` become `Watcher().Pause()` / `Watcher().Resume()`.
- `var _ Services = (*workspace.Workspace)(nil)` — DELETE the compile-time assertion. Workspace no longer satisfies the new Services interface (the watcher shape changed); that's fine because workspace won't be a Services implementation after this ticket.

### Step 3 — Build the wiring helper

`internal/cli/mcp_wiring.go` (new file) constructs the focused services and
returns a struct that satisfies `mcp.Services`. The structure:

```go
// internal/cli/mcp_wiring.go
package cli

type mcpServices struct {
    store    store.Store
    meta     *metamodel.Metamodel
    tracer   tracer.Tracer
    searcher search.Searcher
    valid    validator.Validator
    em       entitymanager.EntityManager
    config   config.Loader
    paths    *project.Context
    luaDeps  func() lua.WriteDeps  // resolved per-call (cycle-breaker)
    luaCache *lua.Cache
    watcher  mcp.Watcher
}

// Methods that satisfy mcp.Services...

func newMCPServices(...) (*mcpServices, error) {
    // 1. project.Discover, load metamodel
    // 2. bleveindex.NewMem()
    // 3. factory := &app.FSFactory{FS, Paths}; factory.AddObserver(backend)
    // 4. store, _ := factory.OpenStore(meta)
    // 5. backfill backend from store
    // 6. tracer := tracer.New(store)
    // 7. searcher := search.New(store, backend)
    // 8. validator := validator.New(store, meta, luaReadDeps)
    // 9. Manager construction (with ScriptRunner adapter — see Step 4)
    // 10. config.NewFSLoader, lua.NewCache, storage.Watcher adapter
    // 11. assemble mcpServices, return
}
```

### Step 4 — ScriptRunner adapter

`entitymanager.Manager` needs an `autocascade.ScriptRunner` when automations are
configured. The adapter has to resolve `lua.WriteDeps` per cascade dispatch —
same pattern as `wsScriptRunner`. The adapter:

```go
// internal/cli/mcp_wiring.go
type mcpScriptRunner struct{ svc *mcpServices }

func (r *mcpScriptRunner) Run(ctx context.Context, a autocascade.ScriptAction) error {
    return newLuaScriptRunner(scriptEngine, r.svc.luaDeps()).Run(ctx, a)
}
```

`newLuaScriptRunner` is the same factory used by workspace, but it's currently
package-private to `workspace`. **Decision point:** either (a) export it as
`lua.NewScriptRunner(...)` or move it to a shared location, or (b) hand-roll a
similar adapter in the wiring helper.

Looking at `internal/workspace/luascriptrunner.go`: it's ~80 LOC that wraps a
`ScriptExecutor` with cascade-shaped dispatch + ScriptError patching. The
cleanest move is to lift it into `internal/script` as
`script.NewLuaScriptRunner(exec, deps) autocascade.ScriptRunner` — then
workspace and MCP both consume the same helper.

**This is a precursor.** Either:

- **Option A:** Split out `script.NewLuaScriptRunner` as a small follow-up PR before this one.
- **Option B:** Include the lift in TKT-KWAX — adds ~80 LOC of move/refactor to the PR, but keeps the migration in one commit.

Decision: **Option B** for TKT-KWAX. The lift is contained to one file move +
one workspace import update; bundling it avoids the PR-ordering coordination
overhead.

### Step 5 — Watcher adapter

```go
// internal/cli/mcp_wiring.go
type mcpWatcher struct {
    storeWatcher storeStartStopper  // type-assertion on store for StartWatching/StopWatching
    extWatcher   *storage.Watcher    // optional, for ExtraDirs / ExtraFiles use cases
    onChange     func()
}
```

For MCP today, there are no `ExtraDirs` / `ExtraFiles` — just the store's own
watcher. The adapter just calls into fsstore's `StartWatching()` /
`StopWatching()` (already implemented).

The current `workspace.StartWatching` does:
1. Type-assert `store.(storeWatcher)` and call `StartWatching()` if supported
2. Optionally construct a `storage.Watcher` for ExtraDirs/Files

MCP's adapter only needs (1). It can hand off to a tiny helper that type-asserts
the store and starts its own watcher.

### Step 6 — Update MCP tests

`internal/mcp/tools_test.go:68` uses `workspace.NewForTest(meta,
workspace.WithTestStore(st))`.

The migration:

```go
// before
ws := workspace.NewForTest(meta, workspace.WithTestStore(st))

// after — option A (hand-built stub)
ws := newTestMCPServices(t, meta, st)
```

`newTestMCPServices` is a per-test helper that builds the focused services
without going through workspace. ~30 LOC, lives in
`internal/mcp/test_helpers_test.go`.

Alternative: keep tests on `workspace.NewForTest` for now, since `mcp.Services`
is structurally satisfied by `*workspace.Workspace` *minus* the watcher methods
— once the Watcher contract changes, workspace won't satisfy the new Services.
So this isn't really an alternative; the test fixture must migrate.

`internal/mcp/watcher_test.go` is testing `workspace.StartWatching` behavior,
not MCP behavior. The `storage.Watcher` package already has tests for the
underlying debounce + file-change handling. **Decision: delete
watcher_test.go.** Don't relocate — its contents duplicate
`storage/watcher_test.go`.

### Wiring complexity

Original plan estimated ≤100 LOC for the helper. Revised estimate: **150-200
LOC** because:

- ScriptRunner adapter (~30 LOC)
- Watcher adapter (~30 LOC)
- Service construction (~60 LOC including all the validation)
- `mcpServices` struct + accessor methods (~40 LOC)
- Per-call `luaDeps` closure with the cycle-breaking pattern (~20 LOC)

This is still small — comparable to `internal/scheduler/scheduler.go` which is
~150 LOC. The complexity isn't in the wiring helper itself; it's in the
cycle-breaking patterns the helper has to reproduce.

### Alternatives considered

- **(A) Keep `WatchOptions` / `ChangeEvent` in `internal/workspace`, import them in `internal/mcp`** — rejected. Defeats the migration.
- **(B) Move watcher types to a new `internal/watcher` package** — rejected. Extra package weight for what MCP can declare inline.
- **(C) Single `mcp.Watcher` interface declared in MCP** — chosen.
- **(D) Inline `storage.Watcher` directly in `mcp.Services`** — rejected. Ties Services to a concrete storage type.

**Files to modify:**

- `internal/mcp/server.go` — replace 4 watcher methods with `Watcher() Watcher` capability; delete `internal/workspace` import + the compile-time assertion.
- `internal/mcp/tools_entity.go` — rename `PauseWatching` / `ResumeWatching` call sites.
- `internal/cli/mcp.go` — call new wiring helper instead of `workspace.Discover`.
- `internal/cli/mcp_wiring.go` — NEW (~150-200 LOC).
- `internal/mcp/tools_test.go` — migrate test fixture.
- `internal/mcp/test_helpers_test.go` — NEW (~30 LOC) with `newTestMCPServices`.
- `internal/mcp/watcher_test.go` — DELETE.
- `internal/script/` (new file?) — `NewLuaScriptRunner` lifted from `internal/workspace/luascriptrunner.go`.
- `internal/workspace/luascriptrunner.go` — DELETE (moved to `internal/script`).
- `internal/workspace/wsscriptrunner.go` — UPDATE to use `script.NewLuaScriptRunner` instead of the workspace-private helper.

**Files NOT modified:**

- `internal/workspace/workspace.go` — watcher methods stay on Workspace (data-entry / scheduler still use them).
- `internal/mcp/tools_{relation,analysis,helpers,schema,trace,export,lua}.go` — no changes (they only call read-side methods on `Services`).
- `cmd/rela/main.go` — wiring happens in `internal/cli/`, not `cmd/`.

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input sources:** Pure refactor. No new input surfaces. The project root path
comes from `--project-dir` flag or `RELA_PROJECT` env var as today; validation
behavior is unchanged.

**Security-sensitive operations:** None new. File watcher uses the same
`storage.Watcher` semantics. Lua execution uses the same `script.Engine` +
`lua.WriteDeps`.

**Error handling:** Wiring helper returns the same `errors.New("no project
found: ...")` shape as today.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test scenarios:**

1. **AC1 (no workspace import in mcp)** — `grep -r 'internal/workspace' internal/mcp/*.go` returns zero hits (test files included after the fixture migration). Verified by `just arch-lint` enforcing the import ban via a new arch-lint constraint (or via existing rules — verify during implementation).
2. **AC2 (server still works)** — existing MCP integration tests pass: tools_test (entity CRUD via MCP), tools_lua_test (Lua execution), convert_test (type conversions). watcher_test deleted; coverage replaced by `internal/storage/watcher_test.go` (already passing).
3. **AC3 (helper size)** — `wc -l internal/cli/mcp_wiring.go` ≤ 200. Manual review during PR.
4. **AC4 (Services interface narrow)** — `mcp.Watcher` is 4 methods; `mcp.Services` keeps 11 methods (Watcher() replaces 4 watcher methods, net -3). Verified by diff.

**Edge cases:**

- **Project not found** — wiring helper returns the same "no project found" error today.
- **Bad metamodel** — metamodel load fails; wiring returns the underlying error (today swallowed as "no project found"; can stay swallowed for behavior preservation, OR surface — decide during implementation).
- **Lua disabled** — when `scriptEngine` is `script.NewEngine()` with no Lua interpreter configured (impossible today, but theoretically possible), ScriptRunner adapter returns the engine's error. No new behavior.
- **fsstore watcher unsupported (memstore in tests)** — type assertion on `storeWatcher` fails gracefully; the watcher adapter no-ops Start/Stop. Today's `workspace.StartWatching` does the same.

**Negative tests:**

- **mcpServices construction with missing dependency** — exercise via unit test: pass nil store / meta to a constructor variant, expect error. Confirms constructor-validates-required pattern (CLAUDE.md).
- **Watcher.Start invoked twice** — should the adapter detect re-entry and error? Today's `workspace.StartWatching` doesn't guard; preserve that behavior.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:**

1. **ScriptRunner cycle.** Manager needs ScriptRunner; ScriptRunner needs LuaWriteDeps; LuaWriteDeps needs Manager. The per-call `luaDeps func() lua.WriteDeps` closure in `mcpServices` resolves this — same pattern as `wsScriptRunner`. **Mitigation:** confirm the pattern compiles with a small spike before committing to the full migration.
2. **lifting `luaScriptRunner`.** Moving `internal/workspace/luascriptrunner.go` to `internal/script` is mechanical but touches workspace's `wsScriptRunner`. **Mitigation:** keep the file move + import update as the FIRST commit in the PR so the diff is reviewable in isolation.
3. **Test fixture migration.** `tools_test.go` uses `workspace.NewForTest`; building `newTestMCPServices` is ~30 LOC of stub. **Mitigation:** write the helper first, port one test to it, verify it works, then port the rest in one commit.
4. **watcher_test.go deletion.** May lose MCP-specific watcher coverage. **Mitigation:** read the test before deleting; if it asserts MCP-specific behavior (it doesn't — it tests workspace), delete. Otherwise port.
5. **mcp.Services compile-time assertion against Workspace.** Today's `var _ Services = (*workspace.Workspace)(nil)` will fail after the Watcher contract change. **Mitigation:** delete the assertion (not load-bearing now that workspace isn't supposed to satisfy Services).
6. **TKT-Q1JT must merge first.** This ticket assumes `app.FSFactory.AddObserver` exists. **Mitigation:** wait for PR #720 to merge before starting implementation; the plan can be written against the merged API. (Today's status: #720 awaiting tschmits' approval — CI green, auto-merge armed.)

**Effort: M.** ~250-300 LOC of changes (mostly new in the helper, some delete in
mcp). Higher than the original "S" estimate because the ScriptRunner cycle +
tests + script.NewLuaScriptRunner lift add complexity the original plan didn't
account for.

## Documentation Planning

- [x] User-facing docs identified (skip if internal refactor)
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:**

- N/A — Internal refactor, no user-facing docs needed. MCP behavior is unchanged.
- CLAUDE.md cites `mcp.Services` as a positive example. The citation remains correct (in fact more correct — the interface is now genuinely narrow, no longer leaking workspace types).

## Design Review

- [x] Run `/design-review` before starting implementation (done in earlier round; addressed via TKT-LCTG + TKT-Q1JT precursors)
- [x] All critical/significant findings addressed in plan

**Design Review Findings:**

- **Round 1 (cranky on original plan):** Search-lifecycle gap → resolved by TKT-LCTG (merged) + TKT-Q1JT (in review).
- **Round 2 (this plan):** No outstanding findings; the wiring complexity (ScriptRunner cycle, watcher type lifting) is documented and has chosen approaches. If the spike in Risk #1 reveals issues, revise here before implementing.
