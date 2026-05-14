---
id: PLAN-W21Z
type: planning-checklist
title: 'Planning: Migrate MCP server to wire its own services (off Workspace)'
status: in-progress
---

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Problem.** `internal/mcp/server.go` declares a consumer-side `Services`
interface (CLAUDE.md cites it as the positive example) but
`*workspace.Workspace` is the only implementation. The interface itself imports
`internal/workspace` for `WatchOptions` / `ChangeEvent` types and the
compile-time satisfaction check, which means `internal/mcp` cannot exist without
`internal/workspace` even though it never uses any workspace-specific behavior
beyond what `Services` names.

After this ticket lands, `cmd/rela mcp` constructs Store, Meta, Tracer,
Searcher, Validator, EntityManager, Config, Paths, LuaWriteDeps, LuaCache
directly, supplies them to MCP via a wiring helper, and `internal/mcp` drops its
`internal/workspace` import.

**Scope (in):**

- New wiring file `internal/cli/mcp_wiring.go` (or inline in `cli/mcp.go`) builds the focused dependencies and exposes a struct that satisfies `mcp.Services`.
- `internal/mcp/server.go` Services interface keeps its current shape, but watcher types (`WatchOptions`, `ChangeEvent`, the `StartWatching` / `StopWatching` / `PauseWatching` / `ResumeWatching` methods) move out of the `workspace` import dependency. Options A and B below.
- `internal/cli/mcp.go` stops calling `workspace.Discover`; calls the new wiring path.
- `internal/mcp/tools_test.go` (one call site to `workspace.NewForTest`) migrates to a stub `mcp.Services` for the test surface it actually needs, or stays as-is if the test fixture remains useful — decide during implementation when the diff is in hand.
- `internal/mcp/watcher_test.go` is a `workspace` unit test by content; it can stay as a workspace test (rename / relocate to `internal/workspace/`) or move to use the new wiring. Decide during implementation.

**Scope (out):**

- Changes to MCP tool implementations (tools_entity, tools_relation, etc.).
- Changes to the `Services` interface surface beyond the watcher-type extraction.
- Watcher behavioral changes — preserve current semantics (200ms debounce, fsstore feature-test, etc.).
- Removing `Workspace.StartWatching` / `StopWatching` / `Pause` / `Resume` methods — they stay on Workspace for now; just that `mcp.Services` no longer references workspace types.
- Building dataentry off Workspace (separate ticket).
- Wiring Workspace as a fallback Services implementer — once `internal/mcp` no longer imports `internal/workspace`, *Workspace stops being a Services implementation*. This is the point of the migration. Other consumers continue to use Workspace; MCP just stops.

**Acceptance Criteria:**

1. `internal/mcp` package's import graph no longer contains `internal/workspace`. Verified by `! grep -r 'internal/workspace' internal/mcp/` returning success (no matches) AND `just arch-lint` passing.
2. `cmd/rela mcp` still starts a working MCP server: file watcher fires on entity file changes; all MCP tools work end-to-end. Verified by existing MCP integration tests passing (tools_test, tools_lua_test, convert_test, watcher_test or its replacement).
3. The wiring helper is small and obvious: under 100 LOC, no service-locator god-struct, no late-binding setters.
4. `mcp.Services` interface stays a consumer-side interface (3–4 *categories* of dependency, naming methods MCP actually invokes); no new methods leak through unless MCP genuinely calls them.

## Research

- [x] Searched for existing libraries that solve this problem
- [x] Checked codebase for similar patterns or reusable code
- [x] Looked for reference implementations in other projects
- [x] Reviewed relevant rela concepts for prior art

**Existing solutions in this repo:**

- `internal/scheduler/scheduler.go` — `WorkspaceProvider` is a 4-method consumer-side interface; the scheduler's entry point in `cmd/rela` constructs a small struct that satisfies it. Same shape as the target here.
- `internal/lua/deps.go` — `ReadDeps` / `WriteDeps` are capability bundles; written by the consumer (Lua), filled in at wiring sites. Demonstrates "construct focused services once at the wiring boundary."
- TKT-IU2S (just merged, PR #711) — flipped `wsEntityManager` to delegate to `entitymanager.Manager`, with single-phase `newWorkspace(store)` construction. Confirms the focused-services pattern works under the runtime constraints (search index, observers).

**Reference implementation:** the `cmd/rela-server` (data-entry server) entry
point already constructs Store, Meta, Tracer, Search etc. individually before
handing them to its HTTP server. The MCP wiring should look structurally
identical.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

### Step 1 — Lift watcher types out of `internal/workspace`

`mcp.Services` references `workspace.WatchOptions`, `workspace.ChangeEvent`,
`workspace.WatchOptions{OnChange: func(_ []workspace.ChangeEvent)}`. While those
still live in `internal/workspace`, MCP must import workspace.

**Chosen approach: option B — move watcher types to `internal/storage`.**

The watcher is already implemented by `storage.Watcher` and emits
`storage.ChangeEvent`. `workspace.WatchOptions` is a thin wrapper that adds
`ExtraDirs`, `ExtraFiles`, and an `OnChange` callback re-typed in terms of a
workspace-local `ChangeEvent` (which is itself just a re-export of
`storage.ChangeEvent` if I read the watcher correctly). The cleanest move is to
define a `Watcher` capability inline in `internal/mcp/server.go` that names what
MCP actually needs:

```go
// internal/mcp/server.go
type Watcher interface {
    Start(onChange func()) error  // MCP only cares "something changed"
    Stop()
    Pause()
    Resume()
}
```

Then `mcp.Services` becomes:

```go
type Services interface {
    Store() store.Store
    Meta() *metamodel.Metamodel
    Tracer() tracer.Tracer
    Searcher() search.Searcher
    Validator() validator.Validator
    EntityManager() entitymanager.EntityManager
    Config() config.Loader
    Paths() *project.Context
    LuaWriteDeps() lua.WriteDeps
    LuaCache() *lua.Cache
    Watcher() Watcher  // narrow capability, defined in mcp
}
```

MCP no longer imports `workspace.WatchOptions` or `workspace.ChangeEvent`. The
wiring site provides an implementation of `mcp.Watcher` — either a small adapter
around `storage.Watcher` for production, or a no-op stub for tests. **The narrow
capability is the contract; the implementation lives at the adapter layer.**
(CLAUDE.md: "transport-specific types belong at adapter layers.")

The current MCP code calls `s.ws.StartWatching(workspace.WatchOptions{ OnChange:
func(_ []workspace.ChangeEvent) { ... }})` but the body ignores the event slice;
it only cares that *something* changed. So `mcp.Watcher.Start(onChange func())`
is sufficient. No event payload needs to cross the interface.

**Alternatives considered:**

- **(A) Keep `WatchOptions` / `ChangeEvent` in `internal/workspace`, import them in `internal/mcp`** — rejected. Defeats the entire migration; `internal/mcp` would still depend on `internal/workspace`.
- **(C) Move `WatchOptions` / `ChangeEvent` to a new `internal/watcher` package** — viable but extra package weight for two types. Burning a top-level package on this is heavy when MCP's actual need is a four-method interface it can declare locally.
- **(D) Inline storage.Watcher directly in mcp.Services without an interface** — rejected. Ties `mcp.Services` to the concrete storage type, which the wiring site can no longer substitute for tests.

### Step 2 — Build the wiring helper

`internal/cli/mcp.go::runMCPServer` currently does:

```go
mcpWs, err := workspace.Discover(startDir, script.NewEngine())
srv := relamcp.NewServer(mcpWs, Version)
```

Replace with a wiring helper in `internal/cli/mcp_wiring.go` (new file) that:

1. Discovers the project (`project.Discover` — already a thin function used by `workspace.Discover` internally).
2. Constructs Store (`fsstore.New(...)`), Meta (loads metamodel.yaml), Tracer (`tracer.New(store)`), Searcher (`search.NewBleve(...)`), Validator (`validator.New(meta, store)`), EntityManager (`entitymanager.New(deps)`), Config (`config.NewLoader(...)`), LuaCache (`lua.NewCache(...)`), LuaWriteDeps (built per-request? — verify), and a storage.Watcher adapter.
3. Returns an `mcpServices` struct that holds all of the above and has trivial accessor methods satisfying `mcp.Services`.

**Files to modify:**

- `internal/mcp/server.go` — replace `WatchOptions`/`ChangeEvent` usage with `Watcher` capability; drop `internal/workspace` import.
- `internal/mcp/tools_entity.go` — `s.ws.PauseWatching()` / `s.ws.ResumeWatching()` become `s.ws.Watcher().Pause()` / `s.ws.Watcher().Resume()`.
- `internal/cli/mcp.go` — call new wiring helper instead of `workspace.Discover`.
- `internal/cli/mcp_wiring.go` — NEW. Builds focused services, returns `mcp.Services` impl.
- `internal/mcp/tools_test.go` — migrate test-side fixture (decide stub-vs-real during implementation).
- `internal/mcp/watcher_test.go` — relocate or migrate (decide during implementation; this file tests workspace behavior, not MCP).

**Files NOT modified:**

- `internal/workspace/workspace.go` — `StartWatching` / `StopWatching` / Pause / Resume stay on Workspace; only their typed contract with MCP changes.
- `internal/mcp/tools_*.go` (other than tools_entity for pause/resume rename).
- `cmd/rela/main.go` — wiring happens in `internal/cli/`, not `cmd/`.

### Step 3 — Verify and clean up

- Compile-time assertion: `var _ Services = (*mcpServices)(nil)` in the wiring file.
- Constructor validation: `mcpServices` constructor rejects nil required fields per CLAUDE.md.
- Capability bundles split read/write? — MCP doesn't have a clean split here; the Services interface is mostly read-oriented. Skip this split for MCP (it would just be the whole bundle either way).

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input sources:** This ticket is pure refactor — no new input surfaces. The
project root path comes from `--project-dir` flag or `RELA_PROJECT` env var as
today; validation behavior is unchanged.

**Security-sensitive operations:** File-watcher start/stop is now exposed
through a narrower capability but the underlying `storage.Watcher` is untouched.
No new privileges, no new file access patterns.

**Error handling:** Wiring helper returns the same `errors.New("no project
found: ...")` shape as today. No sensitive info added.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test scenarios:**

1. **AC1 (no workspace import in mcp)** — `grep -r 'workspace' internal/mcp/*.go` returns zero hits. Added to CI via existing arch-lint check, which already enforces import rules. May need to extend `.archlint.yaml` to explicitly forbid mcp→workspace.
2. **AC2 (server still works)** — existing MCP integration tests pass: tools_test.go (entity CRUD via MCP), tools_lua_test.go (Lua execution), convert_test.go (type conversions). watcher_test.go either migrates or gets relocated.
3. **AC3 (small wiring helper)** — `wc -l internal/cli/mcp_wiring.go` under 100. Manual review during PR.
4. **AC4 (Services interface stays narrow)** — `mcp.Watcher` is 4 methods. `mcp.Services` keeps ~10-11 methods (one Watcher replaces 4 watcher methods, net change is -3). Verified by diff.

**Edge cases:**

- **Watcher unavailable (memstore in tests)** — production wires `storage.Watcher`; tests can wire a no-op stub. Today fsstore's `StartWatching` is feature-tested via type assertion; the wiring layer absorbs that.
- **Empty/missing extra files** — `workspace.WatchOptions{ExtraDirs: nil}` is the typical case; MCP never sets ExtraDirs. The new `mcp.Watcher.Start(onChange)` has no ExtraDirs concept — that's a workspace concern.
- **Concurrent Start/Stop** — `storage.Watcher` already handles this; no new concurrency surfaces.

**Negative tests:**

- **Project not found** — wiring helper returns the same "no project found" error.
- **Bad metamodel** — metamodel load fails; wiring returns the underlying error (today it's swallowed and returned as "no project found"; document any improvement).

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:**

1. **LuaWriteDeps assembly** (mentioned in original ticket) — Workspace assembles `lua.WriteDeps` by handing over its own `store`, `meta`, `entityManager`, etc. The wiring helper has all those pieces in hand. The risk is that some field is workspace-private; verified by reading `Workspace.LuaWriteDeps()` and confirming all dependencies are constructable at the wiring site. **Mitigation:** if anything is workspace-private, lift it to the focused service before this ticket.
2. **Watcher API change** — replacing `StartWatching(WatchOptions)` with `Watcher().Start(onChange)` is a contract change on `mcp.Services`. The change is internal (only MCP references it), but the diff is non-trivial. **Mitigation:** keep the change mechanical — the new `mcp.Watcher` interface names exactly the four operations MCP performs, no more.
3. **Test fixtures** — `tools_test.go` uses `workspace.NewForTest`; migrating it cleanly may force test-side wiring of stubs that didn't exist before. **Mitigation:** the test stub for `mcp.Services` can be hand-written in `internal/mcp/`'s test files; it's a one-time cost.
4. **watcher_test.go** — tests workspace's watcher via the workspace API, not the MCP layer. It's misfiled in `internal/mcp`. **Mitigation:** relocate to `internal/workspace/` as part of this ticket OR delete (workspace already has its own watcher coverage — verify before deleting).

**Effort:** S — the surface is small (one Services interface, one wiring helper,
~6 file changes). The dependency lifting is the riskiest step but preview
reading of Workspace.LuaWriteDeps suggests it's mechanical.

## Documentation Planning

- [x] User-facing docs identified (skip if internal refactor)
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:**

- N/A — Internal refactor, no user-facing docs needed. MCP behavior is unchanged.
- CLAUDE.md already cites `mcp.Services` as a positive example; that citation remains correct after the migration (in fact more correct, since the interface is now genuinely narrow).

## Design Review

- [ ] Run `/design-review` before starting implementation
- [ ] All critical/significant findings addressed in plan

**Design Review Findings:** &lt;!-- to be filled in after design review --&gt;
