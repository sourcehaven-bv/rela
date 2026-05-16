---
id: IMPL-B6ZF
type: implementation-checklist
title: 'Implementation: Migrate scheduler to wire its own services (off Workspace)'
status: done
---

## Implementation

- [x] `cli_wiring.go`: `State() state.KV` added to `cliWrite` (not `cliRead` — `state.KV.Put/Delete` mutates persistent state, belongs on the write bundle)
- [x] `cli_wiring.go`: import `internal/state` added; `cliServices.State()` delegates to `s.ws.State()`
- [x] `scheduler.go`: `skipProjectDiscovery` annotation removed — command now flows through `PersistentPreRunE → newCLIServices` like every other subcommand
- [x] `scheduler.go`: imports `internal/workspace` dropped entirely
- [x] `scheduler.go`: `runScheduler` calls `cliWriteFromContext(cmd.Context())` and passes the bundle directly into `scheduler.New` — `cliWrite` satisfies `scheduler.WorkspaceProvider` structurally (Paths / Config / State / LuaWriteDeps)
- [x] `.go-arch-lint.yml`: adds `state` to `cli.mayDependOn`
- [x] `go test -race ./...` clean
- [x] `just lint` clean
- [x] `just arch-lint` OK
- [x] `just ci` full pipeline green

## Cranky review disposition

| # | Severity | Status | Notes |
|---|----------|--------|-------|
| 1 | critical | **Addressed** | `State()` moved from `cliRead` to `cliWrite`. Original draft put it on `cliRead` ("persistence-layer ops, orthogonal to entity mutation") — reviewer correctly called that out as a lie. `state.KV.Put` mutates persistent state. |
| 2 | significant | **Addressed** | `schedulerProvider` adapter dropped entirely. `cliWrite` satisfies `scheduler.WorkspaceProvider` structurally. The narrow contract is already declared at the scheduler side; the CLI doesn't need to re-narrow. |
| 3 | minor | Acknowledged | "Deck chairs" concern: this PR doesn't decouple cli from workspace — that's TKT-9JEI/64R3 scope. The win is wiring uniformity: scheduler command now flows through the same PersistentPreRunE path as every other subcommand. Worth doing, not oversold. |
| 4 | minor | Verified | Error path: lost no information. `wrapDiscoverError` produces the same "no project found" message and surfaces other errors more informatively than the old single-error swallow. |
| 5 | minor | Resolved | Coverage: `schedulerCmd` still `coverage-ignore`. The adapter that would have needed its own coverage no longer exists (dropped per #2). |
