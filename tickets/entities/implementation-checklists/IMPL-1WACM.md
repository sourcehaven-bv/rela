---
id: IMPL-1WACM
type: implementation-checklist
title: 'Implementation: Replace lua.Services struct with minimal consumer interfaces per call site'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code (TestReaderRuntime_MutationBindingsAbsent, TestReaderRuntime_MutationCallIsLuaNilCall, TestWriterRuntime_MutationBindingsPresent)
- [x] Integration tests updated (all call-site tests: cli, mcp, script/executor, script/action, validation, dataentry, scheduler)
- [x] Happy path implemented
- [x] Edge cases from planning handled (zero-value deps, nil Tracer/Searcher, nil project root)
- [x] Error handling in place (Lua-level nil-call error replaces Go-level manager-not-available)

## Test Quality

- [x] Using fixture builders or factories for test data (mockWorkspace.services / readDeps)
- [x] No hardcoded values in assertions when object is in scope
- [x] Only specifying values that matter for the test
- [x] ~~Interpolated values constructed from objects~~ (N/A: refactor preserves existing test data patterns)
- [x] ~~Property comparisons use original object~~ (N/A: same as above)

## Manual Verification

- [x] Feature manually tested end-to-end (full `just test` suite passes across all 39 packages)
- [x] Each acceptance criterion verified
- [x] Edge cases verified via new unit tests

**Verification Evidence:**

All 10 acceptance criteria verified via grep and test execution:

1. ✅ `lua.Services` struct deleted (`internal/lua/services.go` gone); replaced by `ReadDeps` / `WriteDeps` in `internal/lua/deps.go`.
2. ✅ Each call site uses minimal deps:
   - `internal/validation/lua.go` → `lua.NewReader(deps, io.Discard)` with `lua.ReadDeps`.
   - `internal/cli/script.go`, `flow.go`, `mcp/tools_lua.go`, `script/executor.go`, `script/action.go`, `dataentry/actions.go` → `script.NewWriterRuntime(ws.LuaWriteDeps(), ...)`.
3. ✅ `metamodel.ScriptContext` deleted. Engine methods take `lua.WriteDeps` + `*entity.Entity` directly.
4. ✅ `svc.Manager = nil` hack gone. Mutation bindings only registered by `registerWriteBindings` (not called by `NewReader`).
5. ✅ Meta/ProjectRoot fallback patches at executor.go:64-70, action.go:60-65, validation/lua.go:43-48 all deleted.
6. ✅ `script.NewReaderRuntime` / `NewWriterRuntime` exist in `internal/script/runtime.go`; all call sites use them.
7. ✅ `go-arch-lint` contract preserved (no new package boundary violations; `internal/script` does not import `internal/workspace`).
8. ✅ `just test` passes across all packages (reported in transcript).
9. ✅ No `interface{}` type-assertions remain for workspace-to-lua handoff (grep confirms).
10. ✅ `TestReaderRuntime_MutationCallIsLuaNilCall`: reader + `rela.create_entity` → VM-level "attempt to call a non-function object" error (Lua-level, not Go).

**Grep verification:**

```
$ grep -rn "lua\.Services\b" --include="*.go" internal/ cmd/
(no matches outside comments, now corrected)

$ grep -rn "metamodel\.ScriptContext\|ScriptContext interface" --include="*.go" internal/ cmd/
(no matches)

$ grep -rn "lua\.New(" --include="*.go" internal/ cmd/
(no matches — replaced by lua.NewReader / lua.NewWriter)

$ grep -rn "\.Manager = nil" --include="*.go" internal/
(no matches — hack eliminated)
```

## Quality

- [x] Code follows project patterns (follows the TKT-910WC consumer-shaped interface philosophy: value structs, capability bundles)
- [x] No security issues introduced (read/write separation is *strengthened* — structural, not runtime nil-check)
- [x] No silent failures (errors surfaced via Lua VM "attempt to call nil value" instead of swallowed nil-checks)
- [x] No debug code left behind
