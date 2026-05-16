---
id: TKT-B01S
type: ticket
title: Lift workspace analysis methods to internal/analysis
kind: refactor
priority: medium
effort: m
status: done
---

## Summary

Move workspace's analysis facade (~450 LOC in `internal/workspace/analysis.go`)
into a new `internal/analysis` package. Last of three sequential lifts (after
TKT-2W0X, TKT-RT01).

## In scope

- New `internal/analysis` package:
  - Types: `MetamodelAccessor`, `ValidationFilter`, `AnalyzeOptions`, `DuplicateGroup`, `GapResult`, `CardinalityViolation`, `AnalysisSummary`, `ValidationResult`, `ValidationViolation`, `ValidationLoadError`.
  - `analysis.Service` with constructor taking Store + Meta + Tracer + Validator + FS + Paths.
  - Methods: `FindOrphansWithScope`, `FindDuplicates`, `FindGaps`, `CheckCardinality`, `RunValidations`, `RunValidationsFiltered`, `AnalyzeAll`, `FindOrphanedTempFiles`, `CleanupOrphanedTempFiles`.
  - Helper: `CountValidationsBySeverity`.
- `cliAnalyze` interface: method signatures switch to consume `internal/analysis.*` types instead of `workspace.*`.
- `internal/cli/validate.go`: update type imports (`workspace.AnalyzeOptions` → `analysis.AnalyzeOptions` etc.).
- `internal/workspace/analysis.go` DELETED.
- Tests follow.
- After this lands, `internal/cli/*.go` (production files) no longer imports `internal/workspace` at all.

## Out of scope

- Re-architecting the analysis algorithms themselves.
- Changes to validation engine (`internal/validation`).
- Deleting `internal/workspace` entirely — that's TKT-64R3.

## Depends on

- TKT-2W0X + TKT-RT01 — same wiring-helper pattern; stacked on each other.

## Why

Final lift. Removes the largest workspace import surface from CLI. After this PR
+ the previous two land, only dataentry/scheduler still hold workspace
references (already-narrow). TKT-64R3 (delete workspace) becomes mostly a
cleanup PR.

## Risks

- `RunValidationsFiltered` has filter-tag plumbing that's not symmetric with `RunValidations` — verify the lifted package preserves the exact API.
- Multiple result types (ValidationResult, ScriptError handling) — make sure error paths are preserved.
- `validate.go` is the only file outside `analyze.go`/`gc.go` that uses these types directly — coordinated update needed.
