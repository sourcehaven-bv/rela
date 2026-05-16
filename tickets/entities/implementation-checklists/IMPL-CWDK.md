---
id: IMPL-CWDK
type: implementation-checklist
title: 'Implementation: ScriptRunner takes Mutator per-call (delete wsScriptRunner + mcpScriptRunner)'
status: done
---

## Implementation

- [x] Type move (precursor commit): `CreateOptions / CreateResult / UpdateResult / DeleteResult / RenameOptions / RenameResult / RelationOptions / Warning` lifted from `internal/entitymanager` to `internal/entity` (commonComponent). Breaks the future autocascade↔entitymanager import cycle.
- [x] `autocascade.Mutator` defined (7 methods, mirrors EntityManager — see review disposition for why 7 not 5).
- [x] `autocascade.ScriptRunner.Run(ctx, action, mutator) error` — signature grew.
- [x] `autocascade.Request.Mutator` field — per-cascade, populated by Manager.runWriteCascade at both dispatch sites.
- [x] `script.LuaScriptRunner` restructured: takes `lua.ReadDeps` statically; assembles `lua.WriteDeps` inside Run from readDeps + per-call mutator.
- [x] `internal/workspace/wsscriptrunner.go` deleted.
- [x] `internal/cli/mcp_wiring.go::mcpScriptRunner` deleted.
- [x] Compile-time assertions in `internal/entitymanager/manager.go`: `*Manager` satisfies both `EntityManager` and `autocascade.Mutator`.
- [x] `LuaScriptRunner.Run` rejects nil mutator with a typed error (matches the doc claim).
- [x] Test pinning Manager-as-Mutator: `TestCreate_PassesManagerAsMutator` verifies `Request.Mutator` is populated and equals the Manager instance.
- [x] `go test -race ./...` clean. `just ci` green.

## Cranky review disposition

| # | Severity | Status | Notes |
|---|----------|--------|-------|
| 1 | significant | **Addressed** | `LuaScriptRunner.Run` now rejects nil mutator with `errors.New("mutator is required")` instead of letting the engine nil-deref. Test pins it. |
| 2 | significant | **Addressed** | Stale doc refs to `wsScriptRunner` / `internal/workspace/luascriptrunner.go` updated in `entitymanager/manager.go` and `workspace/workspace.go` to point at `internal/script/luascriptrunner.go`. |
| 3 | significant | **Partially addressed** | The 7-vs-5 Mutator smell: kept 7 for now because lua.WriteDeps.EntityManager is the wide type. Filed TKT-IF37 for the narrowing PR. mutator.go doc explicitly names the ticket and explains the assignment-compatibility reason. |
| 4 | minor | **Addressed** | `Mutator` doc tightened: "transport-vocabulary, not engine-runtime, agnostic" — doesn't claim free of rela's domain types. |
| 5 | minor | **Addressed** | `Request.Mutator` doc now accurate ("production Lua adapter rejects nil with an explicit error"). |
| 6 | minor | **Addressed** | `TestCreate_PassesManagerAsMutator` added — pins `req.Mutator = m`. |
| 7 | minor | Deferred (TKT-IF37) | `internal/lua → entitymanager` import only goes away once WriteDeps.EntityManager narrows. |
| 8 | leverage | **Addressed** | Compile-time `var _ autocascade.Mutator = (*Manager)(nil)` (plus `_ EntityManager = (*Manager)(nil)`). |
| 9 | leverage | Deferred (TKT-IF37) | "Drop Mutator super-set after narrowing" — done as part of that ticket. |
