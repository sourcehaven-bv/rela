---
id: IMPL-UXTT
type: implementation-checklist
title: 'Implementation: Migrate CLI to scoped services helper (drop package globals)'
status: done
---

## Implementation

- [x] Unit tests written for new code (cli_wiring.go)
- [x] Integration tests written (subcommand tests via cobra context)
- [x] All edge cases from planning handled
- [x] Code follows project patterns (consumer-side interfaces, mid-grain bundles)
- [x] No silent failures (cliXFromContext panics with clear message if services not attached)

**Summary of changes:**

- `internal/cli/cli_wiring.go` (NEW, ~280 LOC) — three consumer-side interfaces (`cliRead`, `cliWrite`, `cliAnalyze`), `cliServices` impl, cobra-context plumbing.
- `internal/cli/resolveentitytype.go` (NEW, ~30 LOC) — lifted `ResolveEntityType` from workspace as a free function taking `*metamodel.Metamodel`.
- `internal/cli/root.go` — dropped `ws`/`projectCtx`/`meta` package globals. `PersistentPreRunE` now calls `newCLIServices` + `attachServices`. Kept `out` (CLI output formatting).
- ~25 subcommand files migrated: each `RunE` now starts `svc := cliXFromContext(cmd.Context())` and uses `svc.X()` instead of `ws.X()`. Helper functions that took no params now take `svc` or `meta *metamodel.Metamodel` explicitly.
- `internal/cli/test_helpers_test.go` — fixture now produces `*cliServices` and stamps it on `testCtx` + recursively on rootCmd's subcommands. Sequential-only by design (CLI tests don't `t.Parallel()`).
- 8 `_test.go` files updated to use the new fixture / context plumbing.
- `internal/workspace/workspace.go` — deleted `ResolveEntityType` method (only consumer was CLI; now uses the free function). Deleted associated test.

**Cranky review round 1 dispositions:**

| Severity | Finding | Status |
|---|---|---|
| Critical | None |  |
| Significant | Dead bundle methods (Searcher/FS/Templater) | deferred to TKT-2W0X — methods may be needed by analyze when facades lift |
| Significant | Silent-nil cliXFromContext accessor | **addressed** — accessors panic with clear "subcommand may be annotated skipProjectDiscovery or invoked without PersistentPreRunE" message |
| Significant | `workspace.(*Workspace).ResolveEntityType` duplicate | **addressed** — deleted workspace's; CLI uses free function |
| Significant | `t.Parallel()` overpromised in plan | **addressed (doc)** — known; out, rootCmd flags still block parallel; documented in IMPL summary |
| Minor | Subcommands use `context.Background()` not `cmd.Context()` | deferred — pre-existing pattern, not regression-creating |
| Minor | testCmd lacks applySeeder assertion | **addressed** — panics if testCtx is nil |
| Minor | Two fatcontext suppressions for t.Context() in test setup | acceptable — testCtx is sequential-test fixture by design |
| Leverage | Move workspace.AnalyzeOptions to internal/analyze/types | deferred to TKT-2W0X |
| Leverage | Split cliServices into 3 structs | deferred to TKT-2W0X |

**Manual verification:**

- `go build ./...` — clean
- `go test -race ./...` — all packages pass
- `just lint` — 0 issues
- `just arch-lint` — OK
- `just ci` — full pipeline (frontend, e2e, docs)

**Acceptance criteria:**

1. ✅ `grep -nE '^var ws \|^var projectCtx\|^var meta ' internal/cli/root.go` returns zero
2. ✅ Three bundle interfaces in `cli_wiring.go` with compile-time `var _ cliX = (*cliServices)(nil)` assertions
3. ✅ Each subcommand reads its bundle via FromContext (verified by grep)
4. ✅ Test fixture migrated; `storeSeeder.build()` returns `*cliServices`
5. ✅ CLI race tests pass
6. ✅ CI green
7. ✅ Subcommand workspace imports scrubbed (excluding mcp/validate/flow/scheduler/migrate per scope-out)

**Net delta:** ~+540 / -510 across 32 code files + ~280 LOC of new wiring + ~150
LOC test fixture/test helper changes.
