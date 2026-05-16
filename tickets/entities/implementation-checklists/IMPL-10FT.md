---
id: IMPL-10FT
type: implementation-checklist
title: 'Implementation: Lift workspace analysis methods to internal/analysis'
status: done
---

## Implementation

- [x] New `internal/analysis` package with `Service` exposing 9 methods + 1 free function
- [x] Type names lifted: `Options` (was `AnalyzeOptions`), `Summary` (was `AnalysisSummary`), `ValidationFilter`, `DuplicateGroup`, `GapResult`, `CardinalityViolation`, plus aliases for `validation.{Violation,Result,LoadError}`
- [x] Constructor `New(Deps{Store, Meta, Tracer, LuaReadDeps, LuaCache, FS, Paths})` rejects nil for mandatory deps (Store, Meta, Tracer); LuaCache, FS, Paths optional
- [x] CLI wired via `cliServices.analysis` field; `newCLIServicesFromWorkspace` constructs it from the workspace's primitives
- [x] `validate.go` constructs its own `analysis.Service` (it has its own `workspace.Discover` path); signature of `runValidationChecks` + helpers updated to take `*analysis.Service` instead of `*workspace.Workspace`
- [x] `MetamodelAccessor` moved out of analysis into a private `metamodelAccessor` in `internal/cli/validate.go` — consumer-side narrow interface per CLAUDE.md
- [x] Error masking surfaced via `slog.Warn` in `FindOrphansWithScope` / `collectEntities` (was silent)
- [x] `internal/workspace/analysis.go` + tests deleted (~775 LOC removed)
- [x] `.go-arch-lint.yml`: new `analysis` component; `cli` gains it; `analysis` deps = [lua, metamodel, project, schema, storage, store, tracer, validation]
- [x] `go test -race ./...` clean
- [x] `just lint` clean
- [x] `just arch-lint` OK
- [x] `just ci` full pipeline green

## Cranky review disposition

| # | Severity | Status | Notes |
|---|----------|--------|-------|
| 1 | significant | **Addressed** | Stale `workspace/analysis_test.go` references in CLI comments updated to point at `internal/analysis/analysis_test.go`. |
| 2 | significant | **Addressed (partial)** | `cliAnalyze` drift toward service locator: documented with explicit "drift warning" comment + follow-up note. Splitting into per-subsystem bundles deferred to a separate ticket — out of scope for the lift. |
| 3 | significant | **Addressed (partial)** | Silent error swallowing in `FindOrphansWithScope` + `collectEntities`: now `slog.Warn` instead of silent return. Returning `error` from these methods would ripple across all four facades + AnalyzeAll + CLI callers — deferred as a follow-up. |
| 4 | minor | **Addressed** | `CleanupOrphanedTempFiles` doc now states the (0, nil) no-op behavior when FS/Paths absent. |
| 5 | minor | **Addressed** | Comment on `newValidationService` corrected: per-call to avoid cache aliasing, not to dodge lazy-load state. |
| 6 | minor | Won't fix | Test pattern (hardcoded `DOC-001` etc.) — most tests legitimately need deterministic IDs (gaps + cardinality scoping). CLAUDE.md exempts these. |
| 7 | minor | **Addressed** | `AnalyzeOptions` → `Options` to match the `AnalysisSummary` → `Summary` rename. |
| 8 | leverage | **Addressed** | `MetamodelAccessor` moved to `internal/cli/validate.go` as `metamodelAccessor`. The interface had exactly one consumer; it now lives where it's used. |
| 9 | leverage | Won't fix | `findTempFilesInDir` symlink-loop concern — pre-existing, follow-up ticket if it becomes real. |
| 10 | leverage | Already applied | Compile-time bundle assertions kept. |
