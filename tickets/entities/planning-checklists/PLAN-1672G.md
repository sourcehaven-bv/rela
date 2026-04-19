---
id: PLAN-1672G
type: planning-checklist
title: 'Planning: Replace lua.Services struct with minimal consumer interfaces per call site'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Problem statement:**

`internal/lua` exposes a single `Services` struct with Store, Manager, Tracer,
Searcher, Meta, and ProjectRoot fields. Every call site constructs the whole
thing. This violates two Go best practices:

1. **"Accept interfaces, return structs"** — callers pass a concrete struct that exposes capabilities they don't use (e.g., validation never writes but still gets `Manager`).
2. **"Define interfaces at the consumer"** — the lua runtime implicitly requires `Manager`, `Store`, `Tracer`, `Searcher`, `Meta`, but there's no single statement of what it actually needs.

The downstream effects are concrete:

- `internal/validation/lua.go:42` has to do `svc.Manager = nil` after the fact to disable writes — a dangerous construction hack (easy to forget, and the lua runtime has nil-checks scattered through it).
- `internal/metamodel/script_context.go:15` has `GetWorkspace() interface{}` with a hidden contract ("must be a `lua.Services` value"). Every script caller does `svc, ok := ctx.GetWorkspace().(lua.Services)` with a runtime failure mode. This is there because `metamodel` can't import `lua` (would create a cycle), so the typed contract was erased.
- `internal/workspace/services.go:20` has `LuaServices()` returning the full struct — workspace knows too much about what scripts need.

**Scope:**

*IN scope:*

- Replace `lua.Services` struct with role-based deps (Read/Write) defined in `internal/lua`.
- Update every call site to wire only the capabilities it actually needs.
- Remove the `interface{}` leak in `metamodel.ScriptContext.GetWorkspace()` — delete `ScriptContext` entirely.
- Add `script.NewReaderRuntime` / `NewWriterRuntime` helpers to consolidate wiring across 5-6 sites.
- Delete Meta/ProjectRoot fallback patches in executor.go, action.go, validation/lua.go.
- Keep Lua-script-observable behavior identical except: mutation bindings on a reader runtime now produce "attempt to call a nil value" (Lua-level) instead of "entity manager not available" (Go-level).

*OUT of scope:*

- Changing the Lua surface API for `rela.*` read bindings (no function signature changes).
- Refactoring `internal/lua/runtime.go` internals beyond what the deps change requires.
- Changing script/flow/action semantics, timeout handling, or the secrets/AI wiring in `LoadContextOptions` (reused as-is inside the new helpers).

**Acceptance Criteria:**

1. `lua.Services` struct no longer exists. Replaced by `lua.ReadDeps` and `lua.WriteDeps` value structs.
2. Each call site passes only what it needs:
   - `internal/validation/lua.go` uses `lua.NewReader(readDeps, …)`.
   - `internal/cli/script.go`, `internal/cli/flow.go`, `internal/mcp/tools_lua.go`, `internal/script/executor.go`, `internal/script/action.go`, `internal/dataentry/actions.go` use `lua.NewWriter(writeDeps, …)` — most via the `script.NewWriterRuntime` helper.
3. `metamodel.ScriptContext` is deleted. Engine methods take `lua.WriteDeps` and `*entity.Entity` args directly.
4. The `svc.Manager = nil` hack at `validation/lua.go:42` is gone. Mutation bindings are **not registered** on a reader runtime.
5. Fallback patches (`if svc.Meta == nil { svc.Meta = ctx.GetMeta() }` etc.) at `executor.go:64-70`, `action.go:60-65`, `validation/lua.go:43-48` are all deleted.
6. `script.NewReaderRuntime` / `NewWriterRuntime` helpers exist and all 5-6 call sites use them (no duplicated `LuaServices() + WithAIProvider + LoadContextOptions` boilerplate).
7. `go-arch-lint` passes (no rule changes needed — `script` already depends on `lua` and `workspace` depends on both).
8. `just test`, `just lint`, `just coverage-check` all pass.
9. No `interface{}` type-assertions remain for workspace-to-lua handoff.
10. New unit test: reader runtime + Lua `rela.create_entity(…)` call produces a Lua "attempt to call a nil value" error.

## Research

- [x] Searched for existing libraries that solve this problem
- [x] Checked codebase for similar patterns or reusable code
- [x] Looked for reference implementations in other projects
- [x] Reviewed relevant rela concepts for prior art

**Existing Solutions / Patterns in Codebase:**

- `internal/store.Store` (used by `tracer`, `search`, `entitymanager`, `lua`) is a small consumer-shaped interface — good model: reader-focused, no writer methods leak in.
- `internal/entitymanager.EntityManager` is the write-focused twin of Store.
- BUG-WQ7Y (fixed) established the rule: shared cross-package interfaces live in the **neutral / lower** layer. `metamodel` is lower than `lua` and `workspace`. But `lua` is lower than `script`, `validation`, `workspace`, `cli`, `mcp`, `dataentry` — so `lua` can own its own deps types without creating cycles.
- Prior refactor TKT-910WC ("Introduce workspace.Snapshot as the consumer read API") is the same philosophy for the workspace read path — small consumer-shaped value, narrow interface on the consumer side.

**Key insight:** `internal/lua` already depends on `entitymanager`, `search`,
`store`, `tracer`. Those packages each already expose small interfaces.
`lua.ReadDeps`/`WriteDeps` are just value structs that bundle those existing
types — no new interfaces needed.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical approach — summary (post-design-review):**

Design updated after architect review. All 6 review-responses addressed
(RR-KHO6Q critical; RR-XU0FS, RR-AV6RF, RR-NCNDE, RR-7PZ70 significant; RR-4HS3J
minor).

### 1. Deps as plain value structs in `internal/lua/deps.go`

```go
// ReadDeps is the capability bundle for read-only Lua execution.
type ReadDeps struct {
    Store       store.Store
    Tracer      tracer.Tracer
    Searcher    search.Searcher
    Meta        *metamodel.Metamodel
    ProjectRoot string
}

// WriteDeps extends ReadDeps with entity mutation capability.
type WriteDeps struct {
    ReadDeps
    EntityManager entitymanager.EntityManager
}
```

Why value structs (not accessor interfaces)?

- Avoids service-locator smell (`r.deps.Store()` on every binding call).
- `Workspace.Tracer()` / `Searcher()` allocate fresh instances each call — with a value struct, that cost is paid exactly once at construction time.
- Runtime stores deps as a field; bindings read them directly (`r.deps.Store`), no method dispatch.
- Read/write split is compile-time-enforced via struct embedding: `WriteDeps` embeds `ReadDeps`, and the narrower constructor takes only `ReadDeps`.

### 2. Two constructors, different binding registration

```go
func NewReader(d ReadDeps, stdout io.Writer, opts ...Option) *Runtime
func NewWriter(d WriteDeps, stdout io.Writer, opts ...Option) *Runtime
```

Internally they share most setup, but each calls a different binding registrar:

- `registerReadBindings`: `get_entity`, `list_entities`, `get_relations`, `search`, `trace_from`, `trace_to`, `find_path`, `refresh`, `write_file`, `output`, schema introspection.
- `registerWriteBindings`: read bindings **plus** `create_entity`, `update_entity`, `delete_entity`, `create_relation`, `delete_relation`.

A reader Lua script calling `rela.create_entity` gets a Lua "attempt to call a
nil value" error — the function is not registered. This deletes all scattered
`if r.svc.Manager == nil` checks (runtime.go:951, 979, 1031, 1056, 1082).

### 3. Wiring consolidation helpers in `internal/script`

The helpers take `lua.ReadDeps` / `lua.WriteDeps` directly — no workspace
parameter. This avoids moving the same fat-provider smell one layer out:
`script.NewWriterRuntime` advertises exactly what it consumes, not "give me
the whole workspace and I'll go figure out what I need."

```go
// internal/script/runtime.go (new file)

// Combined wiring: builds a ready-wired runtime from deps + config.
// LoadContextOptions (AI + secrets) is called here so call sites don't repeat it.
func NewReaderRuntime(deps lua.ReadDeps, cacheDir, scriptPath string,
    stdout io.Writer, opts ...lua.Option) (*lua.Runtime, error)
func NewWriterRuntime(deps lua.WriteDeps, cacheDir, scriptPath string,
    stdout io.Writer, opts ...lua.Option) (*lua.Runtime, error)
```

Workspace gets two materialization methods (replacing `LuaServices()`):

```go
// internal/workspace/services.go
func (w *Workspace) LuaReadDeps() lua.ReadDeps
func (w *Workspace) LuaWriteDeps() lua.WriteDeps
```

Call sites do a one-liner:

```go
// cli/script.go
rt, err := script.NewWriterRuntime(
    ws.LuaWriteDeps(), projectCtx.CacheDir, scriptPath, os.Stdout, opts...)
```

Benefits of this shape:

- `internal/script/runtime.go` does not import `internal/workspace`. It only
  uses `lua`, keeping the helper a thin glue layer.
- Tests can construct `lua.ReadDeps{Store: ..., Meta: ...}` directly and call
  the helper without a workspace at all.
- `workspace.LuaReadDeps() / LuaWriteDeps()` is the single place that knows
  how to extract values from workspace — one place to change if workspace
  internals move.
- Validation in `internal/validation` (which cannot import `script` or
  `workspace`) builds its own `lua.ReadDeps` directly — no indirection needed.

All 5-6 write sites (cli/script, cli/flow, mcp eval/run, script/executor,
script/action, dataentry) use `script.NewWriterRuntime`. Validation uses
`lua.NewReader` directly with a hand-built `lua.ReadDeps`. The helper is
optional, not a required detour.

### 4. Delete `metamodel.ScriptContext` entirely

With deps flowing as typed `lua.WriteDeps` arguments directly into the
`script.Engine` methods, `ScriptContext` has no reason to exist. The engine API
becomes:

```go
func (e *Engine) ExecuteCode(code string, deps lua.WriteDeps,
    newEnt, oldEnt *entity.Entity) error
func (e *Engine) ExecuteFile(path string, deps lua.WriteDeps,
    newEnt, oldEnt *entity.Entity) error
func (e *Engine) ExecuteAction(scriptPath string, deps lua.WriteDeps,
    triggerEnt *entity.Entity, params map[string]string,
    timeout time.Duration) (*ActionResponse, error)
```

Callers already hold `*workspace.Workspace` and the entities — they pass them
directly, instead of building a `scriptContextImpl` or `actionScriptContext`
wrapper that re-implemented `ScriptContext`.

### 5. Delete Meta/ProjectRoot fallbacks in same PR

`executor.go:64-70`, `action.go:60-65`, `validation/lua.go:43-48` fallback
patches are dead code once deps are the single source of truth.

**Files to modify:**

| File | Change |
|---|---|
| `internal/lua/services.go` | Delete. |
| `internal/lua/deps.go` | New. `ReadDeps` and `WriteDeps` value structs. |
| `internal/lua/runtime.go` | Replace `svc Services` field with `deps ReadDeps` or `deps WriteDeps` (internal type). Split binding registration into read-only / write. `NewReader` / `NewWriter` constructors replace `New`. Remove all `r.svc.Manager == nil` checks. |
| `internal/script/runtime.go` | New. `NewReaderRuntime`, `NewWriterRuntime` taking `lua.ReadDeps`/`WriteDeps` + `cacheDir` + `scriptPath`. Does not import workspace. |
| `internal/workspace/services.go` | Replace `LuaServices()` with `LuaReadDeps()` / `LuaWriteDeps()` returning the value structs. Delete `luaServices()` internal alias. |
| `internal/metamodel/script_context.go` | Delete. |
| `internal/script/executor.go` | `ExecuteCode`/`ExecuteFile` take `lua.WriteDeps` + `newEnt, oldEnt *entity.Entity`. Drop type-assertion and Meta/ProjectRoot fallbacks. Use `NewWriterRuntime`. |
| `internal/script/action.go` | `ExecuteAction` takes `lua.WriteDeps` + `triggerEnt *entity.Entity`. Drop type-assertion and fallbacks. Use `NewWriterRuntime`. |
| `internal/cli/script.go` | Use `script.NewWriterRuntime(ws.LuaWriteDeps(), projectCtx.CacheDir, scriptPath, os.Stdout, opts...)`. |
| `internal/cli/flow.go` | Same. |
| `internal/mcp/tools_lua.go` | Same. |
| `internal/validation/lua.go` | Build `lua.ReadDeps` directly; use `lua.NewReader`. Delete `svc.Manager = nil` hack and fallback patches. |
| `internal/validator/validator.go`, `_test.go` | Update to pass ReadDeps. |
| `internal/dataentry/app.go` | Drop `luaSvc` construction. Pass workspace. |
| `internal/dataentry/actions.go` | `actionScriptContext` is replaced — pass deps and entity to engine directly. |
| `internal/scheduler/scheduler.go` | Follow engine signature change. |
| Tests that build `lua.Services{...}` | Update to `lua.ReadDeps{...}` / `lua.WriteDeps{...}`. |

**Alternatives considered (rejected):**

1. *Keep `Services` struct, split into `ReadServices` / `WriteServices` structs.* — Rejected: does not solve the `interface{}` leak or the boilerplate.
2. *Method-based interfaces (`ReadDeps.Store() store.Store`).* — Rejected after review (RR-XU0FS, RR-4HS3J): service-locator smell and per-call allocation risk with Workspace's non-memoizing `Tracer()` / `Searcher()`.
3. *Single fat interface.* — Rejected: validation needs read-only guarantee; only two-constructor split gives compile-time enforcement.
4. *Keep `ScriptContext` but typed (Option B in original plan).* — Rejected (RR-KHO6Q): after the refactor it reduces to wrapping 2 `*entity.Entity` fields; just pass them as args.
5. *`WithReadOnly()` option on a single constructor.* — Rejected (RR-AV6RF): would re-introduce nil-checks at runtime. Two constructors with different binding registration is the whole point.

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:**

Structural refactor. No new input surfaces. All existing path validation in
`loadScript`, `openLocalScript`, action redirect validation, secrets/AI wiring
via `LoadContextOptions` remains unchanged.

**Security-Sensitive Operations:**

- **Read/write separation is *strengthened*, not weakened**: today's `svc.Manager = nil` relies on every mutation binding having a correct nil-check. Miss one → validation has write access. After refactor: mutation bindings are not even registered on a reader. Structural enforcement.
- Lua sandbox (no `io`/`os`/`debug`, removed `loadfile`/`dofile`/`load`/`loadstring`/`rawget` etc.) unchanged.
- `os.OpenRoot`-based traversal-resistance unchanged.
- AI and secrets wiring via `LoadContextOptions` unchanged — moved inside helpers but logic identical.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test scenarios:**

1. `lua.Services` gone: `grep -r "lua\.Services" internal/` returns zero matches.
2. Per-site minimal deps: existing call-site tests pass (`cli/script_test.go`, `mcp` integration tests, `script/executor_test.go`, `script/action_test.go`, `validation/lua_test.go`, `dataentry/actions_test.go`).
3. New unit test — `TestReaderRuntimeHasNoMutationBindings`: construct reader runtime, evaluate `return type(rela.create_entity)` as Lua expression, assert it returns `"nil"`.
4. New unit test — `TestReaderRuntimeMutationIsLuaNilCall`: evaluate `rela.create_entity("foo", {})` on reader, assert error message contains "attempt to call a nil value" (not "entity manager not available").
5. `ScriptContext` deleted: `grep metamodel.ScriptContext internal/` returns zero matches.
6. Helpers in use: `grep "lua.New\b" internal/` returns zero matches at call sites (only internal to `internal/lua`).
7. go-arch-lint, coverage ratchet, test suite all pass.

**Edge cases:**

- Nil `Tracer` / `Searcher` in deps: `ReadDeps{Store: s}` with zero-value Tracer/Searcher — existing `if r.svc.Searcher == nil` at runtime.go:914 becomes `if r.deps.Searcher == nil`. Keep the nil-tolerance for bindings that genuinely might not have them (search is optional for non-CLI contexts).
- Workspace without project root: `Workspace.LuaReadDeps()` reads `ws.Paths().Root`; if `Paths()` is nil, returns `ProjectRoot: ""`. Matches current `LuaServices()` behavior.
- Tests that built `lua.Services{}`: switch to `lua.ReadDeps{}` / `lua.WriteDeps{}`. Zero-value accepted by constructors.

**Negative Tests:**

- Reader runtime + each mutation binding name → Lua "attempt to call a nil value".
- Writer with nil `EntityManager` field — should this be constructor-rejected or runtime-errored? Plan: runtime-errored to match current leniency, but covered by test.
- Action script tests continue with `lua.WriteDeps` — all currently-passing tests keep passing.

**Integration test approach:**

- `internal/mcp/tools_lua.go` — MCP integration tests exercise full wiring.
- `internal/script/executor_test.go` — engine end-to-end.
- `internal/validation/lua_test.go` — validation Lua end-to-end. Add: attempt at `rela.create_entity` from a validation rule fails with the new Lua-level error.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl): **m**

**Risks:**

| Risk | Likelihood | Impact | Mitigation |
|---|---|---|---|
| Breaking `ScriptContext` deletion misses a non-obvious implementor | Low | Medium | Known implementors: `workspace.scriptContextImpl`, `actionScriptContext`, test stubs. `grep metamodel.ScriptContext` before and after. |
| Test fixtures hardcode `lua.Services{...}` | High | Low | Simple find/replace to `lua.ReadDeps` / `lua.WriteDeps`. Field names are similar. |
| Error message wording change for mutation-from-read breaks a test | Medium | Low | Explicitly test the new Lua-level error; update matching integration tests. |
| Binding-registration split duplicates code | Medium | Medium | Extract shared table-construction helpers inside `lua`. Reader binds a subset; writer calls reader + adds mutations. |
| A new helper in `script/runtime.go` creates a circular arch boundary | Low | High | `script` already imports `lua`, `workspace`, `metamodel`. All present in current arch-lint config. Verify with `just lint`. |

**Effort: m**

Rough breakdown:
- Define deps structs: 20 min
- Split binding registration in runtime: 2 h
- New constructors + helpers: 1 h
- Update call sites: 2 h
- Delete `ScriptContext` + fallbacks: 1 h
- Update tests: 2 h
- New unit tests for read-only bindings: 30 min
- Address code-review findings: 1-2 h

## Documentation Planning

- [x] User-facing docs identified (skip if internal refactor)
- [x] ~~Docs-checklist will be created when entering implementation~~ (N/A: internal refactor, no user-facing changes)

**Documentation Impact:**

- [x] N/A - Internal refactor. No Lua script surface change, no CLI command change, no schema change.

## Design Review

- [x] Run `/design-review` before starting implementation
- [x] All critical/significant findings addressed in plan

**Design Review Findings:**

- RR-KHO6Q (critical, addressed): Collapse `metamodel.ScriptContext` — deleted entirely, engine takes entity args directly.
- RR-XU0FS (significant, addressed): Workspace.Tracer()/Searcher() per-call allocation — dissolved by switching from accessor interfaces to value structs.
- RR-AV6RF (significant, addressed): Reader runtime does not register mutation bindings at all.
- RR-NCNDE (significant, addressed): Meta/ProjectRoot fallback patches deleted in same PR.
- RR-7PZ70 (significant, addressed): `script.NewReaderRuntime` / `NewWriterRuntime` helpers consolidate wiring at all 5-6 sites.
- RR-4HS3J (minor, addressed): Subsumed by RR-XU0FS redesign — `deps` is a value struct, bindings read fields directly.
