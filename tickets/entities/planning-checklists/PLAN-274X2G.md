---
id: PLAN-274X2G
type: planning-checklist
title: 'Planning: Lua code sharing between scripts (shared modules via require)'
started: "2026-10-13"
completed: "2026-10-14"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Problem.** No rela Lua script can load code from another file. The sandbox
removes `dofile`, `loadfile`, `load` and `loadstring`, and never opens the
`package` library. gopher-lua's base library still registers `require` and
`module`, but `require` of any project name fails with "package.loaders must be
a table". No `.lua` file in the repo calls `require(`.
`docs/lua-scripting.md:50-62` even shows a `scripts/utils/helpers.lua` that
nothing can load.

**Scope (in):**
- A new top-level project directory `lib/` for shared modules.
- A rela-owned `require(name)` that loads `lib/<name>.lua` through `ReadDeps.Files` (rootfs on disk, the layered loader on sqlite).
- Every runtime built by `lua.newRuntime` with a project file source gets it: automations (`lua:`/`lua_file:`, sync and background jobs), schedules, documents and export renders, data-entry actions and webhooks, validations, MCP `lua_run`/`lua_eval`, CLI `rela script`.
- Inline code (`lua:` in schema.yaml, MCP `lua_eval`) can `require` too.
- Remove the leftover `module` global from the sandbox.
- sqlite: `lib` joins the areas `rela.db` carries (`project_files`), so `rela db load`/`dump` move it.
- Error envelopes name the module file (`lib/fmt/date.lua:12`) with a source excerpt.
- Docs: a "Sharing code between scripts" section, a corrected layout example and the Security section.

**Scope (out):**
- Data-migration `lua` steps. A migration is a pure transform that must give the same result when re-run later; a shared module that changed since would silently change an old migration. `require` stays unavailable there, with an explicit error.
- Mail script transports (`.rela/mail.yaml`) and the docs build. Their runtimes have no project file source (`NewDetached`, empty deps); `require` of a module raises "not available in this context". Flows DO get `require` (they use `script.NewWriterRuntime`); a module that yields at its top level raises an error (RR-PCP6DY).
- A compiled-chunk cache. Every run already re-parses its entry script; modules add the same per-run parse. Separate perf ticket if measurements call for it.
- Static checking that a required module exists (names can be computed at run time).
- Third-party module distribution (luarocks, versioning).

**Acceptance Criteria:**
1. An action in `actions/` and a scheduled script in `scripts/` both `require("fmt.date")`, which loads `lib/fmt/date.lua`; both get the module's return value.
2. A module required twice in one run executes once; the second call returns the same table.
3. A module runs with the caller's capabilities. In a reader runtime (validation) a module calling `rela.create_entity` fails exactly as the caller would; `http`/`ai` exist in a module only when the caller was granted them.
4. Invalid names raise a Lua error without loading anything: `../x`, `/abs`, `a/b`, `a..b`, `.a`, `A-b`, empty, non-string. Only `[a-z0-9_]+(\.[a-z0-9_]+)*` is accepted.
5. A symlink in `lib/` that points outside `lib/` is refused (rootfs area containment).
6. A require cycle (`a` requires `b` requires `a`) raises "require cycle: a -> b -> a" instead of overflowing the stack.
7. A missing module raises "module not found: fmt.date (looked in lib/fmt/date.lua)". No error message contains an absolute filesystem path.
8. A runtime error inside a module is reported with the module's path and line, with a source excerpt.
9. `require("string")` (and the other opened stdlib names) still returns the standard library.
10. `module` is nil in every runtime; `require` is nil in migration steps.
11. On the sqlite build a module stored only in `rela.db` (`project_files`, area `lib`) is found by `require`.
12. `rela.cache` inside a module uses the entry script's namespace (documented).
13. An entry script at `actions/lib/x.lua` that requires module `x` keeps its own frames apart from the module's (RR-R4CHJY).
14. `ScriptError.Error()` and the data-entry panel name the module file of the failing line (`lib/x.lua:3`), not the entry path (RR-96T876).
15. A module compile error and a module top-level error point at the module line (RR-CPWD6P).
16. A module that yields at its top level raises "yielded while loading"; yielding from a returned function works (RR-PCP6DY).
17. A validation rule loads a module once for all entities; module functions see each entity they are passed (RR-UQHT7O).
18. Reserved first segments (`rela`, `http`, `os`, ...) are rejected; non-string names are rejected (RR-TK6AA7, RR-ITLV2L).
19. `rela db load` stores only `.lua` files under `lib/` (RR-9S78TV).

## Research

- [x] For larger features: run `/research` to create a structured research doc
- [x] Searched for existing libraries that solve this problem
- [x] Checked codebase for similar patterns or reusable code
- [x] Looked for reference implementations in other projects
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** ~~/research~~ (N/A: options fit in this checklist; one
subsystem, no external dependency)

**Existing Solutions:**
- gopher-lua `OpenPackage` + `package.path`: rejected. It uses `os` file access, follows `?.lua` search templates and allows `package.loadlib`; it bypasses rootfs containment and the sqlite layered loader.
- gopher-lua `PreloadModule`: fits Go-defined modules only; still needs a Go-side source of the Lua files. The custom loader below is the same idea with lazy file loading.
- Neovim (`lua/` dir, dotted names), Redis functions (library registration), Roblox `ModuleScript` (run once, cached return value) all use: one dedicated module root, dotted names, run once per state, cached return. The plan follows that.
- Reusable code: `lua.ReadScript` (`internal/lua/deps.go:178`) already validates local `.lua` paths and reads through `ProjectFiles`; `rootfs` gives per-area containment; `wrapScriptError` (`internal/script/executor.go:163`) remaps frames.
- Concepts: `lua-scripting`. Related tickets: TKT-96VGO (rela.cache with one script serving many documents), TKT-5LCNM (script path plumbing), TKT-CGPYW (rela.mode everywhere).

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:**
1. `internal/lua/require.go` (new). `installRequire(ls, files)` is called from `newRuntime` after options apply and sets the global `require` to a Go function; the read context is `ls.Context()` at call time, so the run's timeout and cancellation apply (RR-LKGRF6):
   - Non-string or invalid name → `ls.ArgError`.
   - Standard library name → the library from the registry's `_LOADED`. Reserved first segment → argument error. User modules are cached in a Go-side map, not `_LOADED`, so a future gopher-lua module cannot shadow one (RR-TK6AA7).
   - Size cap 1 MiB; unreadable-file errors name the likely causes without a filesystem path (RR-O62O59).
   - The module's top level runs in its own coroutine thread (`ls.NewThread` + `Resume`), so a yield there is caught and reported instead of corrupting the caller (RR-PCP6DY).
   - Name in the per-runtime "loading" set → raise the cycle error with the chain.
   - Otherwise map `a.b` → `lib/a/b.lua`, read through `files.Load` directly (not `ReadScript`, whose messages name scripts; RR-D00QPQ), `ls.Load` with chunk name `lib/a/b.lua`, call it with the name as argument, store the result (or `true` when nil) in `_LOADED`, return it.
   - When the runtime has no project file source (detached), `require` of a non-stdlib name raises "require: project modules are not available in this context".
Modules share the caller's globals (`_G`), so capabilities, reader/writer split,
principal, elevation and timeout carry over unchanged. Each run uses a fresh
LState, so `_LOADED` is per run; a module edit takes effect on the next run, as
scripts do today.
2. `internal/lua/runtime.go` `openSafeLibraries`: add `module` to the removed globals; call `installRequire` after the libraries open. `NewDetached` gets the "not available" variant (its `ProjectRoot` is empty, and `rootfs.New("")` would resolve against the working directory).
3. `internal/lua/deps.go`: no new API; `ReadScript` already takes a directory. Update its comment to list `lib`.
4. Error envelopes (RR-R4CHJY, RR-96T876, RR-QBMQ4I): every file-backed entry script runs under its project-relative chunk name (`actions/a.lua`, `scripts/s.lua`) through a new `Runtime.RunChunk`, so the per-surface frame-remap loops are deleted (action, automation, document, MCP `lua_run`). Cache namespaces are unchanged (`SetScriptPath`). `ScriptError` gains `LuaPath` (the file of `LuaLine` when it is not `Path`); `Error()` prints it, the data-entry envelope carries it as `lua.file`, and the SPA panel shows it. Inline surfaces (inline automation, `lua_eval`, inline validation) read the excerpt from the project files when the failure is in `lib/`. Compile and top-level module errors get a synthetic frame at the module line (RR-CPWD6P).
5. `internal/datamigration/luastep.go` `newSandboxedState`: nil `require` and `module`.
6. `internal/appbuild/projectfiles_sqlite.go:42`: add `lib`, collecting only `*.lua` under it (RR-9S78TV). `internal/docscapture/server.go` copies `lib` too; no other code enumerates project areas (RR-SIHAQL).
7. Docs: `docs/lua-scripting.md` new section, layout example, Security section.

**Alternatives considered:**
- Modules under `scripts/lib/`: no new area, but `scripts/` holds runnable entry points; a module there could be scheduled or run via MCP by mistake, and an action requiring from `scripts/` couples two areas. Rejected for a dedicated `lib/`.
- Resolve relative to the caller's own directory (`actions/` requires from `actions/`): blocks the main use case, one helper shared by an action, a validation and a schedule.
- Declare modules in config (`lua_modules:` in schema.yaml, preloaded): extra config surface, and every runtime would load every module. Lazy loading is cheaper and needs no config.
- A `rela.import()` binding instead of `require`: non-standard, editors and linters (lua-language-server) already understand `require` with dotted names.
- Isolated environment per module (own `_ENV`): Lua 5.1 semantics in gopher-lua make this setfenv-based and fragile; shared `_G` matches standard Lua.

**Files to modify:**
- `internal/lua/require.go` (new), `internal/lua/require_test.go` (new)
- `internal/lua/runtime.go`, `internal/lua/deps.go` (comment), `internal/lua/runtime_test.go`
- `internal/script/executor.go`, `internal/script/action.go` (+ tests)
- `internal/datamigration/luastep.go` (+ test)
- `internal/appbuild/projectfiles_sqlite.go` (+ test)
- `docs/lua-scripting.md`
- `tickets/` demo: optionally move shared code of `tickets/scripts/*.lua` into `tickets/lib/` as a dogfood example.

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:**
- Module name: from operator-authored script code, but it can be computed from request data (`require(rela.request.params.x)`). Allowlist regex `^[a-z0-9_]+(\.[a-z0-9_]+)*$`, max length 128; anything else is an argument error. This rules out `..`, separators, absolute paths, volumes and NUL bytes before a path is built.
- Module file content: operator-authored, same trust as `scripts/`.

**Security-Sensitive Operations:**
- File read: only through `ProjectFiles.Load("lib/...")`. On disk that is rootfs with an `os.Root` at `lib/`, so a symlink cannot leave `lib/`; only regular files are read. Never `os.ReadFile`, never `package.path`.
- Capability inheritance: a module adds no capability; it runs in the caller's state. A reader runtime stays a reader. `rela.bypass_acl` is reachable from a module only when the caller already has the elevated handle; this is the same trust as inlining the code.
- No code loading from strings: `load`/`loadstring`/`dofile` stay removed; `require` only reads `lib/`.
- Errors name the module and the project-relative path only.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:**
- AC1: `internal/script` integration test with a temp project: an action (`ExecuteActionRequest`) and a scheduled-style `ExecuteFile` both require `fmt.date`.
- AC2: module increments a global counter at load; require twice; counter is 1 and both results are the same table (`rawequal` is removed, so compare in Go).
- AC3: validation runtime (`NewReader`) module calling `rela.create_entity` → same error as the caller; writer without `http` capability → `http` nil inside the module.
- AC4, AC6, AC7, AC9, AC10: table-driven tests in `require_test.go` and `runtime_test.go`; `luastep` test for `require == nil`.
- AC5: rootfs-backed test with a symlink `lib/evil.lua -> ../secrets.lua`; require raises, message has no absolute path.
- AC8: script error test asserting envelope frame path `lib/x.lua` and excerpt text.
- AC11: sqlite-tagged appbuild test with a module only in `project_files`.
- AC12: cache key test: module writes `rela.cache.set`, entry script reads it.
- Manual: `just dev` on the tickets project with a `lib/` helper used by an action and a document; check error rendering in the UI.

**Edge Cases:**
- Module returns nil → `require` returns `true` (standard Lua).
- Module returns multiple values → only the first is kept.
- Module errors during load → not cached; a second require in the same run retries (and fails again).
- Name `string`, `table` → stdlib; a `lib/string.lua` is never reached (documented).
- Deep nesting `a.b.c.d` → `lib/a/b/c/d.lua`.
- Validation runtime reused across entities: `_LOADED` persists for the rule's analyze run; module loads once per rule.
- Timeout: module load counts towards the caller's timeout (same LState context).
- Unicode or upper case names → rejected by the allowlist.

**Negative Tests:**
- Invalid names (AC4), missing module (AC7), symlink escape (AC5), cycle (AC6), detached runtime, migration step.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:**
- A project already has a top-level `lib/` for something else → runtime: files there are only read when required by name. `rela db load`: only `.lua` files under `lib/` are stored; a symlink to a `.lua` file there is refused, as in `scripts/`.
- Shared globals: a module can clobber a caller global. Same as standard Lua; docs recommend returning a table and using `local`.
- Per-request parse cost for actions with many modules. Mitigation: out-of-scope cache ticket if measured.
- Background jobs pick up a module edit mid-queue. Same behaviour as scripts today.
- Error-frame remapping misses a surface (documents use `runDocumentScript`). Mitigation: test per surface (action, automation, document).

**Effort:** m

## Documentation Planning

- [x] User-facing docs identified (skip if internal refactor)
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:**
- [x] docs/lua-scripting.md: "Sharing code between scripts", layout example, Security section (`require` semantics, `module` removed)
- [x] docs/data-migration.md: one line that migration steps cannot `require`
- [x] ~~docs/cli-reference.md~~ (N/A: no CLI change)
- [x] ~~CLAUDE.md~~ (N/A: no new cross-cutting rule)

## Design Review

- [x] Run `/design-review` before starting implementation
- [x] All critical/significant findings addressed in plan

**Design Review Findings:** RR-R4CHJY, RR-96T876, RR-QBMQ4I (significant);
RR-CPWD6P, RR-PCP6DY, RR-UQHT7O, RR-ITLV2L, RR-L9LODY, RR-9S78TV, RR-SIHAQL,
RR-TK6AA7, RR-O62O59, RR-QDXE61 (minor); RR-LKGRF6, RR-D00QPQ (nit). All are
folded into the approach and acceptance criteria above.
