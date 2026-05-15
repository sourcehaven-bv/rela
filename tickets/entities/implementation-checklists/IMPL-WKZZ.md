---
id: IMPL-WKZZ
type: implementation-checklist
title: 'Implementation: Migrate MCP server to wire its own services (off Workspace)'
status: done
---

## Implementation

- [x] Unit tests written for new code
- [x] Integration tests written
- [x] All edge cases from planning handled
- [x] Code follows project patterns (consumer-side interfaces)
- [x] No silent failures (backfill errors surfaced via slog.Warn; bleve init failure surfaced via slog.Warn)

**Summary of changes:**

- `internal/mcp/server.go`:
  - Dropped `internal/workspace` import.
  - Replaced 4 watcher methods on `Services` with `Watcher() Watcher` capability.
  - New `Watcher` interface (4 methods, declared inline at the consumer per CLAUDE.md).
  - Removed compile-time assertion that `*workspace.Workspace` satisfies Services.
- `internal/mcp/tools_entity.go` — rename pause/resume call sites to `Watcher().Pause()` / `Watcher().Resume()`.
- `internal/mcp/test_helpers_test.go` (NEW) — `testServices` stub + `newTestServices` helper for tool tests. Mirrors production wiring; wires autocascade if metamodel declares automations (closes a footgun cranky review flagged: silently bypassing cascade).
- `internal/mcp/tools_test.go` — fixture switched from `workspace.NewForTest` to `newTestServices`.
- `internal/mcp/watcher_test.go` — DELETED. Was a workspace test misfiled in mcp/; storage tests already cover the underlying behavior.
- `internal/cli/mcp_wiring.go` (NEW, ~250 LOC) — `mcpServices` bundle satisfying `mcp.Services`, `mcpScriptRunner` per-call adapter, `mcpWatcher` adapter.
- `internal/cli/mcp_wiring_test.go` (NEW) — coverage for `newMCPServices` happy path, ErrNoProject vs bad-metamodel propagation, Close idempotency, observer wiring proven by writing + searching synchronously, `mcpWatcher` adapter delegation.
- `internal/cli/mcp.go` — calls `newMCPServices` instead of `workspace.Discover`; preserves `ErrNoProject` user message but propagates real errors (metamodel parse, store open).
- `internal/script/luascriptrunner.go` (MOVED from `internal/workspace/`) — public `script.NewLuaScriptRunner(exec Executor, deps lua.WriteDeps)`. Consumer-side `script.Executor` interface (2 methods).
- `internal/script/luascriptrunner_test.go` (MOVED) — same tests, renamed exports.
- `internal/workspace/wsscriptrunner.go` — calls `script.NewLuaScriptRunner` instead of the workspace-private constructor.
- `internal/search/errsearcher.go` (NEW) — `search.ErrSearcher(err)` factory. Lifted from a duplicated implementation in workspace + cli per cranky #1.
- `internal/workspace/services.go` — drops the duplicate `errSearcher`; uses `search.ErrSearcher` instead.
- `.go-arch-lint.yml`:
  - mcp.mayDependOn: drops `workspace`.
  - cli.mayDependOn: adds `app`, `autocascade`, `automation`, `bleveindex`, `config`, `search`, `storage`, `templating`, `tracer`, `validator` (all needed by the wiring helper).
  - script.mayDependOn: adds `autocascade` (LuaScriptRunner consumes the autocascade.ScriptRunner interface).

**Cranky review (round 1) findings + dispositions:**

| # | Severity | Disposition | Resolution |
|---|----------|-------------|------------|
| 1 | critical | addressed | `errSearcher` lifted to `search.ErrSearcher`; both call sites use it |
| 2 | critical | addressed | `backfillBackend` now mirrors workspace's collected-error pattern; caller slog.Warns on partial-index failure |
| 3 | critical | addressed | `runMCPServer` distinguishes `ErrNoProject` (user message) from other errors (wrapped with `fmt.Errorf("mcp startup: %w", err)`) |
| 4 | significant | addressed | 8 unit tests in `mcp_wiring_test.go` covering happy path, error paths, Close idempotency, watcher adapter |
| 5 | significant | addressed | `newTestServices` wires autocascade when `meta.Automations` declared |
| 6 | significant | addressed | Dropped dead `fs` field from `mcpServices` |
| 7 | significant | wont-fix | `mcpWatcher.Pause/Resume` no-op preserved — matches pre-migration behavior (workspace's was also no-op for MCP because no ExtraDirs watcher was ever wired). Separable cleanup |
| 8 | covered by #2 | — | — |
| 9 | minor | addressed | Renamed `scriptEng` → `scriptEngine` |
| 10 | minor | addressed | slog.Warn added when bleve init fails |
| 11 | minor | addressed | (renamed scriptEng) |
| 12 | minor | addressed | Added `var _ relamcp.Services = (*mcpServices)(nil)` |
| 13/14 | leverage | deferred | Shared script.NewPerCallRunner / search.SearchFunc — out of scope |

**Manual verification:**

- `go build ./...` — clean
- `go test -race ./...` — all packages pass
- `just lint` — 0 issues
- `just arch-lint` — OK
- `just ci` — full pipeline passes

**Acceptance criteria:**

1. ✅ `grep -r 'internal/workspace' internal/mcp/` returns zero (test files included after fixture migration)
2. ✅ MCP integration tests pass; watcher_test.go deleted (coverage moved to storage)
3. ✅ Wiring helper ~250 LOC (over the 200 target by ~50 because of the cranky-driven helpers and additional logging — acceptable)
4. ✅ `mcp.Services` interface narrowed (4 watcher methods → 1 Watcher() capability; net -3)
